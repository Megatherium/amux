package app

import (
	"context"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/app/activity"
	"github.com/andyrewlee/amux/internal/app/orchestrator"
	"github.com/andyrewlee/amux/internal/app/workspaces"
	"github.com/andyrewlee/amux/internal/config"
	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/discovery"
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
	workspaceService *workspaces.Service
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

	// UI orchestration (focus, prefix, message pump)
	orch *orchestrator.Orchestrator

	// Layout
	keymap KeyMap
	// Lifecycle
	err          error
	shutdownOnce sync.Once
	ctx          context.Context
	supervisor   *supervisor.Supervisor

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
	workspaceManager *workspaces.Manager

	// Terminal capabilities
	keyboardEnhancements tea.KeyboardEnhancementsMsg

	// Perf tracking
	lastInputAt         time.Time
	pendingInputLatency bool

	// Ticket auto-refresh: supervisor worker that polls LatestUpdate() off-thread.
	ticketPoller *ticketPoller
}

// wm returns the WorkspaceManager, lazily initializing it if nil.
// This allows test code to construct App without explicitly setting workspaceManager.
func (a *App) wm() *workspaces.Manager {
	if a.workspaceManager == nil {
		a.workspaceManager = workspaces.NewManager()
	}
	return a.workspaceManager
}

// oc returns the Orchestrator, lazily initializing it if nil.
// This allows test code to construct App without explicitly setting orch.
func (a *App) oc() *orchestrator.Orchestrator {
	if a.orch == nil {
		a.orch = orchestrator.New()
	}
	return a.orch
}
