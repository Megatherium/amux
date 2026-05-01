<p align="center">
  <img width="339" height="105" alt="Screenshot 2026-01-20 at 1 00 23 AM" src="https://github.com/user-attachments/assets/fdbefab9-9f7c-4e08-a423-a436dda3c496" />  
</p>

<p align="center">TUI for easily running parallel coding agents</p>

<p align="center">
  <a href="https://github.com/andyrewlee/amux/releases">
    <img src="https://img.shields.io/github/v/release/andyrewlee/amux?style=flat-square" alt="Latest release" />
  </a>
  <a href="LICENSE">
    <img src="https://img.shields.io/github/license/andyrewlee/amux?style=flat-square" alt="License" />
  </a>
  <img src="https://img.shields.io/badge/Go-1.24.12-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go version" />
  <a href="https://discord.gg/Dswc7KFPxs">
    <img src="https://img.shields.io/badge/Discord-5865F2?style=flat-square&logo=discord&logoColor=white" alt="Discord" />
  </a>
</p>

<p align="center">
  <a href="#quick-start">Quick start</a> ·
  <a href="#how-it-works">How it works</a> ·
  <a href="#features">Features</a> ·
  <a href="#configuration">Configuration</a>
</p>

![amux TUI preview](https://github.com/user-attachments/assets/f5c4647e-a6ee-4d62-b548-0fdd73714c90)

## What is amux?

amux is a terminal UI for running multiple coding agents in parallel with a workspace-first model that can import git worktrees.

## Prerequisites

amux requires [tmux](https://github.com/tmux/tmux) (minimum 3.2). Each agent runs in its own tmux session for terminal isolation and persistence.

## Quick start

```bash
brew tap andyrewlee/amux
brew install amux
```

Or via the install script:

```bash
curl -fsSL https://raw.githubusercontent.com/andyrewlee/amux/main/install.sh | sh
```

Or with Go:

```bash
go install github.com/andyrewlee/amux/cmd/amux@latest
```

Then run `amux` to open the dashboard.

## How it works

Each workspace tracks a repo checkout and its metadata. For local workflows, workspaces are typically backed by git worktrees on their own branches so agents work in isolation and you can merge changes back when done.

## Architecture quick tour

Start with `internal/app/ARCHITECTURE.md` for lifecycle, PTY flow, tmux tagging, and persistence invariants. Message boundaries and command discipline are documented in `internal/app/MESSAGE_FLOW.md`.

## Features

- **Parallel agents**: Launch multiple agents within main repo and within workspaces
- **Ticket-to-agent draft flow**: Select a ticket from the dashboard and configure an agent through a step-by-step slot stack (Harness → Model → Agent)
- **No wrappers**: Works with Claude Code, Codex, Gemini, Amp, OpenCode, and Droid
- **Keyboard + mouse**: Can be operated with just the keyboard or with a mouse
- **All-in-one tool**: Run agents, view diffs, and access terminal

## Configuration

### Assistant harnesses (`~/.amux/config.yaml`)

amux loads assistant definitions from `~/.amux/config.yaml` (preferred) with `~/.amux/config.json` as an override layer. See [`config.example.yaml`](config.example.yaml) for a full annotated example.

Each assistant (also called a *harness*) defines how to launch an AI coding agent with ticket-aware context from your [beads](https://github.com/megatherium/beads) issue tracker.

#### YAML config fields

| Field | Required | Description |
|-------|----------|-------------|
| `command_template` | Yes | Go text/template for the shell command that launches the assistant. Receives the full template context including ticket, model, and agent fields. |
| `prompt_template` | No | Go text/template rendered into `{{.Prompt}}` for use in `command_template`. Useful for composing ticket-specific prompts. |
| `models` | No | List of supported model identifiers in `provider/org/name` format (e.g., `anthropic/claude/claude-sonnet-4`). Shown in the model selector during the draft flow. Empty list = unrestricted. |
| `agents` | No | List of agent types this harness supports (e.g., `coder`, `architect`). Shown in the agent selector. Empty list = unrestricted. |
| `env` | No | Environment variables set when launching this assistant (e.g., `LOG_LEVEL: debug`). |

#### Defaults section

The optional `defaults` block pre-selects values in the draft flow:

| Field | Description |
|-------|-------------|
| `harness` | Default assistant to pre-select in the harness picker. |
| `model` | Default model to pre-select. |
| `agent` | Default agent type to pre-select. |

#### Template context

Both `command_template` and `prompt_template` receive a rich context with the selected ticket, model, and agent:

**Ticket fields:**
- `{{.TicketID}}` — Issue ID (e.g., `bmx-108`)
- `{{.TicketTitle}}` — Issue title
- `{{.TicketDescription}}` — Issue description
- `{{.TicketStatus}}` — Current status (open, in_progress, etc.)
- `{{.TicketPriority}}` — Priority number (1-4, where 1 is critical)
- `{{.TicketIssueType}}` — Issue type (task, bug, epic)
- `{{.TicketAssignee}}` — Assignee identifier
- `{{.TicketParentID}}` — Parent epic ID (if any)
- `{{.TicketCreatedAt}}` — Creation timestamp (ISO 8601)
- `{{.TicketUpdatedAt}}` — Last update timestamp (ISO 8601)

**Selection fields:**
- `{{.Assistant}}` — Harness name (e.g., `claude`)
- `{{.Model}}` — Raw model ID string
- `{{.Model.Provider}}` — Provider segment (e.g., `anthropic`)
- `{{.Model.Org}}` / `{{.Model.Organization}}` — Organization segment (e.g., `claude`)
- `{{.Model.Name}}` — Model name segment (e.g., `claude-sonnet-4`)
- `{{.Agent}}` — Agent type (e.g., `coder`)

**Environment fields:**
- `{{.WorkDir}}` — Working directory for the agent
- `{{.RepoPath}}` — Root repository path
- `{{.Branch}}` — Current git branch
- `{{.User}}` — Current system user
- `{{.Hostname}}` — Machine hostname

**Composed prompt:**
- `{{.Prompt}}` — The rendered `prompt_template` string (available ONLY in `command_template`)

#### File-based templates

Templates can reference external files using the `@` prefix:
```yaml
command_template: "@./templates/claude-command.txt"
```
Relative paths resolve from the config file's directory. Absolute paths work too.

### JSON overrides (`~/.amux/config.json`)

JSON config acts as an override layer — any fields set here take precedence over YAML values. Non-specified fields are preserved from YAML. Useful for machine-managed config (e.g., from CI or tooling) while keeping human-authored templates in YAML.

```json
{
  "assistants": {
    "claude": {
      "command_template": "claude --model {{.Model}}",
      "prompt_template": "Work on {{.TicketID}}: {{.TicketTitle}}"
    }
  }
}
```

### Beads integration (ticket-aware launching)

amux integrates directly with [beads](https://github.com/megatherium/beads) (the Dolt-backed issue tracker) for ticket-aware agent launching.

**Draft flow:**
1. Select a ticket from the beads database in the sidebar
2. The center pane opens a step-by-step slot stack: Harness → Model → Agent
3. Each selection filters available options based on the harness config
4. Templates render with the selected ticket's context
5. Launch fires the rendered command in a new tmux session

Tickets are loaded from the beads Dolt database via the `bd` CLI. Prefix routing is automatic — amux detects the beads project prefix from the workspace's git repository.

### Built-in assistants

amux ships with defaults for these assistants (overridable via config):

| Assistant | Default command | Interrupt count |
|-----------|----------------|-----------------|
| `claude` | `claude` | 2 (Ctrl-C needs double) |
| `codex` | `codex` | 1 |
| `gemini` | `gemini` | 1 |
| `amp` | `amp` | 1 |
| `opencode` | `opencode` | 1 |
| `droid` | `droid` | 1 |
| `cline` | `cline` | 1 |
| `cursor` | `agent` | 1 |
| `pi` | `pi` | 1 |

## Platform Support

AMUX requires `tmux` and is supported on Linux/macOS. Windows is not supported.

Create `.amux/workspaces.json` in your project to run setup commands for new workspaces:

```json
{
  "setup-workspace": [
    "npm install",
    "cp $ROOT_WORKSPACE_PATH/.env.local .env.local"
  ]
}
```

Workspace metadata is stored in `~/.amux/workspaces-metadata/<workspace-id>/workspace.json`, and local worktree directories live under `~/.amux/workspaces/<project>/<workspace>`.

## Development

```bash
git clone https://github.com/andyrewlee/amux.git
cd amux
make run
```

## Operations

- Logs are written to `~/.amux/logs/amux-YYYY-MM-DD.log` (default retention 14 days). Override retention with `AMUX_LOG_RETENTION_DAYS`.
- Perf profiling: set `AMUX_PROFILE=1` to emit periodic timing/counter snapshots; adjust cadence with `AMUX_PROFILE_INTERVAL_MS` (default 5000).
- pprof: set `AMUX_PPROF=1` (or a port like `6061`) to expose `net/http/pprof` on `127.0.0.1`.
- Debug signals: set `AMUX_DEBUG_SIGNALS=1` and send `SIGUSR1` to dump goroutines into the log.
- PTY tracing: set `AMUX_PTY_TRACE=1` or a comma-separated assistant list; traces write to the log dir (or OS temp dir if logging is disabled).
- Prefix key: set `AMUX_PREFIX_KEY` to remap the prefix key (e.g. `ctrl+p`). Default is `ctrl+space`.
- Prefix timeout: set `AMUX_PREFIX_TIMEOUT` to change how long the command palette waits for input (e.g. `5s`). Default is `3s`.
