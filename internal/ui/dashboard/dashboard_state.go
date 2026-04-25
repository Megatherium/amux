package dashboard

import (
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/ui/common"
)

// tickSpinner returns a command that ticks the spinner
func (m *Model) tickSpinner() tea.Cmd {
	return common.SafeTick(spinnerInterval, func(t time.Time) tea.Msg {
		return SpinnerTickMsg{}
	})
}

// startSpinnerIfNeeded starts spinner ticks if we have pending activity.
func (m *Model) startSpinnerIfNeeded() tea.Cmd {
	if m.spinnerActive {
		return nil
	}
	if len(m.creatingWorkspaces) == 0 && len(m.deletingWorkspaces) == 0 {
		return nil
	}
	m.spinnerActive = true
	return m.tickSpinner()
}

// StartSpinnerIfNeeded is the public version for external callers.
func (m *Model) StartSpinnerIfNeeded() tea.Cmd {
	return m.startSpinnerIfNeeded()
}

// SetWorkspaceCreating marks a workspace as creating (or clears it).
func (m *Model) SetWorkspaceCreating(ws *data.Workspace, creating bool) tea.Cmd {
	if ws == nil {
		return nil
	}
	if creating {
		m.creatingWorkspaces[ws.Root] = ws
		m.rebuildRows()
		return m.startSpinnerIfNeeded()
	}
	delete(m.creatingWorkspaces, ws.Root)
	m.rebuildRows()
	return nil
}

// SetWorkspaceDeleting marks a workspace as deleting (or clears it).
func (m *Model) SetWorkspaceDeleting(root string, deleting bool) tea.Cmd {
	if deleting {
		m.deletingWorkspaces[root] = true
		return m.startSpinnerIfNeeded()
	}
	delete(m.deletingWorkspaces, root)
	return nil
}

// rebuildRows rebuilds the row list from projects.
func (m *Model) rebuildRows() {
	rows := []Row{
		{Type: RowHome},
		{Type: RowSpacer},
	}

	for i := range m.projects {
		rows = m.appendProjectRows(rows, &m.projects[i])
		rows = append(rows, Row{Type: RowSpacer})
	}

	m.rows = rows
	m.clampCursor()
	m.clampScrollOffset()
}

// appendProjectRows appends all rows for a single project: header, workspaces,
// tickets section, and create button.
func (m *Model) appendProjectRows(rows []Row, project *data.Project) []Row {
	mainWS := m.getMainWorkspace(project)
	mainWSID := ""
	if mainWS != nil {
		mainWSID = string(mainWS.ID())
	}

	rows = append(rows, Row{
		Type:                RowProject,
		Project:             project,
		ActivityWorkspaceID: mainWSID,
		MainWorkspace:       mainWS,
	})

	rows = m.appendWorkspaceRows(rows, project)
	rows = m.appendTicketRows(rows, project)

	rows = append(rows, Row{
		Type:    RowCreate,
		Project: project,
	})

	return rows
}

// appendWorkspaceRows appends RowWorkspace entries for a project's non-main,
// non-primary workspaces, sorted by creation date (descending).
func (m *Model) appendWorkspaceRows(rows []Row, project *data.Project) []Row {
	for _, ws := range m.sortedWorkspaces(project) {
		// Hide main branch - users access via project row
		if ws.IsMainBranch() || ws.IsPrimaryCheckout() {
			continue
		}

		rows = append(rows, Row{
			Type:                RowWorkspace,
			Project:             project,
			Workspace:           ws,
			ActivityWorkspaceID: string(ws.ID()),
		})
	}
	return rows
}

// appendTicketRows appends the tickets header and (if expanded) individual
// ticket rows for a project. Returns rows unchanged if no tickets are cached.
func (m *Model) appendTicketRows(rows []Row, project *data.Project) []Row {
	cached, ok := m.ticketCache[project.Path]
	if !ok || len(cached) == 0 {
		return rows
	}

	rows = append(rows, Row{
		Type:    RowTicketsHeader,
		Project: project,
	})

	if m.ticketsCollapsed[project.Path] {
		return rows
	}

	ticketRows := make([]Row, 0, len(cached))
	for i := range cached {
		ticketRows = append(ticketRows, Row{
			Type:    RowTicket,
			Project: project,
			Ticket:  &cached[i],
		})
	}

	// Reorder to place children under parents
	ticketRows = reorderTicketRows(ticketRows)
	rows = append(rows, ticketRows...)

	return rows
}

