package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/app/activity"
	"github.com/andyrewlee/amux/internal/config"
	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/discovery"
	"github.com/andyrewlee/amux/internal/git"
	"github.com/andyrewlee/amux/internal/logging"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/process"
	"github.com/andyrewlee/amux/internal/supervisor"
	"github.com/andyrewlee/amux/internal/tickets"
	"github.com/andyrewlee/amux/internal/tickets/dolt"
	"github.com/andyrewlee/amux/internal/tmux"
	"github.com/andyrewlee/amux/internal/ui/center"
	"github.com/andyrewlee/amux/internal/ui/common"
	"github.com/andyrewlee/amux/internal/ui/compositor"
	"github.com/andyrewlee/amux/internal/ui/dashboard"
	"github.com/andyrewlee/amux/internal/ui/layout"
	"github.com/andyrewlee/amux/internal/ui/sidebar"
)

// New creates a new App instance.
func New(version, commit, date string) (*App, error) {
	cfg, err := config.DefaultConfig()
	if err != nil {
		return nil, err
	}
	applyTmuxEnvFromConfig(cfg, false)
	tmuxOpts := tmux.DefaultOptions()

	// Ensure directories exist
	if err := cfg.Paths.EnsureDirectories(); err != nil {
		return nil, err
	}

	registry := data.NewRegistry(cfg.Paths.RegistryPath)
	workspaces := data.NewWorkspaceStore(cfg.Paths.MetadataRoot)
	scripts := process.NewScriptRunner(cfg.PortStart, cfg.PortRangeSize)
	workspaceService := newWorkspaceService(registry, workspaces, scripts, cfg.Paths.WorkspacesRoot)

	modelReg, err := discovery.NewRegistry(cfg.Paths.Home)
	if err != nil {
		logging.Warn("Model discovery registry disabled: %v", err)
	}
	ticketRenderer := tickets.NewRenderer()

	// Create status manager (callback will be nil, we use it for caching only)
	statusManager := git.NewStatusManager(nil)
	gitStatus := newGitStatusService(statusManager)

	var tmuxSvc TmuxOps = tmuxOps{}
	updateSvc := newUpdateService(version, commit, date)

	// Create file watcher event channel
	fileWatcherCh := make(chan messages.FileWatcherEvent, 10)

	// Create file watcher with callback that sends to channel
	fileWatcher, fileWatcherErr := git.NewFileWatcher(func(root string) {
		select {
		case fileWatcherCh <- messages.FileWatcherEvent{Root: root}:
		default:
			// Channel full, drop event (will catch on next change)
		}
	})
	if fileWatcherErr != nil {
		logging.Warn("File watcher disabled: %v", fileWatcherErr)
		fileWatcher = nil
	}

	// Create state watcher event channel
	stateWatcherCh := make(chan messages.StateWatcherEvent, 10)

	// Create state watcher with callback that sends to channel
	stateWatcher, stateWatcherErr := newStateWatcher(cfg.Paths.RegistryPath, cfg.Paths.MetadataRoot, func(reason string, paths []string) {
		select {
		case stateWatcherCh <- messages.StateWatcherEvent{Reason: reason, Paths: paths}:
		default:
			// Channel full, drop event (will catch on next change)
		}
	})
	if stateWatcherErr != nil {
		logging.Warn("State watcher disabled: %v", stateWatcherErr)
		stateWatcher = nil
	}

	ctx := context.Background()
	kmap := buildKeymapFromEnv()
	app := &App{
		config:                 cfg,
		workspaceService:       workspaceService,
		gitStatus:              gitStatus,
		tmuxService:            tmuxSvc,
		updateService:          updateSvc,
		modelRegistry:          modelReg,
		ticketRenderer:         ticketRenderer,
		fileWatcher:            fileWatcher,
		fileWatcherCh:          fileWatcherCh,
		fileWatcherErr:         fileWatcherErr,
		stateWatcher:           stateWatcher,
		stateWatcherCh:         stateWatcherCh,
		stateWatcherErr:        stateWatcherErr,
		layout:                 layout.NewManager(),
		dashboard:              dashboard.New(),
		center:                 center.New(cfg),
		sidebar:                sidebar.NewTabbedSidebar(),
		sidebarTerminal:        sidebar.NewTerminalModel(),
		toast:                  common.NewToastModel(),
		focusedPane:            messages.PaneDashboard,
		showWelcome:            true,
		keymap:                 kmap,
		prefixLabel:            PrefixKeyLabel(),
		prefixHelpLabel:        PrefixHelpLabel(),
		dashboardChrome:        &compositor.ChromeCache{},
		centerChrome:           &compositor.ChromeCache{},
		sidebarChrome:          &compositor.ChromeCache{},
		version:                version,
		commit:                 commit,
		buildDate:              date,
		externalMsgs:           make(chan tea.Msg, externalMsgBuffer),
		externalCritical:       make(chan tea.Msg, externalCriticalBuffer),
		ctx:                    ctx,
		tmuxOptions:            tmuxOpts,
		tmuxActiveWorkspaceIDs: make(map[string]bool),
		sessionActivityStates:  make(map[string]*activity.SessionState),
		workspaceManager:       newWorkspaceManager(),
		maxAttachedAgentTabs:   maxAttachedAgentTabsFromEnv(),
	}
	app.instanceID = newInstanceID()
	app.supervisor = supervisor.New(ctx)
	app.installSupervisorErrorHandler()
	// Route PTY messages through the app-level pump.
	app.center.SetMsgSinkTry(app.tryEnqueueExternalMsg)
	app.sidebarTerminal.SetMsgSink(app.enqueueExternalMsg)
	app.center.SetInstanceID(app.instanceID)
	app.sidebarTerminal.SetInstanceID(app.instanceID)
	// Apply saved theme before creating styles
	common.SetCurrentTheme(common.ThemeID(cfg.UI.Theme))
	app.styles = common.DefaultStyles()
	// Propagate styles to all components (they were created with default theme)
	app.dashboard.SetStyles(app.styles)
	app.sidebar.SetStyles(app.styles)
	app.sidebarTerminal.SetStyles(app.styles)
	app.center.SetStyles(app.styles)
	app.toast.SetStyles(app.styles)
	app.setKeymapHintsEnabled(cfg.UI.ShowKeymapHints)
	// Propagate prefix key label to components for help bars
	app.dashboard.SetPrefixHelpLabel(app.prefixHelpLabel)
	app.center.SetPrefixHelpLabel(app.prefixHelpLabel)
	app.sidebarTerminal.SetPrefixHelpLabel(app.prefixHelpLabel)
	// Propagate tmux config to components
	app.center.SetTmuxConfig(tmuxOpts.ServerName, tmuxOpts.ConfigPath)
	app.sidebarTerminal.SetTmuxConfig(tmuxOpts.ServerName, tmuxOpts.ConfigPath)
	app.supervisor.Start("center.tab_actor", app.center.RunTabActor, supervisor.WithRestartPolicy(supervisor.RestartAlways))
	if app.gitStatus != nil {
		app.supervisor.Start("git.status_manager", app.gitStatus.Run)
	}
	if fileWatcher != nil {
		app.supervisor.Start("git.file_watcher", fileWatcher.Run, supervisor.WithBackoff(supervisorBackoff))
	}
	if stateWatcher != nil {
		app.supervisor.Start("app.state_watcher", stateWatcher.Run, supervisor.WithBackoff(supervisorBackoff))
	}
	return app, nil
}

