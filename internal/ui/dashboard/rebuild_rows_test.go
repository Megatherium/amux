package dashboard

import (
	"testing"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/tickets"
)

// --- appendWorkspaceRows ---

func TestAppendWorkspaceRowsSkipsMainAndPrimary(t *testing.T) {
	m := New()
	project := makeProject()
	rows := m.appendWorkspaceRows(nil, &project)

	if len(rows) != 1 {
		t.Fatalf("expected 1 workspace row (skipping main), got %d", len(rows))
	}
	if rows[0].Type != RowWorkspace {
		t.Fatalf("expected RowWorkspace, got %v", rows[0].Type)
	}
	if rows[0].Workspace.Name != "feature" {
		t.Fatalf("expected 'feature' workspace, got %s", rows[0].Workspace.Name)
	}
}

func TestAppendWorkspaceRowsEmpty(t *testing.T) {
	m := New()
	project := data.Project{
		Name: "repo",
		Path: "/repo",
		Workspaces: []data.Workspace{
			{Name: "repo", Branch: "main", Repo: "/repo", Root: "/repo"},
		},
	}
	rows := m.appendWorkspaceRows(nil, &project)
	if len(rows) != 0 {
		t.Fatalf("expected 0 workspace rows when only main exists, got %d", len(rows))
	}
}

func TestAppendWorkspaceRowsSetsActivityID(t *testing.T) {
	m := New()
	project := makeProject()
	rows := m.appendWorkspaceRows(nil, &project)

	for _, row := range rows {
		if row.ActivityWorkspaceID == "" {
			t.Fatal("workspace row should have non-empty ActivityWorkspaceID")
		}
	}
}

// --- appendTicketRows ---

func TestAppendTicketRowsNoCache(t *testing.T) {
	m := New()
	project := makeProject()
	rows := m.appendTicketRows(nil, &project)
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows when no tickets cached, got %d", len(rows))
	}
}

func TestAppendTicketRowsEmptyCache(t *testing.T) {
	m := New()
	m.ticketCache = make(map[string][]tickets.Ticket)
	project := makeProject()
	m.ticketCache[project.Path] = []tickets.Ticket{}
	rows := m.appendTicketRows(nil, &project)
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows when ticket cache is empty, got %d", len(rows))
	}
}

func TestAppendTicketRowsExpanded(t *testing.T) {
	m := New()
	m.ticketCache = make(map[string][]tickets.Ticket)
	project := makeProject()
	ts := []tickets.Ticket{
		{ID: "bmx-001", Title: "Bug", Status: "open"},
		{ID: "bmx-002", Title: "Feature", Status: "in_progress"},
	}
	m.ticketCache[project.Path] = ts

	rows := m.appendTicketRows(nil, &project)

	// 1 header + 2 tickets = 3
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (header + 2 tickets), got %d", len(rows))
	}
	if rows[0].Type != RowTicketsHeader {
		t.Fatalf("first row should be RowTicketsHeader, got %v", rows[0].Type)
	}
	if rows[1].Type != RowTicket || rows[1].Ticket.ID != "bmx-001" {
		t.Fatalf("second row should be ticket bmx-001, got %v", rows[1])
	}
	if rows[2].Type != RowTicket || rows[2].Ticket.ID != "bmx-002" {
		t.Fatalf("third row should be ticket bmx-002, got %v", rows[2])
	}
}

func TestAppendTicketRowsCollapsed(t *testing.T) {
	m := New()
	m.ticketCache = make(map[string][]tickets.Ticket)
	project := makeProject()
	ts := []tickets.Ticket{
		{ID: "bmx-001", Title: "Bug", Status: "open"},
	}
	m.ticketCache[project.Path] = ts
	m.ticketsCollapsed[project.Path] = true

	rows := m.appendTicketRows(nil, &project)

	// Only header, no ticket rows
	if len(rows) != 1 {
		t.Fatalf("expected 1 row (header only), got %d", len(rows))
	}
	if rows[0].Type != RowTicketsHeader {
		t.Fatalf("expected RowTicketsHeader, got %v", rows[0].Type)
	}
}

