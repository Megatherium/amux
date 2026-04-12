package dashboard

import (
	"strings"
	"testing"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/tickets"
)

func makeProjectWithTickets() (data.Project, []tickets.Ticket) {
	p := data.Project{
		Name: "repo",
		Path: "/repo",
		Workspaces: []data.Workspace{
			{Name: "repo", Branch: "main", Repo: "/repo", Root: "/repo"},
			{Name: "feature", Branch: "feature", Repo: "/repo", Root: "/repo/.amux/workspaces/feature"},
		},
	}
	ts := []tickets.Ticket{
		{ID: "bmx-001", Title: "Fix login bug", Status: "open"},
		{ID: "bmx-002", Title: "Add feature X", Status: "in_progress"},
		{ID: "bmx-003", Title: "Update docs", Status: "closed"},
	}
	return p, ts
}

func TestRebuildRowsWithTickets(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	var ticketRows int
	for _, row := range m.rows {
		if row.Type == RowTicket {
			ticketRows++
			if row.Ticket == nil {
				t.Fatalf("RowTicket should have non-nil Ticket")
			}
		}
	}
	if ticketRows != 3 {
		t.Fatalf("expected 3 ticket rows, got %d", ticketRows)
	}

	// Ticket rows should appear between workspace rows and RowCreate
	var lastWSIdx, firstTicketIdx, createIdx int
	for i, row := range m.rows {
		switch row.Type {
		case RowWorkspace:
			lastWSIdx = i
		case RowTicket:
			if firstTicketIdx == 0 {
				firstTicketIdx = i
			}
		case RowCreate:
			createIdx = i
		}
	}
	if firstTicketIdx <= lastWSIdx {
		t.Fatalf("ticket rows should come after workspace rows: ws=%d ticket=%d", lastWSIdx, firstTicketIdx)
	}
	if createIdx <= firstTicketIdx+2 {
		t.Fatalf("RowCreate should come after ticket rows: create=%d lastTicket=%d", createIdx, firstTicketIdx+2)
	}
}

func TestRebuildRowsWithoutTickets(t *testing.T) {
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{makeProject()})

	for _, row := range m.rows {
		if row.Type == RowTicket {
			t.Fatal("should not have ticket rows when no tickets cached")
		}
	}
}

func TestSetTicketsTriggersRebuild(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})

	before := len(m.rows)
	m.SetTickets(p.Path, ts)
	after := len(m.rows)

	if after <= before {
		t.Fatalf("SetTickets should increase row count: before=%d after=%d", before, after)
	}
}

func TestTicketNavigation(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	// Find first ticket row
	var ticketIdx int
	for i, row := range m.rows {
		if row.Type == RowTicket {
			ticketIdx = i
			break
		}
	}

	m.cursor = ticketIdx - 1
	m.moveCursor(1)
	if m.cursor != ticketIdx {
		t.Fatalf("cursor should land on ticket row %d, got %d", ticketIdx, m.cursor)
	}

	// moveCursor should be able to move past ticket rows
	m.moveCursor(1)
	if m.rows[m.cursor].Type == RowSpacer {
		t.Fatal("cursor should not land on spacer")
	}
}

func TestTicketActivateReturnsNil(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	var ticketIdx int
	for i, row := range m.rows {
		if row.Type == RowTicket {
			ticketIdx = i
			break
		}
	}
	m.cursor = ticketIdx
	cmd := m.activateCurrentRow()
	if cmd != nil {
		t.Fatal("activateCurrentRow should return nil for ticket rows (auto-activate gated)")
	}
}

func TestTicketHandleEnterReturnsTicketSelectedMsg(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	var ticketIdx int
	for i, row := range m.rows {
		if row.Type == RowTicket {
			ticketIdx = i
			break
		}
	}
	m.cursor = ticketIdx
	cmd := m.handleEnter()
	if cmd == nil {
		t.Fatal("handleEnter should return a command for ticket rows")
	}
	msg := cmd()
	sel, ok := msg.(messages.TicketSelectedMsg)
	if !ok {
		t.Fatalf("expected TicketSelectedMsg, got %T", msg)
	}
	if sel.Ticket == nil || sel.Ticket.ID != "bmx-001" {
		t.Fatalf("expected ticket bmx-001, got %v", sel.Ticket)
	}
}