// Init initializes the application.
func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{
		a.loadProjects(),
		a.dashboard.Init(),
		a.center.Init(),
		a.sidebar.Init(),
		a.sidebarTerminal.Init(),
		a.startGitStatusTicker(),
		a.startPTYWatchdog(),
		a.startOrphanGCTicker(),
		a.startTmuxActivityTicker(),
		a.triggerTmuxActivityScan(),
		a.startTmuxSyncTicker(),
		a.checkTmuxAvailable(),
		a.startFileWatcher(),
		a.startStateWatcher(),
		a.checkForUpdates(),
		a.loadDiscoveryRegistry(),
	}
	if a.fileWatcherErr != nil {
		cmds = append(cmds, a.toast.ShowWarning("File watching disabled; git status may be stale"))
	}
	if a.stateWatcherErr != nil {
		cmds = append(cmds, a.toast.ShowWarning("Workspace sync disabled; other instances may be stale"))
	}
	return common.SafeBatch(cmds...)
}

// checkForUpdates starts a background check for updates.
func (a *App) checkForUpdates() tea.Cmd {
	return func() tea.Msg {
		if a.updateService == nil {
			return messages.UpdateCheckComplete{}
		}
		result, err := a.updateService.Check()
		if err != nil {
			logging.Warn("Update check failed: %v", err)
			return messages.UpdateCheckComplete{Err: err}
		}
		return messages.UpdateCheckComplete{
			CurrentVersion:  result.CurrentVersion,
			LatestVersion:   result.LatestVersion,
			UpdateAvailable: result.UpdateAvailable,
			ReleaseNotes:    result.ReleaseNotes,
			Err:             nil,
		}
	}
}