// clampCursor ensures the cursor is within bounds and on a selectable row.
func (m *Model) clampCursor() {
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	// Ensure cursor lands on a selectable row (skip spacers).
	if len(m.rows) > 0 && !isSelectable(m.rows[m.cursor].Type) {
		if next := m.findSelectableRow(m.cursor, 1); next != -1 {
			m.cursor = next
		} else if prev := m.findSelectableRow(m.cursor, -1); prev != -1 {
			m.cursor = prev
		}
	}
}

// clampScrollOffset ensures scrollOffset stays within valid bounds.
func (m *Model) clampScrollOffset() {
	maxOffset := len(m.rows) - m.visibleHeight()
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m *Model) sortedWorkspaces(project *data.Project) []*data.Workspace {
	existingRoots := make(map[string]bool, len(project.Workspaces))
	workspaces := make([]*data.Workspace, 0, len(project.Workspaces)+len(m.creatingWorkspaces))

	for i := range project.Workspaces {
		ws := &project.Workspaces[i]
		existingRoots[ws.Root] = true
		workspaces = append(workspaces, ws)
	}

	for _, ws := range m.creatingWorkspaces {
		if ws == nil || ws.Repo != project.Path {
			continue
		}
		if existingRoots[ws.Root] {
			continue
		}
		workspaces = append(workspaces, ws)
	}

	sort.SliceStable(workspaces, func(i, j int) bool {
		if workspaces[i].Created.Equal(workspaces[j].Created) {
			if workspaces[i].Name == workspaces[j].Name {
				return workspaces[i].Root < workspaces[j].Root
			}
			return workspaces[i].Name < workspaces[j].Name
		}
		return workspaces[i].Created.After(workspaces[j].Created)
	})

	return workspaces
}

// isProjectActive returns true if the project's primary workspace is active.
func (m *Model) isProjectActive(p *data.Project) bool {
	if p == nil {
		return false
	}
	mainWS := m.getMainWorkspace(p)
	if mainWS == nil {
		return false
	}
	return m.activeWorkspaceIDs[string(mainWS.ID())]
}

// getMainWorkspace returns the primary or main branch workspace for a project
func (m *Model) getMainWorkspace(p *data.Project) *data.Workspace {
	if p == nil {
		return nil
	}
	for i := range p.Workspaces {
		ws := &p.Workspaces[i]
		if ws.IsMainBranch() || ws.IsPrimaryCheckout() {
			return ws
		}
	}
	return nil
}

// SelectedRow returns the currently selected row
func (m *Model) SelectedRow() *Row {
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		return &m.rows[m.cursor]
	}
	return nil
}

// Projects returns the current projects
func (m *Model) Projects() []data.Project {
	return m.projects
}

// ClearActiveRoot resets the active workspace selection to "Home".
func (m *Model) ClearActiveRoot() {
	m.activeRoot = ""
}

// reorderTicketRows reorders ticket rows so that child tickets
// (tickets with a non-empty ParentID) appear immediately after
// their parent.
func reorderTicketRows(rows []Row) []Row {
	if len(rows) == 0 {
		return rows
	}

	// Build parent index map: ticket ID → position
	parentPos := make(map[string]int, len(rows))
	for i, r := range rows {
		if r.Ticket != nil {
			parentPos[r.Ticket.ID] = i
		}
	}

	// Separate children from parents
	var ordered []Row
	childrenOf := make(map[int][]Row) // parent position → children

	for _, r := range rows {
		if r.Ticket == nil || r.Ticket.ParentID == "" {
			// Parent or root ticket
			ordered = append(ordered, r)
		} else {
			// Child ticket - defer placement
			pidx, ok := parentPos[r.Ticket.ParentID]
			if ok {
				childrenOf[pidx] = append(childrenOf[pidx], r)
			} else {
				// Parent not in this batch, treat as root
				ordered = append(ordered, r)
			}
		}
	}

	// Insert children after their parents (iterate in reverse to preserve order)
	for i := len(ordered) - 1; i >= 0; i-- {
		r := ordered[i]
		if r.Ticket == nil {
			continue
		}
		// Find the original position of this parent to look up children
		for pidx, children := range childrenOf {
			if pidx < 0 || pidx >= len(rows) {
				continue
			}
			if rows[pidx].Ticket != nil && rows[pidx].Ticket.ID == r.Ticket.ID {
				// Insert children directly after this parent
				sort.Slice(children, func(a, b int) bool {
					return children[a].Ticket.CreatedAt.Before(children[b].Ticket.CreatedAt)
				})
				insertAt := i + 1
				ordered = append(ordered[:insertAt], append(children, ordered[insertAt:]...)...)
				delete(childrenOf, pidx)
				break
			}
		}
	}

	return ordered
}
