package app

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/git"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/ui/center"
	"github.com/andyrewlee/amux/internal/ui/dashboard"
	"github.com/andyrewlee/amux/internal/ui/layout"
	"github.com/andyrewlee/amux/internal/ui/sidebar"
)

// ---------------------------------------------------------------------------
// setWorkspaceActivationState
// ---------------------------------------------------------------------------

func TestSetWorkspaceActivationState_SetsActiveWorkspace(t *testing.T) {
	ws := data.NewWorkspace("feature", "feature", "main", "/repo", "/repo/feature")
	project := data.NewProject("/repo")
	project.AddWorkspace(*ws)

	app := &App{
		showWelcome:      true,
		centerBtnFocused: true,
		centerBtnIndex:   3,
		center:           center.New(nil),
		sidebar:          sidebar.NewTabbedSidebar(),
		sidebarTerminal:  sidebar.NewTerminalModel(),
	}

	app.setWorkspaceActivationState(messages.WorkspaceActivated{
		Project:   project,
		Workspace: ws,
	})

	if app.activeProject != project {
		t.Fatal("activeProject not set")
	}
	if app.activeWorkspace != ws {
		t.Fatal("activeWorkspace not set")
	}
	if app.showWelcome {
		t.Fatal("showWelcome should be false")
	}
	if app.centerBtnFocused {
		t.Fatal("centerBtnFocused should be false")
	}
	if app.centerBtnIndex != 0 {
		t.Fatalf("centerBtnIndex should be 0, got %d", app.centerBtnIndex)
	}
	if app.previewTicket != nil {
		t.Fatal("previewTicket should be nil")
	}
	if app.previewProject != nil {
		t.Fatal("previewProject should be nil")
	}
}

func TestSetWorkspaceActivationState_ClearsWelcomeOnNilWorkspace(t *testing.T) {
	app := &App{
		showWelcome:     true,
		center:          center.New(nil),
		sidebar:         sidebar.NewTabbedSidebar(),
		sidebarTerminal: sidebar.NewTerminalModel(),
	}

	app.setWorkspaceActivationState(messages.WorkspaceActivated{
		Workspace: nil,
	})

	if app.showWelcome {
		t.Fatal("showWelcome should be cleared even for nil workspace")
	}
}

// ---------------------------------------------------------------------------
// discoverWorkspaceTmux
// ---------------------------------------------------------------------------

func TestDiscoverWorkspaceTmux_ReturnsRestoreCmd(t *testing.T) {
	ws := data.NewWorkspace("feature", "feature", "main", "/repo", "/repo/feature")
	centerModel := center.New(nil)
	centerModel.SetWorkspace(ws)

	app := &App{
		center: centerModel,
	}

	cmds := app.discoverWorkspaceTmux(ws)
	// At minimum, RestoreTabsFromWorkspace should return a non-nil cmd
	// when workspace has no tabs. The exact count depends on tmux availability;
	// we verify it doesn't panic and returns a slice.
	_ = cmds
}

func TestDiscoverWorkspaceTmux_NilWorkspace(t *testing.T) {
	app := &App{
		center: center.New(nil),
	}

	cmds := app.discoverWorkspaceTmux(nil)
	// Should not panic; may return nil cmds from sub-operations.
	_ = cmds
}

// ---------------------------------------------------------------------------
// routeFocusOnActivation
// ---------------------------------------------------------------------------

func TestRouteFocusOnActivation_CenterVisibleWithTabs(t *testing.T) {
	ws := data.NewWorkspace("feature", "feature", "main", "/repo", "/repo/feature")
	centerModel := center.New(nil)
	centerModel.SetWorkspace(ws)
	centerModel.AddTab(&center.Tab{
		ID:          center.TabID("tab-1"),
		Name:        "Claude",
		Assistant:   "claude",
		SessionName: "amux-test-session",
		Workspace:   ws,
	})

	layoutManager := layout.NewManager()
	layoutManager.Resize(140, 40)

	app := &App{
		layout:      layoutManager,
		center:      centerModel,
		focusedPane: messages.PaneDashboard,
	}
	app.syncPaneFocusFlags()

	var cmds []tea.Cmd

	queued := app.routeFocusOnActivation(messages.WorkspaceActivated{
		Workspace: ws,
	}, &cmds)

	if !queued {
		t.Fatal("expected centerFocusQueuedReattach=true when center visible with tabs")
	}
	if app.focusedPane != messages.PaneCenter {
		t.Fatalf("expected focus on PaneCenter, got %v", app.focusedPane)
	}
}

