package app

import (
	"testing"

	"github.com/andyrewlee/amux/internal/app/workspaces"
	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/ui/center"
	"github.com/andyrewlee/amux/internal/ui/dashboard"
	"github.com/andyrewlee/amux/internal/ui/sidebar"
)

func TestHandleWorkspaceDeletedClearsDirtyWorkspaceMarker(t *testing.T) {
	ws := data.NewWorkspace("feature", "feature", "main", "/repo", "/repo/feature")
	wsID := string(ws.ID())

	app := &App{
		ui: &UICompositor{
			dashboard:       dashboard.New(),
			center:          center.New(nil),
			sidebar:         sidebar.NewTabbedSidebar(),
			sidebarTerminal: sidebar.NewTerminalModel(),
		},
		workspaceManager: workspaces.NewManagerWithConfig(workspaces.ManagerConfig{
			DirtyWorkspaceIDs:    map[string]bool{wsID: true},
			DeletingWorkspaceIDs: map[string]bool{wsID: true},
		}),
	}

	app.handleWorkspaceDeleted(messages.WorkspaceDeleted{Workspace: ws})

	if app.isWorkspaceDeleteInFlight(wsID) {
		t.Fatal("expected delete-in-flight marker to be cleared on delete success")
	}
	if app.wm().IsWorkspaceDirty(wsID) {
		t.Fatal("expected dirty workspace marker to be cleared on delete success")
	}
}
