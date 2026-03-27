# Amux Integration Map

## Amux Architecture Summary

```
internal/
├── app/            Root App model, message pump, services
│   ├── app_core.go           App struct (all state)
│   ├── app_init.go           New(), Init() wiring
│   ├── app_msgpump.go        External message pump (PTY backpressure)
│   ├── app_input.go          Update() dispatch
│   ├── app_input_keys.go     Key routing
│   ├── app_input_messages_*.go  Message handlers
│   ├── app_operations.go     High-level operations
│   ├── app_tmux_*.go         tmux lifecycle (discover, sync, GC, activity)
│   ├── app_persistence.go    Workspace state persistence
│   ├── workspace_service.go  Workspace CRUD service
│   ├── harness.go            Headless UI test harness (NOT coding harness)
│   ├── ARCHITECTURE.md       Core runtime docs
│   └── MESSAGE_FLOW.md       Message taxonomy
├── config/         Config loading (JSON), assistant definitions
│   ├── config.go             Config struct + AssistantConfig
│   └── paths.go              Path resolution
├── data/           Data layer (projects, workspaces, persistence)
│   ├── project.go            Project struct
│   ├── workspace.go          Workspace struct (has Assistant field)
│   ├── workspace_store*.go   Workspace persistence (JSON files)
│   └── registry.go           Cross-platform file locking
├── supervisor/     Background process management
│   └── supervisor.go         Task lifecycle
├── tmux/           tmux operations
│   └── (session management, tagging)
├── pty/            PTY agent creation
│   └── (agent struct, session handling)
├── git/            Git operations
│   └── (worktree discovery, status)
├── messages/       Message type definitions
│   └── (all BubbleTea message types)
├── ui/
│   ├── center/     Tab management, vterm rendering
│   ├── dashboard/  Project tree navigation
│   ├── sidebar/    File tree, git changes, terminal
│   ├── layout/     Responsive pane management
│   ├── compositor/ Layer-based rendering (ultraviolet)
│   ├── diff/       Native diff viewer
│   └── common/     Dialog, file picker, toast, styles
├── vterm/          Virtual terminal emulator
└── update/         Version update checking
```

## Integration Touchpoints

### 1. New Package: `internal/tickets/`

```
internal/tickets/
├── types.go              Ticket, Selection, LaunchSpec, ModelContext
├── template_context.go   TemplateContext struct
├── store.go              TicketStore interface + TicketFilter
├── renderer.go           Template rendering (Go text/template)
├── service.go            TicketService (wraps store + discovery + renderer)
├── dolt/
│   ├── store.go          Dolt-backed TicketStore
│   ├── metadata.go       .beads/metadata.json parsing
│   ├── agent_store.go    Running agent persistence
│   └── schema.go         SQL schema helpers
└── fake/
    └── store.go          In-memory fake for testing
```

### 2. New Package: `internal/discovery/`

```
internal/discovery/
├── models.go             Registry, Provider, Model structs
├── models_cache.go       Fetch from models.dev + cache
└── models_query.go       Active model filtering
```

### 3. Extensions to Existing Amux Files

#### `internal/config/config.go`
- Add `Defaults` field (default assistant, model, agent)
- Extend `AssistantConfig` with template and model fields
- Add ticket-related config loading

#### `internal/data/workspace.go`
- Extend `TabInfo` with `TicketID`, `TicketTitle`, `Model`, `Agent` fields
- Add `TicketContext` to workspace (which beads project to read from)

#### `internal/app/app_core.go`
- Add `ticketService` field
- Add `discovery` registry field

#### `internal/app/app_init.go`
- Initialize ticket service during app creation
- Load discovery registry

#### `internal/app/workspace_service.go`
- Add ticket-aware workspace creation (template rendering before tab launch)

#### `internal/ui/dashboard/model.go`
- Add ticket nodes to project tree (beads issues as children of worktrees)

#### `internal/ui/sidebar/`
- Add ticket detail tab (metadata, description)

#### `internal/ui/common/`
- Extend dialog system for multi-step ticket selection flow

## Message Types to Add

```go
// internal/messages/tickets.go
type TicketsLoadedMsg struct {
    Tickets []tickets.Ticket
}
type TicketSelectedMsg struct {
    Ticket tickets.Ticket
}
type TicketRefreshMsg struct{}
type DiscoveryLoadedMsg struct{}
type ModelSelectedMsg struct {
    ModelID string
}
type AgentModeSelectedMsg struct {
    Agent string
}
type LaunchReadyMsg struct {
    Selection tickets.Selection
    Spec      tickets.LaunchSpec
}
```

## Service Layer Integration

```go
// internal/tickets/service.go
type TicketService struct {
    store     TicketStore
    registry  *discovery.Registry
    renderer  *Renderer
}

func (s *TicketService) ListTickets(ctx context.Context, filter TicketFilter) ([]Ticket, error)
func (s *TicketService) ResolveModels(assistant string) []string
func (s *TicketService) RenderLaunch(selection Selection, workDir string) (*LaunchSpec, error)
```

The App delegates to `TicketService` for all ticket-related operations, following
amux's service pattern (like `workspaceService`).

## Amux-Specific Considerations

### 1. Bubble Tea v2

Amux uses `charm.land/bubbletea/v2` (NOT `github.com/charmbracelet/bubbletea`).
All message types, Cmd patterns, and Model interfaces must use v2.

### 2. External Message Pump

Amux has a dedicated external message pump (`app_msgpump.go`) for PTY output
and background workers. Ticket loading (Dolt queries) should return messages
through this pump, not directly mutate state.

### 3. Supervisor Tree

Long-running tasks (model discovery refresh, ticket polling) should use amux's
supervisor pattern rather than raw `tea.Tick`.

### 4. Compositor

Don't implement any rendering. Amux's compositor handles all visual output.
The integration provides data; amux renders it.

### 5. Workspace Persistence

Running agent metadata should extend amux's workspace store rather than creating
a parallel Dolt-only persistence path. Consider storing ticket associations in
`workspace.json` or a separate `tickets.json` in the amux metadata directory.

### 6. Config Format

Amux uses JSON (`~/.amux/config.json`). Blunderbust uses YAML. Either:
- Add YAML support to amux's config loader
- Migrate blunderbust config to JSON
- Support both formats (YAML for beads-specific, JSON for amux core)

## Open Questions

1. **Dolt dependency:** Should amux require Dolt for ticket functionality, or
   should there be an abstraction layer that also supports SQLite/local JSON?
   (Beads uses Dolt, so Dolt is probably fine.)

2. **Template rendering scope:** Should amux support full Go text/template, or
   a simpler variable substitution? (Go text/template is already well-tested
   in blunderbust.)

3. **Config format:** Keep YAML for beads config, JSON for amux config, or unify?

4. **Model selection UI:** Multi-step dialog vs. integrated into existing
   assistant picker vs. new selection pane?

5. **Ticket visibility:** Always show tickets in dashboard, or only when a
   beads project is detected?