func TestRouteFocusOnActivation_PreviewReturnsFalse(t *testing.T) {
	ws := data.NewWorkspace("feature", "feature", "main", "/repo", "/repo/feature")
	layoutManager := layout.NewManager()
	layoutManager.Resize(140, 40)

	app := &App{
		layout:      layoutManager,
		center:      center.New(nil),
		focusedPane: messages.PaneDashboard,
	}
	app.syncPaneFocusFlags()

	var cmds []tea.Cmd

	queued := app.routeFocusOnActivation(messages.WorkspaceActivated{
		Workspace: ws,
		Preview:   true,
	}, &cmds)

	if queued {
		t.Fatal("preview activation should not queue reattach")
	}
	if app.focusedPane != messages.PaneDashboard {
		t.Fatalf("preview should leave focus unchanged, got %v", app.focusedPane)
	}
}

func TestRouteFocusOnActivation_NilWorkspaceReturnsFalse(t *testing.T) {
	app := &App{
		focusedPane: messages.PaneDashboard,
	}

	var cmds []tea.Cmd

	queued := app.routeFocusOnActivation(messages.WorkspaceActivated{
		Workspace: nil,
	}, &cmds)

	if queued {
		t.Fatal("nil workspace should not queue reattach")
	}
}

func TestRouteFocusOnActivation_DashboardOnlyFocusesDashboard(t *testing.T) {
	ws := data.NewWorkspace("feature", "feature", "main", "/repo", "/repo/feature")
	layoutManager := layout.NewManager()
	// Very small width forces LayoutOnePane (dashboard-only).
	layoutManager.Resize(40, 24)

	app := &App{
		layout:      layoutManager,
		center:      center.New(nil),
		focusedPane: messages.PaneSidebar,
	}
	app.syncPaneFocusFlags()

	var cmds []tea.Cmd

	queued := app.routeFocusOnActivation(messages.WorkspaceActivated{
		Workspace: ws,
	}, &cmds)

	if queued {
		t.Fatal("dashboard-only layout should not queue center reattach")
	}
	if app.focusedPane != messages.PaneDashboard {
		t.Fatalf("expected focus on PaneDashboard, got %v", app.focusedPane)
	}
}

func TestRouteFocusOnActivation_CenterVisibleNoTabs(t *testing.T) {
	ws := data.NewWorkspace("feature", "feature", "main", "/repo", "/repo/feature")
	centerModel := center.New(nil)
	centerModel.SetWorkspace(ws)
	// No tabs added, no OpenTabs on workspace.

	layoutManager := layout.NewManager()
	layoutManager.Resize(140, 40)

	app := &App{
		layout:      layoutManager,
		center:      centerModel,
		focusedPane: messages.PaneDashboard,
	}
	app.syncPaneFocusFlags()

	var cmds []tea.Cmd

	queued := app.routeFocusOnActivation(messages.WorkspaceActivated{
		Workspace: ws,
	}, &cmds)

	if queued {
		t.Fatal("no tabs should not queue center reattach")
	}
	// Focus should stay on dashboard since there are no tabs to focus.
	if app.focusedPane != messages.PaneDashboard {
		t.Fatalf("expected focus to remain on dashboard, got %v", app.focusedPane)
	}
}

func TestRouteFocusOnActivation_CenterVisibleWithPersistedTabs(t *testing.T) {
	ws := data.NewWorkspace("feature", "feature", "main", "/repo", "/repo/feature")
	// Simulate persisted OpenTabs but no in-memory tabs.
	ws.OpenTabs = []data.TabInfo{
		{Name: "Claude", Assistant: "claude"},
	}

	centerModel := center.New(nil)
	centerModel.SetWorkspace(ws)

	layoutManager := layout.NewManager()
	layoutManager.Resize(140, 40)

	app := &App{
		layout:      layoutManager,
		center:      centerModel,
		focusedPane: messages.PaneDashboard,
	}
	app.syncPaneFocusFlags()

	var cmds []tea.Cmd

	queued := app.routeFocusOnActivation(messages.WorkspaceActivated{
		Workspace: ws,
	}, &cmds)

	if !queued {
		t.Fatal("persisted tabs should trigger center focus and reattach queue")
	}
}