func TestAppendTicketRowsPreservesExisting(t *testing.T) {
	m := New()
	m.ticketCache = make(map[string][]tickets.Ticket)
	project := makeProject()
	ts := []tickets.Ticket{{ID: "bmx-001", Title: "Bug", Status: "open"}}
	m.ticketCache[project.Path] = ts

	existing := []Row{{Type: RowHome}}
	rows := m.appendTicketRows(existing, &project)

	if len(rows) != len(existing)+2 {
		t.Fatalf("expected %d rows, got %d", len(existing)+2, len(rows))
	}
	if rows[0].Type != RowHome {
		t.Fatal("existing rows should be preserved")
	}
}

// --- appendProjectRows ---

func TestAppendProjectRowsStructure(t *testing.T) {
	m := New()
	project := makeProject()
	rows := m.appendProjectRows(nil, &project)

	// Expected: Project, Workspace (feature), Create = 3 rows
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (project + workspace + create), got %d", len(rows))
	}
	if rows[0].Type != RowProject {
		t.Fatalf("first row should be RowProject, got %v", rows[0].Type)
	}
	if rows[0].MainWorkspace == nil || rows[0].MainWorkspace.Branch != "main" {
		t.Fatal("project row should have MainWorkspace pointing to main branch")
	}
	if rows[1].Type != RowWorkspace {
		t.Fatalf("second row should be RowWorkspace, got %v", rows[1].Type)
	}
	if rows[2].Type != RowCreate {
		t.Fatalf("last row should be RowCreate, got %v", rows[2].Type)
	}
}

func TestAppendProjectRowsWithTickets(t *testing.T) {
	m := New()
	m.ticketCache = make(map[string][]tickets.Ticket)
	project := makeProject()
	ts := []tickets.Ticket{
		{ID: "bmx-001", Title: "Bug", Status: "open"},
	}
	m.ticketCache[project.Path] = ts

	rows := m.appendProjectRows(nil, &project)

	// Expected: Project, Workspace, TicketsHeader, Ticket, Create = 5 rows
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(rows))
	}
	if rows[2].Type != RowTicketsHeader {
		t.Fatalf("third row should be RowTicketsHeader, got %v", rows[2].Type)
	}
	if rows[3].Type != RowTicket {
		t.Fatalf("fourth row should be RowTicket, got %v", rows[3].Type)
	}
}

// --- clampCursor ---

func TestClampCursorBounds(t *testing.T) {
	m := New()
	m.rows = []Row{
		{Type: RowHome},
		{Type: RowProject},
		{Type: RowWorkspace},
	}

	t.Run("clamp from above", func(t *testing.T) {
		m.cursor = 100
		m.clampCursor()
		if m.cursor != 2 {
			t.Fatalf("expected cursor clamped to 2, got %d", m.cursor)
		}
	})

	t.Run("clamp from below", func(t *testing.T) {
		m.cursor = -5
		m.clampCursor()
		if m.cursor != 0 {
			t.Fatalf("expected cursor clamped to 0, got %d", m.cursor)
		}
	})
}

func TestClampCursorSkipsSpacer(t *testing.T) {
	m := New()
	m.rows = []Row{
		{Type: RowHome},
		{Type: RowSpacer},
		{Type: RowProject},
	}

	m.cursor = 1 // on spacer
	m.clampCursor()
	if m.cursor == 1 {
		t.Fatal("cursor should have moved off spacer")
	}
}

func TestClampCursorEmptyRows(t *testing.T) {
	m := New()
	m.rows = []Row{}
	m.cursor = 5
	m.clampCursor()
	// Should not panic; cursor should be 0 (len-1 clamped to 0)
	if m.cursor != 0 {
		t.Fatalf("expected cursor 0 for empty rows, got %d", m.cursor)
	}
}
