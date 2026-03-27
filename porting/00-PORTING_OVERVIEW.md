# Porting Blunderbust → Amux: Overview

## Goal

Port blunderbust's **ticket-driven workflow** functionality onto amux's superior TUI
architecture. Blunderbust provides the "what to run and why" (ticket selection,
harness/model/agent configuration, template rendering, models.dev discovery).
Amux provides the "how to run it" (tmux session management, PTY virtualization,
workspace isolation, supervisor tree, layer-based compositor UI).

**Blunderbust's UI is intentionally discarded.** Amux's dashboard+center+sidebar
layout, tab system, and compositor are strictly superior.

## Concept Mapping

| Blunderbust Concept | Amux Equivalent | Notes |
|---|---|---|
| `domain.Ticket` | New: `Ticket` (beads integration) | Issue from beads/Dolt DB |
| `domain.Harness` | Amux `config.AssistantConfig` (extended) | Coding tool config |
| `domain.Selection` | New: tab creation context | Ticket + assistant + model + agent |
| `domain.LaunchSpec` | Implicit in amux tab creation | Rendered command for PTY |
| `domain.ModelContext` | New: model field on workspace/tab | provider/org/name from models.dev |
| Template rendering (`config.Renderer`) | New: template engine | Go text/template for commands |
| Discovery (`discovery.Registry`) | New: models.dev integration | Provider/model catalog |
| `data.TicketStore` (Dolt) | New: beads/ticket service | Reads from Dolt |
| `data.WorktreeDiscoverer` | Amux git worktree discovery | Already exists in amux |
| `exec.Launcher` (tmux) | Amux PTY agent + tmux sessions | Already exists in amux |
| `PersistedRunningAgent` | Amux `TabInfo` (extended) | Agent metadata persistence |
| File picker recents | Amux `common.FilePicker` | Already exists |
| TUI config persistence | Amux user settings | Already exists |
| `config.Config` (YAML) | Extend amux `config.Config` | Add ticket/harness blocks |

### What Amux Already Has (DO NOT PORT)

- tmux session lifecycle (create, discover, sync, GC)
- PTY pipeline (reader → message pump → vterm → compositor)
- Supervisor tree for background processes
- Dashboard (project tree, workspaces)
- Center pane (tabs, agent sessions, vterm rendering)
- Sidebar (file tree, git changes, terminal)
- Layout manager (responsive 1-3 pane layout)
- Workspace persistence (metadata store)
- Git worktree discovery
- Assistant config (claude, opencode, etc.)
- File picker with recents
- Toast notifications
- Mouse support, keyboard enhancements
- Layer-based compositor (ultraviolet)
- Activity tracking / hysteresis

### What Must Be Ported (Blunderbust-Only)

1. **Ticket/Beads Integration** — Reading issues from beads/Dolt databases
2. **Template Engine** — Go text/template for command and prompt rendering
3. **Harness Config (Extended)** — Per-assistant models, agents, env vars
4. **Model Discovery** — models.dev API integration for provider/model catalogs
5. **Model/Agent Selection UI** — Column-based selection before launching
6. **Selection → Tab Creation Flow** — Ticket context injected into agent sessions
7. **Running Agent Persistence** — Tracking ticket→agent associations across restarts
8. **Agent Status Monitoring** — tmux window status polling for agent state

## Component Inventory

### Packages to Port

```
blunderbust/internal/          → amux/internal/
├── domain/                    → tickets/ (new package)
│   ├── types.go               → tickets/types.go
│   ├── template_context.go    → tickets/template_context.go
│   └── running_agent.go       → (merge into data workspace)
├── config/                    → config/ (extend existing)
│   ├── harness.go             → config/harness.go (binary aliases)
│   ├── render.go              → tickets/renderer.go (new)
│   ├── harness_test.go        → (port tests)
│   └── render_test.go         → (port tests)
├── data/                      → tickets/ (new package)
│   ├── store.go               → tickets/store.go
│   ├── project_context.go     → (merge into workspace service)
│   ├── dolt/                  → tickets/dolt/ (new)
│   │   ├── store.go           → tickets/dolt/store.go
│   │   ├── metadata.go        → tickets/dolt/metadata.go
│   │   ├── agent_store.go     → tickets/dolt/agent_store.go
│   │   └── schema.go          → tickets/dolt/schema.go
│   └── fake/                  → tickets/fake/ (for testing)
│       └── store.go           → tickets/fake/store.go
├── discovery/                 → discovery/ (new package)
│   ├── models.go              → discovery/models.go
│   ├── models_cache.go        → discovery/models_cache.go
│   └── models_query.go        → discovery/models_query.go
└── config/yaml*.go            → config/ (extend existing loaders)
```

### UI Changes (Not Porting — Adapting)

Amux's existing UI gets extended, not replaced:

- **Dashboard**: Add ticket nodes (beads issues) alongside project/workspace nodes
- **Center**: Tab creation dialog gets ticket/model/agent selection steps
- **Sidebar**: Add ticket detail tab (metadata, description, agent status)
- **New**: Selection matrix or multi-step dialog for ticket→assistant→model→agent

## Suggested Porting Order

Each step is independently testable and builds on the previous.

### Phase 1: Data Layer (no UI changes)

1. **`tickets/types.go`** — Domain types (`Ticket`, `Selection`, `LaunchSpec`, etc.)
2. **`tickets/store.go`** — `TicketStore` interface + `TicketFilter`
3. **`tickets/dolt/store.go`** — Dolt connection, `ListTickets`, `LatestUpdate`
4. **`tickets/dolt/metadata.go`** — Beads metadata.json parsing
5. **`tickets/fake/store.go`** — Fake store with sample data for testing
6. **`tickets/dolt/agent_store.go`** — Running agent persistence (table CRUD)

