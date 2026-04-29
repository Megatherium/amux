package app

import (
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/git"
	"github.com/andyrewlee/amux/internal/logging"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/supervisor"
	"github.com/andyrewlee/amux/internal/ui/dashboard"
	"github.com/andyrewlee/amux/internal/ui/sidebar"
)

// GitStatusController owns the file watcher, state watcher, and their
// associated channels and errors. It encapsulates all watcher lifecycle
// management that was previously scattered across the App struct.
type GitStatusController struct {
	fileWatcher    *git.FileWatcher
	fileWatcherCh  chan messages.FileWatcherEvent
	fileWatcherErr error

	stateWatcher    *stateWatcher
	stateWatcherCh  chan messages.StateWatcherEvent
	stateWatcherErr error

	supervisor *supervisor.Supervisor

	cfg GitStatusControllerConfig
}

// GitStatusControllerConfig holds the dependencies and callbacks for handler methods.
type GitStatusControllerConfig struct {
	GitStatusService     GitStatusService
	Dashboard            *dashboard.Model
	Sidebar              *sidebar.TabbedSidebar
	ActiveWorkspaceRoot  func() string
	LoadProjects         func() tea.Cmd
	ShouldSuppressReload func(paths []string, now time.Time) bool
	SyncDashboard        func()
	StartTicker          func() tea.Cmd
}

