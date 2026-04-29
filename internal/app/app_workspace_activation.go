package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/messages"
)

// setWorkspaceActivationState updates app-level state fields and propagates
// the activated workspace to all UI sub-components.
func (a *App) setWorkspaceActivationState(msg messages.WorkspaceActivated) {
	a.activeProject = msg.Project
	a.activeWorkspace = msg.Workspace
	a.showWelcome = false
	a.ui.centerBtnFocused = false
	a.ui.centerBtnIndex = 0
	a.ui.previewTicket = nil
	a.ui.previewProject = nil
	a.ui.center.SetWorkspace(msg.Workspace)
	a.ui.center.SetHasTicketService(a.hasTicketService())
	a.ui.sidebar.SetWorkspace(msg.Workspace)
	a.ui.sidebar.SetPreviewTicket(nil)
	a.ui.sidebarTerminal.SetWorkspacePreview(msg.Workspace)
}

// discoverWorkspaceTmux returns commands for tmux tab discovery, sidebar
// terminal discovery, tab sync, and persisted tab restore for the given
// workspace.
func (a *App) discoverWorkspaceTmux(ws *data.Workspace) []tea.Cmd {
	var cmds []tea.Cmd
	if discoverCmd := a.discoverWorkspaceTabsFromTmux(ws); discoverCmd != nil {
		cmds = append(cmds, discoverCmd)
	}
	if discoverTermCmd := a.discoverSidebarTerminalsFromTmux(ws); discoverTermCmd != nil {
		cmds = append(cmds, discoverTermCmd)
	}
	if syncCmd := a.syncWorkspaceTabsFromTmux(ws); syncCmd != nil {
		cmds = append(cmds, syncCmd)
	}
	if restoreCmd := a.ui.center.RestoreTabsFromWorkspace(ws); restoreCmd != nil {
		cmds = append(cmds, restoreCmd)
	}
	return cmds
}

// routeFocusOnActivation handles focus pane routing on explicit (non-preview)
// workspace activation. It returns true if a center reattach was already
// queued, so the caller can skip the fallback reattach.
func (a *App) routeFocusOnActivation(msg messages.WorkspaceActivated, cmds *[]tea.Cmd) bool {
	if msg.Workspace == nil || msg.Preview {
		return false
	}
	wsID := string(msg.Workspace.ID())
	centerVisible := a.ui.layout != nil && a.ui.layout.ShowCenter()
	if centerVisible {
		hasCenterTabs := false
		if tabs, _ := a.ui.center.GetTabsInfoForWorkspace(wsID); len(tabs) > 0 {
			hasCenterTabs = true
		}
		if !hasCenterTabs && len(msg.Workspace.OpenTabs) > 0 {
			hasCenterTabs = true
		}
		if hasCenterTabs {
			// focusPane(PaneCenter) already performs the reattach attempt;
			// mark it as queued regardless of returned command to avoid
			// coupling deduplication to a nil/non-nil command shape.
			focusCmd := a.focusPane(messages.PaneCenter)
			if focusCmd != nil {
				*cmds = append(*cmds, focusCmd)
			}
			return true
		}
	}
	if !centerVisible {
		if focusCmd := a.focusPane(messages.PaneDashboard); focusCmd != nil {
			*cmds = append(*cmds, focusCmd)
		}
	}
	return false
}

// refreshWorkspaceResources requests a full git status refresh and sets up
// file watching for the activated workspace root.
func (a *App) refreshWorkspaceResources(ws *data.Workspace) []tea.Cmd {
	if ws == nil {
		return nil
	}
	var cmds []tea.Cmd
	if a.gitStatusController != nil {
		cmds = append(cmds, a.gitStatusController.requestGitStatusFull(ws.Root))
		if err := a.gitStatusController.watchRoot(ws.Root); err != nil {
			if a.gitStatusController.isWatchLimitReached() {
				cmds = append(cmds, a.ui.toast.ShowWarning("File watching disabled (watch limit reached); git status may be stale"))
			}
		}
	}
	return cmds
}
