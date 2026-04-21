package dashboard

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/tickets"
)

func TestTicketsHeaderAppearsWhenTicketsExist(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	var headerCount int
	for _, row := range m.rows {
		if row.Type == RowTicketsHeader {
			headerCount++
		}
	}
	if headerCount != 1 {
		t.Fatalf("expected 1 RowTicketsHeader, got %d", headerCount)
	}
}

func TestTicketsHeaderAbsentWhenNoTickets(t *testing.T) {
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{makeProject()})

	for _, row := range m.rows {
		if row.Type == RowTicketsHeader {
			t.Fatal("RowTicketsHeader should not appear when no tickets cached")
		}
	}
}

func TestTicketsHeaderPositionedBetweenWorkspacesAndCreate(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	var lastWSIdx, headerIdx, firstTicketIdx, createIdx int
	for i, row := range m.rows {
		switch row.Type {
		case RowWorkspace:
			lastWSIdx = i
		case RowTicketsHeader:
			headerIdx = i
		case RowTicket:
			if firstTicketIdx == 0 {
				firstTicketIdx = i
			}
		case RowCreate:
			createIdx = i
		}
	}

	if headerIdx <= lastWSIdx {
		t.Fatalf("RowTicketsHeader should come after workspace rows: ws=%d header=%d", lastWSIdx, headerIdx)
	}
	if firstTicketIdx <= headerIdx {
		t.Fatalf("RowTicket rows should come after RowTicketsHeader: header=%d ticket=%d", headerIdx, firstTicketIdx)
	}
	if createIdx <= firstTicketIdx+2 {
		t.Fatalf("RowCreate should come after ticket rows: create=%d lastTicket=%d", createIdx, firstTicketIdx+2)
	}
}

func TestTicketsHeaderIsSelectable(t *testing.T) {
	if !isSelectable(RowTicketsHeader) {
		t.Fatal("RowTicketsHeader should be selectable")
	}
}

func TestCollapseHidesTicketRows(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	// Verify tickets are visible initially
	initialTicketCount := 0
	for _, row := range m.rows {
		if row.Type == RowTicket {
			initialTicketCount++
		}
	}
	if initialTicketCount != 3 {
		t.Fatalf("expected 3 ticket rows initially, got %d", initialTicketCount)
	}

	// Collapse
	m.ticketsCollapsed[p.Path] = true
	m.rebuildRows()

	// Tickets should be hidden but header should remain
	ticketCount := 0
	headerCount := 0
	for _, row := range m.rows {
		switch row.Type {
		case RowTicket:
			ticketCount++
		case RowTicketsHeader:
			headerCount++
		}
	}
	if ticketCount != 0 {
		t.Fatalf("expected 0 ticket rows when collapsed, got %d", ticketCount)
	}
	if headerCount != 1 {
		t.Fatalf("expected 1 header row when collapsed, got %d", headerCount)
	}
}

func TestExpandShowsTicketRows(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	// Collapse then expand
	m.ticketsCollapsed[p.Path] = true
	m.rebuildRows()
	m.ticketsCollapsed[p.Path] = false
	m.rebuildRows()

	ticketCount := 0
	for _, row := range m.rows {
		if row.Type == RowTicket {
			ticketCount++
		}
	}
	if ticketCount != 3 {
		t.Fatalf("expected 3 ticket rows after expand, got %d", ticketCount)
	}
}

func TestHandleSpaceTogglesCollapse(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	// Find header row
	var headerIdx int
	for i, row := range m.rows {
		if row.Type == RowTicketsHeader {
			headerIdx = i
			break
		}
	}

	// Cursor on header, press space to collapse
	m.cursor = headerIdx
	m.handleSpace()

	if !m.ticketsCollapsed[p.Path] {
		t.Fatal("expected tickets to be collapsed after space")
	}

	// Verify ticket rows are hidden
	for _, row := range m.rows {
		if row.Type == RowTicket {
			t.Fatal("ticket rows should be hidden when collapsed")
		}
	}

	// Press space again to expand
	m.handleSpace()

	if m.ticketsCollapsed[p.Path] {
		t.Fatal("expected tickets to be expanded after second space")
	}

	// Verify ticket rows are visible again
	ticketCount := 0
	for _, row := range m.rows {
		if row.Type == RowTicket {
			ticketCount++
		}
	}
	if ticketCount != 3 {
		t.Fatalf("expected 3 ticket rows after expand, got %d", ticketCount)
	}
}

func TestHandleSpaceOnNonHeaderDoesNothing(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	// Cursor on Home row (not header)
	m.cursor = 0
	cmd := m.handleSpace()
	if cmd != nil {
		t.Fatal("handleSpace on non-header row should return nil")
	}
}

func TestHandleSpaceClampsCursorAfterCollapse(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	// Find header row
	var headerIdx int
	for i, row := range m.rows {
		if row.Type == RowTicketsHeader {
			headerIdx = i
			break
		}
	}

	// Cursor on header, collapse
	m.cursor = headerIdx
	m.handleSpace()

	// Cursor should still be valid and on the header
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		t.Fatalf("cursor out of bounds: %d (rows=%d)", m.cursor, len(m.rows))
	}
	if m.rows[m.cursor].Type != RowTicketsHeader {
		t.Fatalf("cursor should be on RowTicketsHeader after toggle, got %v", m.rows[m.cursor].Type)
	}
}