// newGitStatusController creates a GitStatusController, initializing the
// file and state watchers (including their event channels).
func newGitStatusController(
	registryPath, metadataRoot string,
	supervisor *supervisor.Supervisor,
	cfg GitStatusControllerConfig,
) *GitStatusController {
	// Create file watcher event channel.
	fileWatcherCh := make(chan messages.FileWatcherEvent, 10)

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

	// Create state watcher event channel.
	stateWatcherCh := make(chan messages.StateWatcherEvent, 10)

	stateWatcher, stateWatcherErr := newStateWatcher(registryPath, metadataRoot, func(reason string, paths []string) {
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

	return &GitStatusController{
		fileWatcher:     fileWatcher,
		fileWatcherCh:   fileWatcherCh,
		fileWatcherErr:  fileWatcherErr,
		stateWatcher:    stateWatcher,
		stateWatcherCh:  stateWatcherCh,
		stateWatcherErr: stateWatcherErr,
		supervisor:      supervisor,
		cfg:             cfg,
	}
}

// startSupervisedTasks launches the file and state watcher goroutines under
// the supervisor.
func (g *GitStatusController) startSupervisedTasks() {
	if g.fileWatcher != nil {
		g.supervisor.Start("git.file_watcher", g.fileWatcher.Run, supervisor.WithBackoff(supervisorBackoff))
	}
	if g.stateWatcher != nil {
		g.supervisor.Start("app.state_watcher", g.stateWatcher.Run, supervisor.WithBackoff(supervisorBackoff))
	}
}

// Shutdown closes the file and state watchers. It is safe to call when
// either watcher is nil.
func (g *GitStatusController) Shutdown() {
	if g.fileWatcher != nil {
		_ = g.fileWatcher.Close()
	}
	if g.stateWatcher != nil {
		_ = g.stateWatcher.Close()
	}
}

// startFileWatcher returns a tea.Cmd that blocks until a file watcher event
// arrives. Returns nil when the watcher or channel is not available.
func (g *GitStatusController) startFileWatcher() tea.Cmd {
	if g.fileWatcher == nil || g.fileWatcherCh == nil {
		return nil
	}
	return func() tea.Msg {
		return <-g.fileWatcherCh
	}
}

// startStateWatcher returns a tea.Cmd that blocks until a state watcher event
// arrives. Returns nil when the watcher or channel is not available.
func (g *GitStatusController) startStateWatcher() tea.Cmd {
	if g.stateWatcher == nil || g.stateWatcherCh == nil {
		return nil
	}
	return func() tea.Msg {
		return <-g.stateWatcherCh
	}
}

// watchRoot starts watching a workspace root directory for file changes.
// It records the error (once) when the OS watch limit is hit.
func (g *GitStatusController) watchRoot(root string) error {
	if g.fileWatcher == nil {
		return nil
	}
	if err := g.fileWatcher.Watch(root); err != nil {
		logging.Warn("File watcher error: %v", err)
		if errors.Is(err, git.ErrWatchLimit) && g.fileWatcherErr == nil {
			g.fileWatcherErr = err
		}
		return err
	}
	return nil
}

// unwatchRoot stops watching a workspace root directory.
func (g *GitStatusController) unwatchRoot(root string) {
	if g.fileWatcher != nil {
		g.fileWatcher.Unwatch(root)
	}
}

// isWatchLimitReached reports whether the file watcher has hit the OS limit.
func (g *GitStatusController) isWatchLimitReached() bool {
	return g.fileWatcherErr != nil && errors.Is(g.fileWatcherErr, git.ErrWatchLimit)
}

// fileWatcherInitErr returns the initialisation error (if any) from creating
// the file watcher.
func (g *GitStatusController) fileWatcherInitErr() error {
	return g.fileWatcherErr
}

// stateWatcherInitErr returns the initialisation error (if any) from creating
// the state watcher.
func (g *GitStatusController) stateWatcherInitErr() error {
	return g.stateWatcherErr
}

// HandleFileWatcherEvent processes a FileWatcherEvent: invalidates caches,
// requests git status (full for active workspace, fast otherwise), and
// re-arms the file watcher.
func (g *GitStatusController) HandleFileWatcherEvent(msg messages.FileWatcherEvent) []tea.Cmd {
	requestRoot := msg.Root
	requestFull := false
	if g.cfg.GitStatusService != nil {
		g.cfg.GitStatusService.Invalidate(msg.Root)
	}
	if g.cfg.Dashboard != nil {
		g.cfg.Dashboard.InvalidateStatus(msg.Root)
	}
	activeRoot := g.cfg.ActiveWorkspaceRoot()
	if activeRoot != "" && rootsReferToSameWorkspace(msg.Root, activeRoot) {
		requestRoot = activeRoot
		requestFull = true
		if g.cfg.GitStatusService != nil {
			g.cfg.GitStatusService.Invalidate(requestRoot)
		}
		if g.cfg.Dashboard != nil {
			g.cfg.Dashboard.InvalidateStatus(requestRoot)
		}
	}
	statusCmd := g.requestGitStatus(requestRoot)
	if requestFull {
		statusCmd = g.requestGitStatusFull(requestRoot)
	}
	return []tea.Cmd{
		statusCmd,
		g.startFileWatcher(),
	}
}

// HandleStateWatcherEvent processes a StateWatcherEvent: suppresses
// workspace reload when the event originated from a local save, otherwise
// triggers a project reload and re-arms the state watcher.
func (g *GitStatusController) HandleStateWatcherEvent(msg messages.StateWatcherEvent) []tea.Cmd {
	if msg.Reason == "workspaces" && g.cfg.ShouldSuppressReload != nil && g.cfg.ShouldSuppressReload(msg.Paths, time.Now()) {
		return []tea.Cmd{
			g.startStateWatcher(),
		}
	}
	var cmds []tea.Cmd
	if g.cfg.LoadProjects != nil {
		cmds = append(cmds, g.cfg.LoadProjects())
	}
	return append(cmds, g.startStateWatcher())
}

// HandleGitStatusTick processes a GitStatusTick: requests cached git status
// for the active workspace (falls back to full refresh on cache miss),
// syncs the dashboard, and re-arms the ticker.
func (g *GitStatusController) HandleGitStatusTick() []tea.Cmd {
	var cmds []tea.Cmd
	root := g.cfg.ActiveWorkspaceRoot()
	if root != "" {
		cmds = append(cmds, g.requestGitStatusCached(root, true))
	}
	if g.cfg.SyncDashboard != nil {
		g.cfg.SyncDashboard()
	}
	if g.cfg.StartTicker != nil {
		cmds = append(cmds, g.cfg.StartTicker())
	}
	return cmds
}

// HandleGitStatusResult processes a GitStatusResult: updates the dashboard
// and sidebar model with the git status information.
func (g *GitStatusController) HandleGitStatusResult(msg messages.GitStatusResult) []tea.Cmd {
	var cmds []tea.Cmd
	if g.cfg.Dashboard != nil {
		newDashboard, cmd := g.cfg.Dashboard.Update(msg)
		g.cfg.Dashboard = newDashboard
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	activeRoot := g.cfg.ActiveWorkspaceRoot()
	if g.cfg.Sidebar != nil && activeRoot != "" && rootsReferToSameWorkspace(msg.Root, activeRoot) {
		g.cfg.Sidebar.SetGitStatus(msg.Status)
	}
	return cmds
}

// requestGitStatus returns a command that performs a fast git status refresh.
func (g *GitStatusController) requestGitStatus(root string) tea.Cmd {
	return func() tea.Msg {
		if g.cfg.GitStatusService == nil {
			return messages.GitStatusResult{Root: root}
		}
		status, err := g.cfg.GitStatusService.RefreshFast(root)
		if err == nil {
			g.cfg.GitStatusService.UpdateCache(root, status)
		}
		return messages.GitStatusResult{Root: root, Status: status, Err: err}
	}
}

// requestGitStatusFull returns a command that performs a full git status refresh with line stats.
func (g *GitStatusController) requestGitStatusFull(root string) tea.Cmd {
	return func() tea.Msg {
		if g.cfg.GitStatusService == nil {
			return messages.GitStatusResult{Root: root}
		}
		status, err := g.cfg.GitStatusService.Refresh(root)
		if err == nil {
			g.cfg.GitStatusService.UpdateCache(root, status)
		}
		return messages.GitStatusResult{Root: root, Status: status, Err: err}
	}
}

// requestGitStatusCached returns a command that serves cached git status
// when available, falling back to full or fast refresh otherwise.
func (g *GitStatusController) requestGitStatusCached(root string, fallbackToFull bool) tea.Cmd {
	if g.cfg.GitStatusService != nil {
		if cached := g.cfg.GitStatusService.GetCached(root); cached != nil {
			return func() tea.Msg {
				return messages.GitStatusResult{Root: root, Status: cached}
			}
		}
	}
	if fallbackToFull {
		return g.requestGitStatusFull(root)
	}
	return g.requestGitStatus(root)
}