// tmuxAvailableResult is sent after checking tmux availability.
type tmuxAvailableResult struct {
	available   bool
	installHint string
}

func (a *App) checkTmuxAvailable() tea.Cmd {
	return func() tea.Msg {
		if a.tmuxService == nil {
			return tmuxAvailableResult{available: false, installHint: "tmux service unavailable"}
		}
		if err := a.tmuxService.EnsureAvailable(); err != nil {
			return tmuxAvailableResult{available: false, installHint: a.tmuxService.InstallHint()}
		}
		return tmuxAvailableResult{available: true}
	}
}

// startGitStatusTicker returns a command that ticks every 3 seconds for git status refresh.
func (a *App) startGitStatusTicker() tea.Cmd {
	return common.SafeTick(gitStatusTickInterval, func(t time.Time) tea.Msg {
		return messages.GitStatusTick{}
	})
}

// startOrphanGCTicker returns a command that ticks periodically to clean up orphaned tmux sessions.
func (a *App) startOrphanGCTicker() tea.Cmd {
	return common.SafeTick(orphanGCInterval, func(time.Time) tea.Msg {
		return messages.OrphanGCTick{}
	})
}

// startPTYWatchdog ticks periodically to ensure PTY readers are running.
func (a *App) startPTYWatchdog() tea.Cmd {
	return common.SafeTick(ptyWatchdogInterval, func(time.Time) tea.Msg {
		return messages.PTYWatchdogTick{}
	})
}

// startTmuxSyncTicker returns a command that ticks for tmux session reconciliation.
func (a *App) startTmuxSyncTicker() tea.Cmd {
	a.tmuxSyncToken++
	token := a.tmuxSyncToken
	return common.SafeTick(a.tmuxSyncInterval(), func(time.Time) tea.Msg {
		return messages.TmuxSyncTick{Token: token}
	})
}

func (a *App) tmuxSyncInterval() time.Duration {
	value := strings.TrimSpace(os.Getenv("AMUX_TMUX_SYNC_INTERVAL"))
	if value == "" {
		return tmuxSyncDefaultInterval
	}
	interval, err := time.ParseDuration(value)
	if err != nil || interval <= 0 {
		logging.Warn("Invalid AMUX_TMUX_SYNC_INTERVAL=%q; using %s", value, tmuxSyncDefaultInterval)
		return tmuxSyncDefaultInterval
	}
	return interval
}

func applyTmuxEnvFromConfig(cfg *config.Config, force bool) {
	if cfg == nil {
		return
	}
	if force {
		setEnvOrUnset("AMUX_TMUX_SERVER", cfg.UI.TmuxServer)
		setEnvOrUnset("AMUX_TMUX_CONFIG", cfg.UI.TmuxConfigPath)
		setEnvOrUnset("AMUX_TMUX_SYNC_INTERVAL", cfg.UI.TmuxSyncInterval)
		return
	}
	setEnvIfNonEmpty("AMUX_TMUX_SERVER", cfg.UI.TmuxServer)
	setEnvIfNonEmpty("AMUX_TMUX_CONFIG", cfg.UI.TmuxConfigPath)
	setEnvIfNonEmpty("AMUX_TMUX_SYNC_INTERVAL", cfg.UI.TmuxSyncInterval)
}

// startFileWatcher starts watching for file changes and returns events.
func (a *App) startFileWatcher() tea.Cmd {
	if a.fileWatcher == nil || a.fileWatcherCh == nil {
		return nil
	}
	return func() tea.Msg {
		return <-a.fileWatcherCh
	}
}

// startStateWatcher waits for state change notifications.
func (a *App) startStateWatcher() tea.Cmd {
	if a.stateWatcher == nil || a.stateWatcherCh == nil {
		return nil
	}
	return func() tea.Msg {
		return <-a.stateWatcherCh
	}
}

// ticketStoreResult is sent when a per-project ticket store initialization completes.
type ticketStoreResult struct {
	projectPath string
	store       *dolt.ServerStore
	service     *tickets.TicketService
	err         error
}

