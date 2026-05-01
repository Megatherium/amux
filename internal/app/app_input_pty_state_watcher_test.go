package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/config"
	"github.com/andyrewlee/amux/internal/messages"

	"github.com/andyrewlee/amux/internal/app/workspaces"
)

func TestHandleStateWatcherEvent_SuppressesSelfOriginatedWorkspaceReload(t *testing.T) {
	metadataRoot := filepath.Join(t.TempDir(), "meta")
	localPath := filepath.Join(metadataRoot, "abc123", "workspace.json")
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(localPath): %v", err)
	}
	if err := os.WriteFile(localPath, []byte(`{"tabs":[]}`), 0o644); err != nil {
		t.Fatalf("WriteFile(localPath): %v", err)
	}
	app := &App{
		workspaceService: workspaces.NewService(nil, nil, nil, ""),
		config: &config.Config{
			Paths: &config.Paths{MetadataRoot: metadataRoot},
		},
	}
	app.markLocalWorkspaceSavePath(localPath)

	ctrl := &GitStatusController{
		stateWatcher:   &stateWatcher{},
		stateWatcherCh: make(chan messages.StateWatcherEvent, 1),
		cfg: GitStatusControllerConfig{
			ShouldSuppressReload: func(paths []string, now time.Time) bool {
				return app.shouldSuppressWorkspaceReload(paths, now)
			},
		},
	}

	cmds := ctrl.HandleStateWatcherEvent(messages.StateWatcherEvent{
		Reason: "workspaces",
		Paths:  []string{localPath},
	})
	if len(cmds) != 1 {
		t.Fatalf("expected only state watcher restart command, got %d commands", len(cmds))
	}
	if cmds[0] == nil {
		t.Fatal("expected non-nil state watcher restart command")
	}
}

func TestHandleStateWatcherEvent_DoesNotSuppressExternalWorkspaceReload(t *testing.T) {
	metadataRoot := filepath.Join(t.TempDir(), "meta")
	localPath := filepath.Join(metadataRoot, "abc123", "workspace.json")
	externalPath := filepath.Join(metadataRoot, "def456", "workspace.json")
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(localPath): %v", err)
	}
	if err := os.WriteFile(localPath, []byte(`{"tabs":[]}`), 0o644); err != nil {
		t.Fatalf("WriteFile(localPath): %v", err)
	}
	app := &App{
		workspaceService: workspaces.NewService(nil, nil, nil, ""),
		config: &config.Config{
			Paths: &config.Paths{MetadataRoot: metadataRoot},
		},
	}
	app.markLocalWorkspaceSavePath(localPath)

	ctrl := &GitStatusController{
		stateWatcher:   &stateWatcher{},
		stateWatcherCh: make(chan messages.StateWatcherEvent, 1),
		cfg: GitStatusControllerConfig{
			ShouldSuppressReload: func(paths []string, now time.Time) bool {
				return app.shouldSuppressWorkspaceReload(paths, now)
			},
			LoadProjects: func() tea.Cmd { return func() tea.Msg { return messages.ProjectsLoaded{} } },
		},
	}

	cmds := ctrl.HandleStateWatcherEvent(messages.StateWatcherEvent{
		Reason: "workspaces",
		Paths:  []string{externalPath},
	})
	if len(cmds) != 2 {
		t.Fatalf("expected loadProjects + state watcher restart commands, got %d commands", len(cmds))
	}
	if cmds[0] == nil {
		t.Fatal("expected non-nil loadProjects command")
	}
	if cmds[1] == nil {
		t.Fatal("expected non-nil state watcher restart command")
	}
}

func TestHandleStateWatcherEvent_DoesNotSuppressSamePathWhenFileChanged(t *testing.T) {
	metadataRoot := filepath.Join(t.TempDir(), "meta")
	localPath := filepath.Join(metadataRoot, "abc123", "workspace.json")
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(localPath): %v", err)
	}
	if err := os.WriteFile(localPath, []byte(`{"tabs":[1]}`), 0o644); err != nil {
		t.Fatalf("WriteFile(localPath first): %v", err)
	}

	app := &App{
		workspaceService: workspaces.NewService(nil, nil, nil, ""),
		config: &config.Config{
			Paths: &config.Paths{MetadataRoot: metadataRoot},
		},
	}
	app.markLocalWorkspaceSavePath(localPath)

	if err := os.WriteFile(localPath, []byte(`{"tabs":[2]}`), 0o644); err != nil {
		t.Fatalf("WriteFile(localPath second): %v", err)
	}

	ctrl := &GitStatusController{
		stateWatcher:   &stateWatcher{},
		stateWatcherCh: make(chan messages.StateWatcherEvent, 1),
		cfg: GitStatusControllerConfig{
			ShouldSuppressReload: func(paths []string, now time.Time) bool {
				return app.shouldSuppressWorkspaceReload(paths, now)
			},
			LoadProjects: func() tea.Cmd { return func() tea.Msg { return messages.ProjectsLoaded{} } },
		},
	}

	cmds := ctrl.HandleStateWatcherEvent(messages.StateWatcherEvent{
		Reason: "workspaces",
		Paths:  []string{localPath},
	})
	if len(cmds) != 2 {
		t.Fatalf("expected loadProjects + state watcher restart commands, got %d commands", len(cmds))
	}
	if cmds[0] == nil {
		t.Fatal("expected non-nil loadProjects command")
	}
	if cmds[1] == nil {
		t.Fatal("expected non-nil state watcher restart command")
	}
}

func TestHandleStateWatcherEvent_LoadsProjectsWithoutRecentLocalSave(t *testing.T) {
	ctrl := &GitStatusController{
		stateWatcher:   &stateWatcher{},
		stateWatcherCh: make(chan messages.StateWatcherEvent, 1),
		cfg: GitStatusControllerConfig{
			ShouldSuppressReload: func(paths []string, now time.Time) bool {
				return false
			},
			LoadProjects: func() tea.Cmd { return func() tea.Msg { return messages.ProjectsLoaded{} } },
		},
	}

	cmds := ctrl.HandleStateWatcherEvent(messages.StateWatcherEvent{Reason: "workspaces"})
	if len(cmds) != 2 {
		t.Fatalf("expected loadProjects + state watcher restart commands, got %d commands", len(cmds))
	}
	if cmds[0] == nil {
		t.Fatal("expected non-nil loadProjects command")
	}
	if cmds[1] == nil {
		t.Fatal("expected non-nil state watcher restart command")
	}
}
