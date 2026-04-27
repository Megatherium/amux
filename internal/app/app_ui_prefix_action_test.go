package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/ui/layout"
)

func TestRunPrefixAction_AddProject(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)

	cmd := app.runPrefixAction("add_project")
	if cmd == nil {
		t.Fatal("expected add_project to return command")
	}
	msg := cmd()
	if _, ok := msg.(messages.ShowAddProjectDialog); !ok {
		t.Fatalf("expected ShowAddProjectDialog message, got %T", msg)
	}
}

func TestRunPrefixAction_OpenSettings(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)

	cmd := app.runPrefixAction("open_settings")
	if cmd == nil {
		t.Fatal("expected open_settings to return command")
	}
	msg := cmd()
	if _, ok := msg.(messages.ShowSettingsDialog); !ok {
		t.Fatalf("expected ShowSettingsDialog message, got %T", msg)
	}
}

func TestRunPrefixAction_DeleteWorkspaceRequiresSelection(t *testing.T) {
	app, ws, _ := newPrefixTestApp(t)

	if cmd := app.runPrefixAction("delete_workspace"); cmd != nil {
		t.Fatal("expected nil command when no workspace/project selection exists")
	}

	project := &data.Project{Name: "p", Path: "/repo/ws"}
	app.activeProject = project
	app.activeWorkspace = ws
	cmd := app.runPrefixAction("delete_workspace")
	if cmd == nil {
		t.Fatal("expected delete_workspace command when selection exists")
	}
	result := cmd()
	msg, ok := result.(messages.ShowDeleteWorkspaceDialog)
	if !ok {
		t.Fatalf("expected ShowDeleteWorkspaceDialog message, got %T", result)
	}
	if msg.Project != project || msg.Workspace != ws {
		t.Fatalf("unexpected delete payload: %+v", msg)
	}
}

func TestRunPrefixAction_FocusLeftPartialApp_NoPanic(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)
	app.focusedPane = messages.PaneCenter

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("focus_left should not panic on partial app: %v", r)
		}
	}()

	cmd := app.runPrefixAction("focus_left")
	if cmd != nil {
		t.Fatalf("expected nil follow-up command, got %v", cmd)
	}
	if app.focusedPane != messages.PaneDashboard {
		t.Fatalf("expected focused pane dashboard, got %v", app.focusedPane)
	}
}

func TestRunPrefixAction_FocusRightPartialApp_NoPanic(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)
	app.focusedPane = messages.PaneDashboard
	app.ui.layout = layout.NewManager()
	app.ui.layout.Resize(140, 40) // Ensures center pane is visible.

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("focus_right should not panic on partial app: %v", r)
		}
	}()

	_ = app.runPrefixAction("focus_right")
	if app.focusedPane != messages.PaneCenter {
		t.Fatalf("expected focused pane center, got %v", app.focusedPane)
	}
}

func TestRunPrefixAction_ToggleBothSidebars(t *testing.T) {
	app := newLayoutTestApp(t)

	if app.ui.layout.IsCollapsed() {
		t.Fatal("should not be collapsed initially")
	}

	app.runPrefixAction("toggle_both_sidebars")

	if !app.ui.layout.IsCollapsed() {
		t.Fatal("layout should be collapsed after toggle_both_sidebars")
	}
	if !app.ui.layout.ShowCenter() {
		t.Fatal("center should be visible when collapsed")
	}
	if app.ui.layout.ShowDashboard() {
		t.Fatal("dashboard should be hidden when collapsed")
	}
	if app.ui.layout.ShowSidebar() {
		t.Fatal("sidebar should be hidden when collapsed")
	}
}

func TestRunPrefixAction_ToggleBothSidebars_Roundtrip(t *testing.T) {
	app := newLayoutTestApp(t)
	origCenter := app.ui.layout.CenterWidth()

	app.runPrefixAction("toggle_both_sidebars")
	if app.ui.layout.CenterWidth() <= origCenter {
		t.Fatalf("center should expand when collapsed: got %d, had %d", app.ui.layout.CenterWidth(), origCenter)
	}

	app.runPrefixAction("toggle_both_sidebars")
	if app.ui.layout.CenterWidth() != origCenter {
		t.Fatalf("center should restore: got %d, want %d", app.ui.layout.CenterWidth(), origCenter)
	}
}

func TestRunPrefixAction_ToggleDashboard(t *testing.T) {
	app := newLayoutTestApp(t)

	app.runPrefixAction("toggle_dashboard")

	if app.ui.layout.ShowDashboard() {
		t.Fatal("dashboard should be hidden after toggle_dashboard")
	}
	if !app.ui.layout.ShowSidebar() {
		t.Fatal("sidebar should remain visible after toggle_dashboard")
	}
	if !app.ui.layout.ShowCenter() {
		t.Fatal("center should remain visible")
	}
}