func TestRenderTicketRow(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	var ticketIdx int
	for i, row := range m.rows {
		if row.Type == RowTicket {
			ticketIdx = i
			break
		}
	}
	rendered := m.renderRow(m.rows[ticketIdx], false)
	if !strings.Contains(rendered, "bmx-001") {
		t.Fatalf("rendered ticket row should contain ticket ID: %q", rendered)
	}
	if !strings.Contains(rendered, "Fix login bug") {
		t.Fatalf("rendered ticket row should contain ticket title: %q", rendered)
	}
}

func TestRenderTicketRowSelected(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	var ticketIdx int
	for i, row := range m.rows {
		if row.Type == RowTicket {
			ticketIdx = i
			break
		}
	}
	rendered := m.renderRow(m.rows[ticketIdx], true)
	if !strings.Contains(rendered, "bmx-001") {
		t.Fatalf("selected ticket row should still contain ticket ID: %q", rendered)
	}
}

func TestRenderOpenTicketUsesIdleIcon(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	// Find the open ticket row (bmx-001)
	var openIdx int
	for i, row := range m.rows {
		if row.Type == RowTicket && row.Ticket != nil && row.Ticket.Status == "open" {
			openIdx = i
			break
		}
	}
	rendered := m.renderRow(m.rows[openIdx], false)
	if !strings.Contains(rendered, "bmx-001") {
		t.Fatalf("open ticket row should contain ID: %q", rendered)
	}
	if !strings.Contains(rendered, "○") {
		t.Fatalf("open ticket should use Idle icon (○): %q", rendered)
	}
}

func TestRenderClosedTicketMuted(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	// Find the closed ticket row (bmx-003)
	var closedIdx int
	for i, row := range m.rows {
		if row.Type == RowTicket && row.Ticket != nil && row.Ticket.Status == "closed" {
			closedIdx = i
			break
		}
	}
	rendered := m.renderRow(m.rows[closedIdx], false)
	if !strings.Contains(rendered, "bmx-003") {
		t.Fatalf("closed ticket row should contain ID: %q", rendered)
	}
	if !strings.Contains(rendered, "✓") {
		t.Fatalf("closed ticket should use Clean icon (✓): %q", rendered)
	}
}

func TestRenderInProgressTicket(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	// Find the in_progress ticket row (bmx-002)
	var ipIdx int
	for i, row := range m.rows {
		if row.Type == RowTicket && row.Ticket != nil && row.Ticket.Status == "in_progress" {
			ipIdx = i
			break
		}
	}
	rendered := m.renderRow(m.rows[ipIdx], false)
	if !strings.Contains(rendered, "bmx-002") {
		t.Fatalf("in_progress ticket row should contain ID: %q", rendered)
	}
	if !strings.Contains(rendered, "◌") {
		t.Fatalf("in_progress ticket should use Pending icon (◌): %q", rendered)
	}
}

func TestRenderTicketTruncation(t *testing.T) {
	p := data.Project{
		Name: "repo",
		Path: "/repo",
		Workspaces: []data.Workspace{
			{Name: "repo", Branch: "main", Repo: "/repo", Root: "/repo"},
		},
	}
	longTitle := "This is a very long ticket title that should definitely be truncated when rendered in a narrow pane"
	ts := []tickets.Ticket{
		{ID: "bmx-999", Title: longTitle, Status: "open"},
	}
	m := New()
	m.SetSize(30, 20)
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)

	var ticketIdx int
	for i, row := range m.rows {
		if row.Type == RowTicket {
			ticketIdx = i
			break
		}
	}
	rendered := m.renderRow(m.rows[ticketIdx], false)
	if !strings.Contains(rendered, "…") {
		t.Fatalf("long ticket title should be truncated: %q", rendered)
	}
}

func TestTicketsLoadedMsgUpdatesDashboard(t *testing.T) {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.SetProjects([]data.Project{p})

	msg := messages.TicketsLoadedMsg{
		ProjectPath: p.Path,
		Tickets:     ts,
	}
	newM, cmd := m.Update(msg)
	if cmd != nil {
		t.Fatalf("expected no command from TicketsLoadedMsg, got %v", cmd)
	}

	var ticketRows int
	for _, row := range newM.rows {
		if row.Type == RowTicket {
			ticketRows++
		}
	}
	if ticketRows != 3 {
		t.Fatalf("expected 3 ticket rows after TicketsLoadedMsg, got %d", ticketRows)
	}
}

func TestTicketIsSelectable(t *testing.T) {
	if !isSelectable(RowTicket) {
		t.Fatal("RowTicket should be selectable")
	}
}