### Phase 2: Config Extension

7. **`config/harness.go`** — Binary alias mapping for harness validation
8. **Extend `config.Config`** — Add ticket-related config blocks
9. **`tickets/renderer.go`** — Template engine (command + prompt rendering)
10. **`tickets/template_context.go`** — `TemplateContext` + `ModelContext`

### Phase 3: Discovery

11. **`discovery/models.go`** — Provider/Model structs, Registry
12. **`discovery/models_cache.go`** — models.dev API fetch + cache
13. **`discovery/models_query.go`** — Active model filtering, provider queries

### Phase 4: Service Layer

14. **`tickets/service.go`** — Ticket service (wraps store + discovery + renderer)
15. **Wire into app** — Connect ticket service to amux app lifecycle

### Phase 5: UI Integration

16. **Dashboard tickets** — Show beads issues in project tree
17. **Tab creation flow** — Ticket selection → assistant → model → agent → launch
18. **Sidebar ticket tab** — Ticket metadata viewer
19. **Agent status monitoring** — Poll tmux for agent state, update sidebar
20. **Running agent recovery** — Restore persisted agents on startup

### Phase 6: Polish

21. **Model discovery CLI** — `amux update-models` subcommand
22. **Config examples** — Document beads integration in amux config
23. **Tests** — Port blunderbust tests, adapt for amux architecture

## Key Files Reference

Read these files when porting each component:

| Component | Blunderbust Source | Amux Target |
|---|---|---|
| Domain types | `internal/domain/types.go` | `internal/tickets/types.go` |
| Template context | `internal/domain/template_context.go` | `internal/tickets/template_context.go` |
| Running agent | `internal/domain/running_agent.go` | Extend `internal/data/workspace.go` TabInfo |
| Config harness | `internal/config/harness.go` | `internal/config/harness.go` |
| Config render | `internal/config/render.go` | `internal/tickets/renderer.go` |
| YAML loader | `internal/config/yaml_load.go` | `internal/config/config.go` |
| Dolt store | `internal/data/dolt/store.go` | `internal/tickets/dolt/store.go` |
| Dolt metadata | `internal/data/dolt/metadata.go` | `internal/tickets/dolt/metadata.go` |
| Agent store | `internal/data/dolt/agent_store.go` | `internal/tickets/dolt/agent_store.go` |
| Discovery | `internal/discovery/` | `internal/discovery/` |
| Git worktrees | `internal/data/worktree.go` | Already in amux `internal/git/` |
| App orchestration | `internal/app/app.go` | `internal/app/` (extend) |
| CLI entry | `cmd/blunderbust/root_run.go` | `cmd/amux/` (extend) |

## Architecture Diagram

```
┌─────────────────────────────────────────────────────┐
│                    amux App                         │
│  ┌───────────┐  ┌──────────────┐  ┌─────────────┐  │
│  │ Dashboard  │  │   Center     │  │  Sidebar    │  │
│  │ (projects  │  │ (tabs/vterm/ │  │ (git tree/  │  │
│  │  worktrees │  │  PTY agents) │  │  terminal/  │  │
│  │  +TICKETS) │  │              │  │  ticket     │  │
│  │            │  │  ┌────────┐  │  │  detail)    │  │
│  │            │  │  │ Tab 1  │  │  │             │  │
│  │            │  │  │ Tab 2  │  │  │             │  │
│  │            │  │  │ Tab N  │  │  │             │  │
│  └───────────┘  │  └────────┘  │  └─────────────┘  │
│                 └──────────────┘                    │
│  ┌─────────────────────────────────────────────┐   │
│  │              Layout Manager                  │   │
│  └─────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────┤
│                  Services Layer                      │
│  ┌──────────┐ ┌───────────┐ ┌────────────────────┐ │
│  │ Workspace│ │ Git Status│ │  TICKETS (NEW)      │ │
│  │ Service  │ │ Service   │ │  - TicketStore      │ │
│  │          │ │           │ │  - Dolt connection   │ │
│  │          │ │           │ │  - Template renderer │ │
│  │          │ │           │ │  - Model discovery   │ │
│  │          │ │           │ │  - Running agents    │ │
│  └──────────┘ └───────────┘ └────────────────────┘ │
├─────────────────────────────────────────────────────┤
│                  Data Layer                          │
│  ┌──────────┐ ┌───────────┐ ┌────────────────────┐ │
│  │ Workspace│ │  Git      │ │  Beads/Dolt        │ │
│  │ Store    │ │  Client   │ │  (issues DB)       │ │
│  │ (JSON)   │ │           │ │                    │ │
│  └──────────┘ └───────────┘ └────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

## Design Principles

1. **Amux is authoritative for architecture.** Follow amux patterns (message pump,
   supervisor, service layer, compositor). Don't import blunderbust conventions.

2. **New packages, not modified core.** Ticket functionality lives in `internal/tickets/`
   and `internal/discovery/`. Amux core packages get minimal, surgical extensions.

3. **Interface-driven.** TicketStore, DiscoveryRegistry, and Renderer are interfaces
   to enable testing without Dolt or network access.

4. **No Bubble Tea v1 patterns.** Amux uses `charm.land/bubbletea/v2`. All message
   types and Cmd patterns must follow v2 conventions.

5. **Service layer.** Ticket operations are wrapped in a service (like amux's
   `workspaceService`) that the App delegates to.

6. **Persistence via amux patterns.** Running agent metadata uses amux's workspace
   store or a parallel store in the tickets package, not a separate Dolt-only path.