func TestRunPrefixAction_ToggleSidebar(t *testing.T) {
	app := newLayoutTestApp(t)

	app.runPrefixAction("toggle_sidebar")

	if app.ui.layout.ShowSidebar() {
		t.Fatal("sidebar should be hidden after toggle_sidebar")
	}
	if !app.ui.layout.ShowDashboard() {
		t.Fatal("dashboard should remain visible after toggle_sidebar")
	}
	if !app.ui.layout.ShowCenter() {
		t.Fatal("center should remain visible")
	}
}

func TestRunPrefixAction_ToggleBoth_RelocatesFocusFromDashboard(t *testing.T) {
	app := newLayoutTestApp(t)
	app.focusedPane = messages.PaneDashboard

	app.runPrefixAction("toggle_both_sidebars")

	if app.focusedPane != messages.PaneCenter {
		t.Fatalf("focus should relocate to center, got %v", app.focusedPane)
	}
}

func TestRunPrefixAction_ToggleBoth_KeepsCenterFocus(t *testing.T) {
	app := newLayoutTestApp(t)
	app.focusedPane = messages.PaneCenter

	app.runPrefixAction("toggle_both_sidebars")

	if app.focusedPane != messages.PaneCenter {
		t.Fatalf("center focus should be preserved, got %v", app.focusedPane)
	}
}

func TestRunPrefixAction_ToggleBoth_RelocatesFocusFromSidebar(t *testing.T) {
	app := newLayoutTestApp(t)
	app.focusedPane = messages.PaneSidebar

	app.runPrefixAction("toggle_both_sidebars")

	if app.focusedPane != messages.PaneCenter {
		t.Fatalf("focus should relocate to center when sidebar is hidden, got %v", app.focusedPane)
	}
}

func TestRunPrefixAction_ToggleBoth_RelocatesFocusFromSidebarTerminal(t *testing.T) {
	app := newLayoutTestApp(t)
	app.focusedPane = messages.PaneSidebarTerminal

	app.runPrefixAction("toggle_both_sidebars")

	if app.focusedPane != messages.PaneCenter {
		t.Fatalf("focus should relocate to center when sidebar terminal is hidden, got %v", app.focusedPane)
	}
}

func TestRunPrefixAction_ToggleDashboard_RelocatesFocusFromDashboard(t *testing.T) {
	app := newLayoutTestApp(t)
	app.focusedPane = messages.PaneDashboard

	app.runPrefixAction("toggle_dashboard")

	if app.focusedPane != messages.PaneCenter {
		t.Fatalf("focus should relocate to center when dashboard is hidden, got %v", app.focusedPane)
	}
}

func TestRunPrefixAction_ToggleSidebar_RelocatesFocusFromSidebar(t *testing.T) {
	app := newLayoutTestApp(t)
	app.focusedPane = messages.PaneSidebar

	app.runPrefixAction("toggle_sidebar")

	if app.focusedPane != messages.PaneCenter {
		t.Fatalf("focus should relocate to center when sidebar is hidden, got %v", app.focusedPane)
	}
}

func TestRunPrefixAction_ToggleSidebar_RelocatesFocusFromSidebarTerminal(t *testing.T) {
	app := newLayoutTestApp(t)
	app.focusedPane = messages.PaneSidebarTerminal

	app.runPrefixAction("toggle_sidebar")

	if app.focusedPane != messages.PaneCenter {
		t.Fatalf("focus should relocate to center when sidebar is hidden, got %v", app.focusedPane)
	}
}

func TestPrefixCommand_bb_MatchesToggleBoth(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)

	app.prefixActive = true
	app.prefixSequence = []string{"b"}
	status, _ := app.handlePrefixCommand(tea.KeyPressMsg{Code: 'b', Text: "b"})
	if status != prefixMatchComplete {
		t.Fatalf("prefix 'b b' should complete, got %v", status)
	}
}

func TestPrefixCommand_bh_IsPartial(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)

	app.prefixActive = true
	app.prefixSequence = []string{"b"}
	status, _ := app.handlePrefixCommand(tea.KeyPressMsg{Code: 'h', Text: "h"})
	if status != prefixMatchComplete {
		t.Fatalf("prefix 'b h' should complete, got %v", status)
	}
}

func TestPrefixCommand_bl_IsPartial(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)

	app.prefixActive = true
	app.prefixSequence = []string{"b"}
	status, _ := app.handlePrefixCommand(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if status != prefixMatchComplete {
		t.Fatalf("prefix 'b l' should complete, got %v", status)
	}
}
