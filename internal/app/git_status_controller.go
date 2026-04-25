package app

import (
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/git"
	"github.com/andyrewlee/amux/internal/logging"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/supervisor"
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
}

// newGitStatusController creates a GitStatusController, initializing the
// file and state watchers (including their event channels).
func newGitStatusController(registryPath, metadataRoot string, supervisor *supervisor.Supervisor) *GitStatusController {
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