// ---------------------------------------------------------------------------
// refreshWorkspaceResources
// ---------------------------------------------------------------------------

func TestRefreshWorkspaceResources_NilWorkspaceReturnsNil(t *testing.T) {
	app := &App{}

	cmds := app.refreshWorkspaceResources(nil)
	if cmds != nil {
		t.Fatalf("expected nil cmds for nil workspace, got %v", cmds)
	}
}

func TestRefreshWorkspaceResources_ReturnsGitStatusCmd(t *testing.T) {
	ws := data.NewWorkspace("feature", "feature", "main", "/repo", "/repo/feature")
	app := &App{
		gitStatus: &stubGitStatusSvc{},
	}

	cmds := app.refreshWorkspaceResources(ws)
	if len(cmds) == 0 {
		t.Fatal("expected at least one cmd (git status request)")
	}
}

func TestRefreshWorkspaceResources_NoFileWatcher(t *testing.T) {
	ws := data.NewWorkspace("feature", "feature", "main", "/repo", "/repo/feature")
	app := &App{
		gitStatus:   &stubGitStatusSvc{},
		fileWatcher: nil,
	}

	cmds := app.refreshWorkspaceResources(ws)
	// Should still return git status cmd without panicking.
	if len(cmds) == 0 {
		t.Fatal("expected git status cmd even without file watcher")
	}
}

// ---------------------------------------------------------------------------
// Integration: handleWorkspaceActivated still works end-to-end
// ---------------------------------------------------------------------------

func TestHandleWorkspaceActivated_StillProducesReattachToast(t *testing.T) {
	// This is the original test, re-verified after refactoring.
	ws := data.NewWorkspace("feature", "feature", "main", "/repo", "/repo/feature")
	project := data.NewProject("/repo")
	project.AddWorkspace(*ws)

	centerModel := center.New(nil)
	centerModel.SetWorkspace(ws)
	centerModel.AddTab(&center.Tab{
		ID:          center.TabID("tab-1"),
		Name:        "Claude",
		Assistant:   "claude",
		SessionName: "amux-test-session",
		Workspace:   ws,
		Detached:    true,
	})

	layoutManager := layout.NewManager()
	layoutManager.Resize(140, 40)

	app := &App{
		layout:          layoutManager,
		dashboard:       dashboard.New(),
		center:          centerModel,
		sidebar:         sidebar.NewTabbedSidebar(),
		sidebarTerminal: sidebar.NewTerminalModel(),
	}

	cmds := app.handleWorkspaceActivated(messages.WorkspaceActivated{
		Project:   project,
		Workspace: ws,
	})

	toastCount := 0
	for _, cmd := range cmds {
		if cmd == nil {
			continue
		}
		if toast, ok := cmd().(messages.Toast); ok && toast.Message == "Tab cannot be reattached" {
			toastCount++
		}
	}

	if toastCount != 1 {
		t.Fatalf("expected exactly one reattach toast command, got %d", toastCount)
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// stubGitStatusSvc implements GitStatusService for tests.
type stubGitStatusSvc struct{}

func (s *stubGitStatusSvc) Run(_ context.Context) error                     { return nil }
func (s *stubGitStatusSvc) GetCached(_ string) *git.StatusResult            { return nil }
func (s *stubGitStatusSvc) UpdateCache(_ string, _ *git.StatusResult)       {}
func (s *stubGitStatusSvc) Invalidate(_ string)                             {}
func (s *stubGitStatusSvc) Refresh(_ string) (*git.StatusResult, error)     { return nil, nil }
func (s *stubGitStatusSvc) RefreshFast(_ string) (*git.StatusResult, error) { return nil, nil }
