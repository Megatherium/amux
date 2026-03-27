# Domain Types

## Source Files

- `internal/domain/types.go` — Core entity definitions
- `internal/domain/template_context.go` — Template rendering context
- `internal/domain/running_agent.go` — Persisted agent metadata
- `internal/domain/sidebar.go` — Sidebar node hierarchy

## Types to Port

### Ticket

Represents a beads issue. Maps directly to the `ready_issues` view in the beads Dolt database.

```go
// internal/tickets/types.go
type Ticket struct {
    ID          string
    Title       string
    Description string
    Status      string
    Priority    int
    IssueType   string
    Assignee    string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

**Porting notes:**
- Fields map 1:1 to SQL columns in `ready_issues` view
- `Priority` is int (0=critical, 4=backlog), NOT string
- `Assignee` is nullable in SQL (sql.NullString)

### Harness → AssistantConfig (amux equivalent)

Blunderbust's `Harness` maps to amux's `config.AssistantConfig` with extensions.

```go
// Blunderbust original:
type Harness struct {
    Name            string
    CommandTemplate string  // Go text/template for the CLI command
    PromptTemplate  string  // Go text/template for the prompt
    SupportedModels []string
    SupportedAgents []string
    Env             map[string]string
}

// Amux existing:
type AssistantConfig struct {
    Command          string
    InterruptCount   int
    InterruptDelayMs int
}
```

**Porting approach:**
Extend `AssistantConfig` with template and model/agent fields:

```go
type AssistantConfig struct {
    Command          string
    InterruptCount   int
    InterruptDelayMs int
    // NEW from blunderbust:
    CommandTemplate  string            // Go text/template for command
    PromptTemplate   string            // Go text/template for prompt
    SupportedModels  []string          // Model IDs or discovery keywords
    SupportedAgents  []string          // Agent modes (coder, researcher, etc.)
    Env              map[string]string // Extra env vars for this assistant
}
```

**Key difference:** Amux uses a simple `Command` string. Blunderbust uses Go
`text/template` with dynamic fields. Both can coexist — `Command` for simple
launches, `CommandTemplate` for ticket-aware launches.

### Selection

Captures the user's complete choice before launching a tab.

```go
type Selection struct {
    Ticket    Ticket
    Assistant string  // "opencode", "claude", etc.
    Model     string  // "anthropic/claude-sonnet-4-20250514"
    Agent     string  // "coder", "researcher", etc.
}
```

### LaunchSpec

Fully resolved selection ready for PTY/tmux execution.

```go
type LaunchSpec struct {
    Selection       Selection
    RenderedCommand string
    RenderedPrompt  string
    LauncherID      string  // Ticket ID used as tmux window name
    WorkDir         string
}
```

### ModelContext

String-backed type that exposes structured template accessors. Splits
`"anthropic/claude/claude-sonnet-4-20250514"` into provider/org/name.

```go
type ModelContext string

func (m ModelContext) String() string     // Full ID: "anthropic/claude/claude-sonnet-4"
func (m ModelContext) Provider() string   // "anthropic"
func (m ModelContext) Org() string        // "claude"
func (m ModelContext) Name() string       // "claude-sonnet-4-20250514"
```

### TemplateContext

Fat context passed to both command and prompt templates.

```go
type TemplateContext struct {
    // Ticket fields
    TicketID, TicketTitle, TicketDescription string
    TicketStatus string
    TicketPriority int
    TicketIssueType, TicketAssignee string
    TicketCreatedAt, TicketUpdatedAt time.Time

    // Harness fields
    HarnessName string

    // Selection fields
    Model ModelContext
    Agent string

    // Environment fields
    RepoPath, Branch, WorkDir, User, Hostname string

    // Runtime fields
    DryRun, Debug bool
    Timestamp time.Time

    // Rendered prompt (available in command_template via {{.Prompt}})
    Prompt string
}
```

### PersistedRunningAgent

Tracks launched agent sessions for recovery across restarts.

```go
type PersistedRunningAgent struct {
    ID            int
    ProjectDir    string
    WorktreePath  string
    PID           int
    LauncherType  LauncherType  // Tmux=1, Docker=2
    LauncherID    string
    Ticket        string
    TicketTitle   string
    HarnessName   string
    HarnessBinary string
    Model         string
    Agent         string
    StartedAt     time.Time
    LastSeen      time.Time
}
```

**Amux mapping:** Merge relevant fields into amux's `data.TabInfo`:
```go
type TabInfo struct {
    Assistant   string `json:"assistant"`
    Name        string `json:"name"`
    SessionName string `json:"session_name,omitempty"`
    Status      string `json:"status,omitempty"`
    CreatedAt   int64  `json:"created_at,omitempty"`
    // NEW from blunderbust:
    TicketID    string `json:"ticket_id,omitempty"`
    TicketTitle string `json:"ticket_title,omitempty"`
    Model       string `json:"model,omitempty"`
    Agent       string `json:"agent,omitempty"`
}
```

### SidebarNode (for reference only — amux has its own sidebar)

Blunderbust's sidebar hierarchy: Project → Worktree → Harness → Agent

```go
type SidebarNode struct {
    ID, Name, Path string
    Type           SidebarNodeType  // Project=0, Worktree=1, Harness=2, Agent=3
    Children       []SidebarNode
    IsExpanded     bool
    IsRunning      bool
    ParentProject  *SidebarNode
    WorktreeInfo   *WorktreeInfo
    HarnessInfo    *HarnessInfo
    AgentInfo      *AgentInfo
}
```

**Amux mapping:** Amux's `dashboard.Model` handles project/worktree hierarchy.
Agent nodes extend the existing tree. No sidebar node porting needed.

## Migration Notes

1. **No `domain` package in amux.** Types go into `internal/tickets/types.go`
   or are merged into existing amux types.

2. **`SidebarNodeType` constants are blunderbust-specific.** Amux uses its own
   dashboard node system. Don't port these.

3. **`LauncherType` is tmux-only in practice.** Amux handles tmux sessions
   natively. This concept becomes implicit.

4. **All time fields use `time.Time`** (not int64 timestamps like some amux types).
   Consider aligning with amux conventions.

5. **`ModelContext` is clever.** Keep the string-backed type pattern for backward
   template compatibility. The `parts()` method handles 1/2/3-segment model IDs.
