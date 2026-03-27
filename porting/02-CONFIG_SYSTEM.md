# Config System

## Source Files

- `internal/config/yaml.go` — YAML struct definitions
- `internal/config/yaml_load.go` — YAML loading, validation, template file loading
- `internal/config/yaml_save.go` — YAML saving
- `internal/config/harness.go` — Binary alias mapping
- `internal/config/render.go` — Template rendering engine
- `internal/config/loader.go` — Loader interface
- `internal/config/tui_config.go` — TUI-specific config (file picker recents)
- `config.example.yaml` — Example configuration

## YAML Config Structure

```yaml
general:
  autostart_dolt: true

launcher:
  target: foreground  # or "background"

harnesses:
  - name: opencode
    command_template: "opencode --model {{.Model}} --agent {{.Agent}}"
    prompt_template: "Work on ticket {{.TicketID}}: {{.TicketTitle}}\n\n{{.TicketDescription}}"
    models:
      - discover:active          # All models from providers with active API keys
      - provider:google          # All models from Google
      - claude-sonnet-4-20250514 # Specific model
    agents:
      - coder
      - task
      - researcher
    env:
      OPENCODE_LOG_LEVEL: "info"

  - name: claude-code
    command_template: "claude --model {{.Model}}"
    prompt_template: |
      Ticket: {{.TicketID}}
      Title: {{.TicketTitle}}
      Type: {{.TicketIssueType}}
      Priority: {{.TicketPriority}}
      
      {{.TicketDescription}}
    models:
      - claude-opus-4
      - claude-sonnet-4
    agents:
      - code
      - architect

defaults:
  harness: opencode
  model: claude-sonnet-4-20250514
  agent: coder

workspaces:
  default:
    projects:
      - dir: /path/to/project
        name: my-project
      - dir: ../other-project
```

## Porting Details

### 1. Loader Interface

```go
type Loader interface {
    Load(path string) (*domain.Config, error)
    Save(path string, cfg *domain.Config) error
}
```

**Amux approach:** Amux's `config.DefaultConfig()` reads from `~/.amux/config.json`.
Extend this to also load a YAML config (or migrate to JSON). The YAML loader is
well-tested and handles file-based templates (`@./path/to/file.txt`).

### 2. Template File Loading

Templates can be inline or loaded from files using `@` prefix:
```yaml
command_template: "@./templates/opencode_command.txt"
```

File paths are relative to the config file directory. This is implemented in
`loadTemplateValue()` in `yaml_load.go:22-42`.

### 3. Harness Binary Aliases

`harness.go` maps harness names to known executable aliases for process validation:
```go
var harnessBinaryAliases = map[string][]string{
    "opencode":    {"opencode"},
    "kilocode":    {"kilocode", "kilo", "kilocode-cli"},
    "claude":      {"claude"},
    "gemini":      {"gemini"},
    // ...
}
```

Used by `CommandMatchesAnyBinary()` to check if a running process matches a
harness. This is needed for `ValidateAndPruneRunningAgents()`.

### 4. Template Rendering

`config.Renderer` renders Go `text/template` strings:

```go
func (r *Renderer) RenderSelection(selection Selection, workDir string) (*LaunchSpec, error)
```

**Rendering order:** Prompt template is rendered first, then command template
(because `{{.Prompt}}` in command_template needs the rendered prompt text).

**Template context fields:**
- `{{.TicketID}}`, `{{.TicketTitle}}`, `{{.TicketDescription}}`, etc.
- `{{.HarnessName}}`, `{{.Model}}`, `{{.Agent}}`
- `{{.Model.Provider}}`, `{{.Model.Org}}`, `{{.Model.Name}}` (subfields)
- `{{.RepoPath}}`, `{{.Branch}}`, `{{.WorkDir}}`
- `{{.DryRun}}`, `{{.Debug}}`, `{{.Timestamp}}`
- `{{.Prompt}}` (rendered prompt text, command_template only)

### 5. Model Discovery Keywords

Models can be specified as:
- Specific ID: `"claude-sonnet-4-20250514"`
- Provider prefix: `"provider:openai"` — all models from OpenAI
- Discovery keyword: `"discover:active"` — all models from providers with env vars set

These are resolved at UI time by the `discovery.Registry`, not at config load time.

### 6. Workspace Config

Workspaces are defined under the `workspaces` key. The `default` workspace
contains a list of projects with their directory paths.

### 7. TUI Config (separate file)

TUI-specific settings stored in `tui_config.yaml`:
```yaml
filepicker_max_recents: 10
filepicker_recents:
  - /home/user/projects/myapp
  - /home/user/work/website
```

**Amux equivalent:** Amux has its own user settings system. File picker recents
may already be handled.

## Amux Integration Points

1. **Extend `config.AssistantConfig`** with `CommandTemplate`, `PromptTemplate`,
   `SupportedModels`, `SupportedAgents`, `Env` fields.

2. **Extend `config.Config`** with `Defaults` (default assistant/model/agent).

3. **New YAML loader** or extend existing JSON config to accept harness-style
   assistant definitions.

4. **`config.Renderer`** becomes `tickets.Renderer` (see subdocument 01).

5. **Binary aliases** go into `config/harness.go` in amux (or `tickets/harness.go`).
