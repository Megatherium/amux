# Exec / Launcher Layer

## Source Files

- `internal/exec/launcher.go` — Launcher interface
- `internal/exec/tmux/launcher.go` — tmux-based launcher
- `internal/exec/tmux/runner.go` — Command execution abstraction
- `internal/exec/tmux/status.go` — tmux window status checker
- `internal/exec/tmux/capture.go` — tmux pane output capture

## Launcher Interface

```go
type Launcher interface {
    Launch(ctx context.Context, spec domain.LaunchSpec) (*domain.LaunchResult, error)
}
```

## tmux Launcher

### How It Works

1. Validates blunderbust is running inside tmux (`$TMUX` env check)
2. Builds `tmux new-window` command with:
   - `-d` flag for background mode
   - `-P -F #{window_id}` for output parsing
   - `-e KEY=VALUE` for harness environment variables
   - `-c workdir` for working directory
   - `-n launcherID` for window name (uses ticket ID)
   - `exec <rendered-command>` to avoid shell wrapper (direct PID resolution)
3. Executes via `CommandRunner` interface
4. Parses window ID from output
5. Fetches pane metadata (pane_id, pane_pid, session_name)
6. Returns `LaunchResult` with launcher ID, type, and PID

### Command Construction

```go
func (l *Launcher) buildCommand(spec domain.LaunchSpec) []string {
    args := []string{"tmux", "new-window"}
    if l.target == "background" {
        args = append(args, "-d")
    }
    args = append(args, "-P", "-F", "#{window_id}", "-e", "LINES=", "-e", "COLUMNS=")
    for key, val := range spec.Selection.Harness.Env {
        args = append(args, "-e", fmt.Sprintf("%s=%s", key, val))
    }
    if spec.WorkDir != "" {
        args = append(args, "-c", spec.WorkDir)
    }
    command := "exec " + strings.TrimSpace(spec.RenderedCommand)
    args = append(args, "-n", spec.LauncherID, command)
    return args
}
```

### Status Checker

```go
type StatusChecker struct {
    runner CommandRunner
}

func (c *StatusChecker) CheckStatus(ctx context.Context, windowName string) TmuxWindowStatus
```

Uses `tmux list-windows -F '#{window_name} #{window_id}'` to check if a window
exists. Returns `Running`, `Dead`, or `Unknown`.

### Output Capture

```go
type OutputCapture struct {
    runner   CommandRunner
    windowID string
}

func (c *OutputCapture) ReadOutput() ([]byte, error)
```

Uses `tmux capture-pane -p -t <windowID>` to read pane contents.

## Amux Equivalents (DO NOT PORT)

| Blunderbust | Amux |
|---|---|
| `exec.Launcher` | amux PTY agent creation + tmux session |
| `tmux.Launcher.buildCommand()` | amux `center/model_tabs.go` tab creation |
| `tmux.StatusChecker` | amux `app_tmux_activity.go` activity tracking |
| `tmux.OutputCapture` | amux PTY reader pipeline → vterm |
| `tmux.CommandRunner` | amux direct exec.Command or tmux service |

**Amux handles all of this natively and more robustly.** The PTY pipeline
(reader → message pump → vterm → compositor) is strictly better than raw
`capture-pane`.

## What IS Ported

The only thing from this package that needs porting is the **command rendering**
step — transforming a `Selection` + templates into the actual command string that
gets passed to amux's tab/agent creation. This is handled by the template engine
(`tickets/renderer.go`), not the launcher.

Amux creates PTY agents via its own mechanisms. The ported code provides the
`renderedCommand` and `renderedPrompt` strings; amux handles execution.
