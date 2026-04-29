package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/messages"
)

// handleTicketSelected handles a ticket selection from the dashboard,
// starting the draft flow in the center pane.
func (a *App) handleTicketSelected(msg messages.TicketSelectedMsg) []tea.Cmd {
	if msg.Ticket == nil {
		return nil
	}

	project := msg.Project
	if project == nil {
		return []tea.Cmd{a.ui.toast.ShowError("No project associated with this ticket")}
	}

	var mainWS *data.Workspace
	for i := range project.Workspaces {
		ws := &project.Workspaces[i]
		if ws.IsMainBranch() || ws.IsPrimaryCheckout() {
			mainWS = ws
			break
		}
	}
	if mainWS == nil {
		return []tea.Cmd{a.ui.toast.ShowError("No workspace found for project " + project.Name)}
	}

	var cmds []tea.Cmd
	if a.activeWorkspace == nil || string(a.activeWorkspace.ID()) != string(mainWS.ID()) {
		cmds = append(cmds, a.handleWorkspaceActivated(messages.WorkspaceActivated{
			Project:   project,
			Workspace: mainWS,
		})...)
	}

	a.ui.center.StartDraft(msg.Ticket, mainWS)
	cmds = append(cmds, a.focusPane(messages.PaneCenter))
	return cmds
}

// handleTicketPreview updates the preview state when the cursor hovers over
// a ticket row in the dashboard. When no agent tabs are open, the ticket info
// is shown in the center pane. When agent tabs exist, the info is shown in the
// sidebar's ticket tab (without auto-focusing the sidebar).
func (a *App) handleTicketPreview(msg messages.TicketPreviewMsg) {
	a.ui.previewTicket = msg.Ticket
	a.ui.previewProject = msg.Project

	// Forward the preview to the sidebar for the ticket tab
	if a.ui.sidebar != nil {
		a.ui.sidebar.SetPreviewTicket(msg.Ticket)
	}
}
