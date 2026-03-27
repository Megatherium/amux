# Data Layer

## Source Files

- `internal/data/store.go` — `TicketStore` interface + `TicketFilter`
- `internal/data/project_context.go` — Project context wrapper
- `internal/data/git_client.go` — Git operations for worktree discovery
- `internal/data/worktree.go` — Worktree discovery
- `internal/data/dolt/store.go` — Dolt-based TicketStore implementation
- `internal/data/dolt/metadata.go` — Beads metadata.json parsing
- `internal/data/dolt/agent_store.go` — Running agent persistence
- `internal/data/dolt/schema.go` — SQL schema helpers
- `internal/data/dolt/server.go` — Dolt server management
- `internal/data/fake/store.go` — Fake store for testing

## TicketStore Interface

```go
type TicketStore interface {
    ListTickets(ctx context.Context, filter TicketFilter) ([]domain.Ticket, error)
    LatestUpdate(ctx context.Context) (time.Time, error)
}

type TicketFilter struct {
    Status    string
    IssueType string
    Limit     int
    Search    string
}
```

**Amux mapping:** This interface goes into `internal/tickets/store.go`.

## Dolt Connection

### Metadata Parsing

Beads projects have `.beads/metadata.json`:
```json
{
  "database": "dolt",
  "backend": "dolt",
  "dolt_mode": "server",
  "dolt_database": "beads_fo",
  "dolt_server_host": "10.11.0.1",
  "dolt_server_port": 13307,
  "dolt_server_user": "mysql-root"
}
```

`LoadMetadata()` reads this file and returns a `Metadata` struct. Connection is
via MySQL protocol to a running `dolt sql-server`.

### Store Creation

```go
func NewStore(ctx context.Context, opts domain.AppOptions, autostart bool) (*Store, error)
```

1. Loads `metadata.json` from beads dir
2. Resolves server port (from metadata or auto-detection via `bd dolt status`)
3. Connects via MySQL driver (`github.com/go-sql-driver/mysql`)
4. If connection fails and autostart=true, starts Dolt server and retries

### Ticket Query

`ListTickets()` queries the `ready_issues` view:
```sql
SELECT id, title, description, status, priority, issue_type, assignee, created_at, updated_at
FROM ready_issues
WHERE 1=1
  [AND status = ?]
  [AND issue_type = ?]
  [AND title LIKE ?]
ORDER BY priority ASC, updated_at DESC
[LIMIT ?]
```

`ready_issues` is a beads-defined view that filters for unblocked, non-deferred,
non-ephemeral issues.

## Running Agent Persistence

### Schema

```sql
CREATE TABLE IF NOT EXISTS running_agents (
    id INT PRIMARY KEY AUTO_INCREMENT,
    project_dir VARCHAR(255) NOT NULL,
    worktree_path VARCHAR(255) NOT NULL,
    pid INT NOT NULL,
    launcher_type INT NOT NULL,
    launcher_id VARCHAR(100),
    ticket VARCHAR(100),
    ticket_title TEXT,
    harness_name VARCHAR(50) NOT NULL,
    harness_binary VARCHAR(100),
    model VARCHAR(50),
    agent VARCHAR(50),
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uniq_running_agent (project_dir, worktree_path, pid)
);
```

### Key Operations

- `EnsureRunningAgentsTable()` — CREATE TABLE IF NOT EXISTS
- `UpsertRunningAgent()` — INSERT ... ON DUPLICATE KEY UPDATE
- `ListRunningAgentsByProjects()` — SELECT by project dirs
- `ValidateAndPruneRunningAgents()` — Check PID existence + binary matching
- `DeleteStaleRunningAgents()` — DELETE by last_seen age

### Validation Logic

`validateRunningAgent()` checks:
1. Process exists (`syscall.Kill(pid, 0)`)
2. Command matches harness binary (`ps -p PID -o command=`)
3. Binary matching uses `config.HarnessBinaryCandidates()` + `CommandMatchesAnyBinary()`

**Amux mapping:** Amux tracks tabs via tmux sessions. The PID validation can be
replaced by tmux session existence checks (which amux already does in
`app_tmux_activity.go`). The `running_agents` table might merge into amux's
workspace persistence or stay as a separate beads-specific store.

## Git Worktree Discovery

`WorktreeDiscoverer` uses `GitClient` interface:
```go
type GitClient interface {
    ListWorktrees(ctx context.Context, repoRoot string) ([]WorktreeEntry, error)
    DetectMainBranch(ctx context.Context, repoRoot string) (string, error)
    CheckDirty(ctx context.Context, path string) bool
}
```

Parses `git worktree list --porcelain` output. Each entry has Path, Commit, Branch.

**Amux equivalent:** Already exists in amux `internal/git/`. Don't port this.

## ProjectContext

```go
type ProjectContext struct {
    store    TicketStore
    beadsDir string
    rootPath string
}
```

Simple wrapper around TicketStore + project paths. In amux, this becomes part of
the workspace or a parallel context object.

## Amux Integration

1. **`internal/tickets/store.go`** — TicketStore interface + TicketFilter
2. **`internal/tickets/dolt/store.go`** — Dolt implementation
3. **`internal/tickets/dolt/metadata.go`** — Beads metadata parsing
4. **`internal/tickets/dolt/agent_store.go`** — Running agent persistence (or merge into amux TabInfo)
5. **`internal/tickets/fake/store.go`** — Test fake
6. **`internal/tickets/service.go`** — Service wrapping store + discovery + renderer

The fake store should generate sample tickets for testing without a Dolt database.
