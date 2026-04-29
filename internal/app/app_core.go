package app

import (
	"context"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/app/activity"
	"github.com/andyrewlee/amux/internal/config"
	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/discovery"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/supervisor"
	"github.com/andyrewlee/amux/internal/tickets"
	"github.com/andyrewlee/amux/internal/tickets/dolt"
	"github.com/andyrewlee/amux/internal/tmux"
	"github.com/andyrewlee/amux/internal/update"
)

// DialogID constants
const (
	DialogAddProject      = "add_project"
	DialogCreateWorkspace = "create_workspace"
	DialogDeleteWorkspace = "delete_workspace"
	DialogRemoveProject   = "remove_project"
	DialogSelectAssistant = "select_assistant"
	DialogSelectTicket    = "select_ticket"
	DialogQuit            = "quit"
	DialogCleanupTmux     = "cleanup_tmux"
)

// prefixTimeoutMsg is sent when the prefix mode timer expires.
type prefixTimeoutMsg struct {
	token int
}

// App is the root Bubbletea model.
type App struct {
	// Configuration
	config           *config.Config
	workspaceService *workspaceService
	gitStatus        GitStatusService
	tmuxService      TmuxOps
	updateService    UpdateService
	modelRegistry    *discovery.Registry
	ticketServices   map[string]*tickets.TicketService
	doltStores       map[string]*dolt.ServerStore
	ticketRenderer   *tickets.Renderer
	discoveryReady   bool

	// Limits
	maxAttachedAgentTabs int

	// State
	projects        []data.Project
	activeWorkspace *data.Workspace
	activeProject   *data.Project
	focusedPane     messages.PaneType
	showWelcome     bool

	// Update state
	updateAvailable *update.CheckResult // nil if no update or dismissed
	version         string
	commit          string
	buildDate       string
	upgradeRunning  bool

	// UI Components (extracted to UICompositor)
	ui *UICompositor

	// Git status management
	gitStatusController *GitStatusController

	// Layout
	keymap KeyMap
	// Lifecycle
	err          error
	shutdownOnce sync.Once
	ctx          context.Context
	supervisor   *supervisor.Supervisor
	// Prefix mode (leader key)
	prefixActive    bool
	prefixToken     int
	prefixSequence  []string
	prefixLabel     string
	prefixHelpLabel string

	tmuxSyncToken             int
	tmuxActivityToken         int
	tmuxActivityScanInFlight  bool
	tmuxActivityRescanPending bool
	tmuxActivitySettled       bool
	tmuxActivitySettledScans  int
	tmuxActivityScannerOwner  bool
	tmuxActivityOwnershipSet  bool
	tmuxActivityOwnerEpoch    int64
	tmuxOptions               tmux.Options
	tmuxAvailable             bool
	tmuxCheckDone             bool
	projectsLoaded            bool
	tmuxInstallHint           string
	tmuxActiveWorkspaceIDs    map[string]bool
	sessionActivityStates     map[string]*activity.SessionState // Per-session hysteresis state
	instanceID                string                            // Immutable after init; safe for read-only access from Cmd goroutines.

	// Workspace lifecycle manager
	workspaceManager *WorkspaceManager

	// Terminal capabilities
	keyboardEnhancements tea.KeyboardEnhancementsMsg

	// Perf tracking
	lastInputAt         time.Time
	pendingInputLatency bool

	// External message pump (for PTY readers)
	externalMsgs     chan tea.Msg
	externalCritical chan tea.Msg
	externalSender   func(tea.Msg)
	externalOnce     sync.Once

	// Ticket auto-refresh: supervisor worker that polls LatestUpdate() off-thread.
	ticketPoller *ticketPoller
}

// wm returns the WorkspaceManager, lazily initializing it if nil.
// This allows test code to construct App without explicitly setting workspaceManager.
func (a *App) wm() *WorkspaceManager {
	if a.workspaceManager == nil {
		a.workspaceManager = newWorkspaceManager()
	}
	return a.workspaceManager
}