func TestPerProjectCollapseIsIndependent(t *testing.T) {
	p1, _ := makeProjectWithTickets()
	p2 := data.Project{
		Name: "other",
		Path: "/other",
		Workspaces: []data.Workspace{
			{Name: "other", Branch: "main", Repo: "/other", Root: "/other"},
		},
	}
	ts2 := []tickets.Ticket{
		{ID: "bmx-100", Title: "Other ticket", Status: "open"},
	}

	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p1, p2})
	m.SetTickets(p1.Path, []tickets.Ticket{
		{ID: "bmx-001", Title: "First", Status: "open"},
	})
	m.SetTickets(p2.Path, ts2)

	// Collapse project 1 only
	m.ticketsCollapsed[p1.Path] = true
	m.rebuildRows()

	// Project 1 tickets hidden, project 2 tickets visible
	p1Tickets := 0
	p2Tickets := 0
	for _, row := range m.rows {
		if row.Type == RowTicket {
			switch row.Project.Path {
			case p1.Path:
				p1Tickets++
			case p2.Path:
				p2Tickets++
			}
		}
	}
	if p1Tickets != 0 {
		t.Fatalf("project 1 tickets should be hidden, got %d", p1Tickets)
	}
	if p2Tickets != 1 {
		t.Fatalf("project 2 tickets should be visible, got %d", p2Tickets)
	}
}

func TestRenderTicketsHeaderExpanded(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	var headerIdx int
	for i, row := range m.rows {
		if row.Type == RowTicketsHeader {
			headerIdx = i
			break
		}
	}

	rendered := m.renderRow(m.rows[headerIdx], false)
	if !strings.Contains(rendered, "[Tickets]") {
		t.Fatalf("expanded header should contain [Tickets]: %q", rendered)
	}
	// Expanded uses DirOpen (▼)
	if !strings.Contains(rendered, "▼") {
		t.Fatalf("expanded header should use ▼ indicator: %q", rendered)
	}
	// Expanded should NOT show count
	if strings.Contains(rendered, "(3)") {
		t.Fatalf("expanded header should not show count: %q", rendered)
	}
}

func TestRenderTicketsHeaderCollapsed(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	m.ticketsCollapsed[p.Path] = true
	m.rebuildRows()

	var headerIdx int
	for i, row := range m.rows {
		if row.Type == RowTicketsHeader {
			headerIdx = i
			break
		}
	}

	rendered := m.renderRow(m.rows[headerIdx], false)
	if !strings.Contains(rendered, "[Tickets]") {
		t.Fatalf("collapsed header should contain [Tickets]: %q", rendered)
	}
	// Collapsed uses DirClosed (▶)
	if !strings.Contains(rendered, "▶") {
		t.Fatalf("collapsed header should use ▶ indicator: %q", rendered)
	}
	// Collapsed should show count
	if !strings.Contains(rendered, "(3)") {
		t.Fatalf("collapsed header should show count (3): %q", rendered)
	}
}

func TestRenderTicketsHeaderSelected(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	var headerIdx int
	for i, row := range m.rows {
		if row.Type == RowTicketsHeader {
			headerIdx = i
			break
		}
	}

	rendered := m.renderRow(m.rows[headerIdx], true)
	if !strings.Contains(rendered, "[Tickets]") {
		t.Fatalf("selected header should still contain [Tickets]: %q", rendered)
	}
}

func TestActivateTicketsHeaderReturnsNil(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	var headerIdx int
	for i, row := range m.rows {
		if row.Type == RowTicketsHeader {
			headerIdx = i
			break
		}
	}
	m.cursor = headerIdx
	cmd := m.activateCurrentRow()
	if cmd != nil {
		t.Fatal("activateCurrentRow on RowTicketsHeader should return nil")
	}
}

func TestHandleEnterOnTicketsHeaderReturnsNil(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	var headerIdx int
	for i, row := range m.rows {
		if row.Type == RowTicketsHeader {
			headerIdx = i
			break
		}
	}
	m.cursor = headerIdx
	cmd := m.handleEnter()
	if cmd != nil {
		t.Fatal("handleEnter on RowTicketsHeader should return nil")
	}
}

func TestSpaceKeyBindingTogglesHeader(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	var headerIdx int
	for i, row := range m.rows {
		if row.Type == RowTicketsHeader {
			headerIdx = i
			break
		}
	}
	m.cursor = headerIdx

	// Send space key via Update
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	_ = cmd

	if !m.ticketsCollapsed[p.Path] {
		t.Fatal("space key should collapse tickets")
	}
}

func TestClickOnTicketsHeaderToggles(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.showKeymapHints = false
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)
	_ = m.View()

	// Find header row index
	var headerIdx int
	for i, row := range m.rows {
		if row.Type == RowTicketsHeader {
			headerIdx = i
			break
		}
	}

	// Click on the header row (screenY = headerIdx + 1 for border)
	clickMsg := tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      5,
		Y:      headerIdx + 1,
	}
	_, cmd := m.Update(clickMsg)
	_ = cmd

	if !m.ticketsCollapsed[p.Path] {
		t.Fatal("clicking RowTicketsHeader should collapse tickets")
	}
}

func TestTicketCountHelper(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	count := m.ticketCount(p.Path)
	if count != 3 {
		t.Fatalf("expected ticket count 3, got %d", count)
	}

	// Non-existent project
	count = m.ticketCount("/nonexistent")
	if count != 0 {
		t.Fatalf("expected ticket count 0 for unknown project, got %d", count)
	}
}
