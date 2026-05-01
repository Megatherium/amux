package dashboard

import (
	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/data" //nolint:depguard // existing architectural import, see bmx-zlc.2
	"github.com/andyrewlee/amux/internal/messages"
)

// isSelectable returns whether a row type can be selected
func isSelectable(rt RowType) bool {
	switch rt {
	case RowSpacer:
		return false
	default:
		return true
	}
}

// findSelectableRow finds a selectable row starting from 'from' in direction 'dir'.
// Returns -1 if none found.
func (m *Model) findSelectableRow(from, dir int) int {
	if dir == 0 {
		dir = 1 // Default to forward
	}
	for i := from; i >= 0 && i < len(m.rows); i += dir {
		if isSelectable(m.rows[i].Type) {
			return i
		}
	}
	return -1
}

// moveCursor moves the cursor by delta, skipping non-selectable rows
func (m *Model) moveCursor(delta int) {
	if len(m.rows) == 0 {
		return
	}

	// Determine direction and number of steps
	steps := delta
	if steps < 0 {
		steps = -steps
	}
	direction := 1
	if delta < 0 {
		direction = -1
	}

	// Walk row-by-row, skipping non-selectable rows
	for step := 0; step < steps; step++ {
		next := m.findSelectableRow(m.cursor+direction, direction)
		if next == -1 {
			// No more selectable rows in this direction
			break
		}
		m.cursor = next
	}
}

func rowLineCount(row Row) int {
	return 1
}

func (m *Model) rowIndexAt(screenX, screenY int) (int, bool) {
	borderTop := 1
	borderLeft := 1
	borderRight := 1
	paddingLeft := 0
	paddingRight := 1

	contentX := screenX - borderLeft - paddingLeft
	contentY := screenY - borderTop

	contentWidth := m.width - (borderLeft + borderRight + paddingLeft + paddingRight)
	innerHeight := m.height - 2
	if contentWidth <= 0 || innerHeight <= 0 {
		return -1, false
	}
	if contentX < 0 || contentX >= contentWidth {
		return -1, false
	}
	if contentY < 0 || contentY >= innerHeight {
		return -1, false
	}

	headerHeight := 0
	helpHeight := m.helpLineCount()
	toolbarHeight := m.toolbarHeight()
	rowAreaHeight := innerHeight - headerHeight - toolbarHeight - helpHeight
	if rowAreaHeight < 1 {
		rowAreaHeight = 1
	}

	if contentY < headerHeight || contentY >= headerHeight+rowAreaHeight {
		return -1, false
	}

	rowY := contentY - headerHeight
	line := 0
	for i := m.scrollOffset; i < len(m.rows); i++ {
		if line >= rowAreaHeight {
			break
		}
		rowLines := rowLineCount(m.rows[i])
		if rowY < line+rowLines {
			return i, true
		}
		line += rowLines
	}

	return -1, false
}

// activateCurrentRow returns a command to activate the currently selected row.
// This is called automatically on cursor movement for instant content switching.
// Returns nil for rows that shouldn't auto-activate (like RowCreate which opens a dialog).
func (m *Model) activateCurrentRow() tea.Cmd {
	if m.cursor >= len(m.rows) {
		return nil
	}

	row := m.rows[m.cursor]
	switch row.Type {
	case RowHome:
		return func() tea.Msg { return messages.ShowWelcome{} }
	case RowProject:
		// Find and activate the main/primary workspace for this project
		var mainWS *data.Workspace
		for i := range row.Project.Workspaces {
			ws := &row.Project.Workspaces[i]
			if ws.IsMainBranch() || ws.IsPrimaryCheckout() {
				mainWS = ws
				break
			}
		}
		if mainWS != nil {
			return func() tea.Msg {
				return messages.WorkspaceActivated{
					Project:   row.Project,
					Workspace: mainWS,
					Preview:   true,
				}
			}
		}
		return nil
	case RowWorkspace:
		return func() tea.Msg {
			return messages.WorkspaceActivated{
				Project:   row.Project,
				Workspace: row.Workspace,
				Preview:   true,
			}
		}
	case RowTicket:
		// Emit a preview message so the center pane or sidebar can show
		// ticket info. This is distinct from TicketSelectedMsg (Enter) which
		// starts the draft flow.
		return func() tea.Msg {
			return messages.TicketPreviewMsg{Ticket: row.Ticket, Project: row.Project}
		}
	case RowTicketsHeader:
		// No activation action; user presses Space to toggle collapse.
		return nil
	default:
		// RowCreate, RowAddProject, RowSpacer — these don't emit their own
		// preview-clearing messages (unlike RowHome→ShowWelcome or
		// RowProject/RowWorkspace→WorkspaceActivated), so we explicitly
		// clear any stale ticket preview.
		return func() tea.Msg { return messages.TicketPreviewMsg{Ticket: nil, Project: nil} }
	}
}

