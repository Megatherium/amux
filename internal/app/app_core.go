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
// It auto-injects deps from App when available so test code that constructs
// App without explicitly wiring SetHandlerDependencies still works.
func (a *App) wm() *WorkspaceManager {
	if a.workspaceManager == nil {
		a.workspaceManager = newWorkspaceManager()
	}
	wm := a.workspaceManager
	// Auto-inject from App fields; only set if not already wired.
	if wm.workspaceService == nil && a.workspaceService != nil {
		wm.workspaceService = a.workspaceService
	}
	if wm.dashboard == nil && a.ui != nil && a.ui.dashboard != nil {
		wm.dashboard = a.ui.dashboard
	}
	if wm.toast == nil && a.ui != nil && a.ui.toast != nil {
		wm.toast = a.ui.toast
	}
	if wm.center == nil && a.ui != nil && a.ui.center != nil {
		wm.center = a.ui.center
	}
	if wm.sidebarTerminal == nil && a.ui != nil && a.ui.sidebarTerminal != nil {
		wm.sidebarTerminal = a.ui.sidebarTerminal
	}
	if wm.gitStatus == nil && a.gitStatus != nil {
		wm.gitStatus = a.gitStatus
	}
	if wm.metadataRoot == "" && a.config != nil {
		wm.metadataRoot = a.config.Paths.MetadataRoot
	}
	if wm.cleanupTmuxSessions == nil {
		wm.cleanupTmuxSessions = a.cleanupWorkspaceTmuxSessions
	}
	if wm.findWorkspaceByID == nil {
		wm.findWorkspaceByID = a.findWorkspaceByID
	}
	if wm.isKnownAssistant == nil {
		wm.isKnownAssistant = a.isKnownAssistant
	}
	if wm.setAppError == nil {
		wm.setAppError = func(err error) { a.err = err }
	}
	if wm.deleteWorkspace == nil {
		wm.deleteWorkspace = a.deleteWorkspace
	}
	if wm.persistWorkspaceTabs == nil {
		wm.persistWorkspaceTabs = a.persistWorkspaceTabs
	}
	if wm.persistActiveTabs == nil {
		wm.persistActiveTabs = a.persistActiveWorkspaceTabs
	}
	return wm
}