// loadDiscoveryRegistry loads the models.dev cache asynchronously.
// On success it returns a DiscoveryLoadedMsg; on failure it logs a warning
// and returns nil (discovery features remain disabled).
func (a *App) loadDiscoveryRegistry() tea.Cmd {
	if a.modelRegistry == nil {
		return nil
	}
	return func() tea.Msg {
		if err := a.modelRegistry.Load(context.Background()); err != nil {
			logging.Warn("Model discovery cache load failed: %v", err)
			return nil
		}
		return messages.DiscoveryLoadedMsg{}
	}
}

// initTicketStore attempts to connect to a beads Dolt database at beadsDir.
// If the connection fails, the per-project entry remains nil and ticket
// features are disabled for that project.
func (a *App) initTicketStore(projectPath, beadsDir string) tea.Cmd {
	return func() tea.Msg {
		store, err := dolt.NewStore(context.Background(), beadsDir, false)
		if err != nil {
			logging.Debug("Ticket store not available for %s: %v", projectPath, err)
			return ticketStoreResult{projectPath: projectPath, err: err}
		}

		var svc *tickets.TicketService
		if a.modelRegistry != nil && a.ticketRenderer != nil {
			svc = tickets.NewTicketService(store, a.modelRegistry, a.ticketRenderer)
		}

		return ticketStoreResult{
			projectPath: projectPath,
			store:       store,
			service:     svc,
		}
	}
}

// handleDiscoveryLoaded processes the discovery registry loaded message.
// Once discovery is ready, attempt to init ticket stores for any projects
// that have already been loaded.
func (a *App) handleDiscoveryLoaded(_ messages.DiscoveryLoadedMsg) []tea.Cmd {
	logging.Debug("Model discovery registry loaded")
	a.discoveryReady = true
	return a.initTicketStoresForLoadedProjects()
}

// initTicketStoresForLoadedProjects scans loaded projects for .beads/
// directories and returns init cmds for each. This is called both when
// discovery finishes (projects may already be loaded) and when projects
// load (discovery may already be finished).
func (a *App) initTicketStoresForLoadedProjects() []tea.Cmd {
	if !a.discoveryReady || !a.projectsLoaded {
		return nil
	}
	var cmds []tea.Cmd
	for i := range a.projects {
		p := &a.projects[i]
		beadsDir := filepath.Join(p.Path, ".beads")
		if _, err := os.Stat(beadsDir); err != nil {
			continue
		}
		if _, exists := a.doltStores[p.Path]; exists {
			continue
		}
		cmds = append(cmds, a.initTicketStore(p.Path, beadsDir))
	}
	return cmds
}

// handleTicketStoreResult processes the async ticket store initialization result.
func (a *App) handleTicketStoreResult(msg ticketStoreResult) []tea.Cmd {
	if msg.err != nil {
		logging.Debug("Ticket service disabled for %s: %v", msg.projectPath, msg.err)
		return nil
	}
	if a.doltStores == nil {
		a.doltStores = make(map[string]*dolt.ServerStore)
	}
	if a.ticketServices == nil {
		a.ticketServices = make(map[string]*tickets.TicketService)
	}
	a.doltStores[msg.projectPath] = msg.store
	a.ticketServices[msg.projectPath] = msg.service
	logging.Debug("Ticket service initialized for %s (service=%v)", msg.projectPath, msg.service != nil)
	var cmds []tea.Cmd
	if msg.service != nil {
		cmds = append(cmds, a.loadTicketsForProject(msg.projectPath))
	}
	return cmds
}

// loadTicketsForProject loads open tickets for a project and sends TicketsLoadedMsg.
func (a *App) loadTicketsForProject(path string) tea.Cmd {
	svc := a.ticketServices[path]
	if svc == nil {
		return nil
	}
	return func() tea.Msg {
		t, _ := loadOpenAndInProgress(svc, path, 20)
		return messages.TicketsLoadedMsg{
			ProjectPath: path,
			Tickets:     t,
		}
	}
}

// loadOpenAndInProgress fetches open and in_progress tickets from a TicketService.
func loadOpenAndInProgress(svc *tickets.TicketService, path string, limit int) ([]tickets.Ticket, error) {
	t, err := svc.ListTickets(context.Background(), tickets.TicketFilter{
		Status: "open",
		Limit:  limit,
	})
	if err != nil {
		logging.Debug("Ticket load failed for %s (open): %v", path, err)
		return nil, err
	}
	inProgress, err := svc.ListTickets(context.Background(), tickets.TicketFilter{
		Status: "in_progress",
		Limit:  limit,
	})
	if err != nil {
		logging.Debug("Ticket load failed for %s (in_progress): %v", path, err)
	} else {
		t = append(t, inProgress...)
	}
	return t, nil
}