// handleEnter handles the enter key
func (m *Model) handleEnter() tea.Cmd {
	if m.cursor >= len(m.rows) {
		return nil
	}

	row := m.rows[m.cursor]
	switch row.Type {
	case RowHome:
		return func() tea.Msg { return messages.ShowWelcome{} }
	case RowProject:
		// Find and activate the main/primary workspace for this project
		var mainWS *data.Workspace
		for i := range row.Project.Workspaces {
			ws := &row.Project.Workspaces[i]
			if ws.IsMainBranch() || ws.IsPrimaryCheckout() {
				mainWS = ws
				break
			}
		}
		if mainWS != nil {
			return func() tea.Msg {
				return messages.WorkspaceActivated{
					Project:   row.Project,
					Workspace: mainWS,
				}
			}
		}
		return nil
	case RowWorkspace:
		return func() tea.Msg {
			return messages.WorkspaceActivated{
				Project:   row.Project,
				Workspace: row.Workspace,
			}
		}
	case RowTicket:
		return func() tea.Msg {
			return messages.TicketSelectedMsg{Ticket: row.Ticket, Project: row.Project}
		}
	case RowTicketsHeader:
		// No activation; use Space to toggle collapse.
		return nil
	case RowCreate:
		return func() tea.Msg {
			return messages.ShowCreateWorkspaceDialog{Project: row.Project}
		}
	}

	return nil
}

// handleDelete handles the delete key
func (m *Model) handleDelete() tea.Cmd {
	if m.cursor >= len(m.rows) {
		return nil
	}

	row := m.rows[m.cursor]
	if row.Type == RowWorkspace && row.Workspace != nil {
		return func() tea.Msg {
			return messages.ShowDeleteWorkspaceDialog{
				Project:   row.Project,
				Workspace: row.Workspace,
			}
		}
	}
	if row.Type == RowProject && row.Project != nil {
		return func() tea.Msg {
			return messages.ShowRemoveProjectDialog{
				Project: row.Project,
			}
		}
	}

	return nil
}

// refresh requests a workspace rescan/import.
func (m *Model) refresh() tea.Cmd {
	return func() tea.Msg { return messages.RescanWorkspaces{} }
}

// handleSpace handles the space key for toggling collapsible sections.
func (m *Model) handleSpace() tea.Cmd {
	if m.cursor >= len(m.rows) {
		return nil
	}
	row := m.rows[m.cursor]
	if row.Type == RowTicketsHeader && row.Project != nil {
		path := row.Project.Path
		m.ticketsCollapsed[path] = !m.ticketsCollapsed[path]
		m.rebuildRows()
		return m.activateCurrentRow()
	}
	return nil
}

// ticketCount returns the number of cached tickets for a project path.
func (m *Model) ticketCount(projectPath string) int {
	if m.ticketCache == nil {
		return 0
	}
	return len(m.ticketCache[projectPath])
}
