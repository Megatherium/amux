package sidebar

import (
	"testing"

	"github.com/andyrewlee/amux/internal/tickets"
)

func TestTabbedSidebarSetPreviewTicket(t *testing.T) {
	sb := NewTabbedSidebar()
	sb.SetSize(40, 20)

	// No ticket initially
	view := sb.View()
	if strings := countTicketTab(view); strings != 0 {
		t.Fatalf("expected no ticket tab initially, got %d", strings)
	}

	// Set a preview ticket
	sb.SetPreviewTicket(&tickets.Ticket{
		ID:     "bmx-preview",
		Title:  "Preview test",
		Status: "open",
	})
	view = sb.View()
	if count := countTicketTab(view); count != 1 {
		t.Fatalf("expected 1 ticket tab after SetPreviewTicket, got %d", count)
	}
}

func TestTabbedSidebarClearPreviewTicket(t *testing.T) {
	sb := NewTabbedSidebar()
	sb.SetSize(40, 20)

	sb.SetPreviewTicket(&tickets.Ticket{
		ID:     "bmx-clear",
		Title:  "Clear test",
		Status: "open",
	})
	sb.activeTab = TabTicket
	sb.SetPreviewTicket(nil)

	// Should switch away from ticket tab
	if sb.activeTab == TabTicket {
		t.Fatal("should switch away from TabTicket when preview is cleared")
	}

	view := sb.View()
	if count := countTicketTab(view); count != 0 {
		t.Fatalf("expected no ticket tab after clear, got %d", count)
	}
}

func TestTabbedSidebarTicketTabContentView(t *testing.T) {
	sb := NewTabbedSidebar()
	sb.SetSize(60, 20)

	ticket := &tickets.Ticket{
		ID:     "bmx-content",
		Title:  "Content test",
		Status: "in_progress",
	}
	sb.SetPreviewTicket(ticket)
	sb.activeTab = TabTicket

	content := sb.ContentView()
	if content == "" {
		t.Fatal("expected non-empty content for ticket tab")
	}
}

func TestTabbedSidebarNextTabWithTicket(t *testing.T) {
	sb := NewTabbedSidebar()
	sb.SetSize(40, 20)
	sb.Focus()

	// Without ticket: Changes -> Project -> Changes
	sb.activeTab = TabChanges
	sb.NextTab()
	if sb.activeTab != TabProject {
		t.Fatalf("expected TabProject, got %d", sb.activeTab)
	}
	sb.NextTab()
	if sb.activeTab != TabChanges {
		t.Fatalf("expected TabChanges (no ticket), got %d", sb.activeTab)
	}

	// With ticket: Changes -> Project -> Ticket -> Changes
	sb.SetPreviewTicket(&tickets.Ticket{
		ID: "bmx-nav", Title: "Nav", Status: "open",
	})
	sb.activeTab = TabChanges
	sb.NextTab()
	if sb.activeTab != TabProject {
		t.Fatalf("expected TabProject, got %d", sb.activeTab)
	}
	sb.NextTab()
	if sb.activeTab != TabTicket {
		t.Fatalf("expected TabTicket, got %d", sb.activeTab)
	}
	sb.NextTab()
	if sb.activeTab != TabChanges {
		t.Fatalf("expected TabChanges (wrap), got %d", sb.activeTab)
	}
}

func TestTabbedSidebarPrevTabWithTicket(t *testing.T) {
	sb := NewTabbedSidebar()
	sb.SetSize(40, 20)
	sb.Focus()

	sb.SetPreviewTicket(&tickets.Ticket{
		ID: "bmx-prev", Title: "Prev", Status: "open",
	})

	sb.activeTab = TabChanges
	sb.PrevTab()
	if sb.activeTab != TabTicket {
		t.Fatalf("expected TabTicket (prev from Changes), got %d", sb.activeTab)
	}
	sb.PrevTab()
	if sb.activeTab != TabProject {
		t.Fatalf("expected TabProject (prev from Ticket), got %d", sb.activeTab)
	}
}

func countTicketTab(view string) int {
	count := 0
	for i := 0; i < len(view)-6; i++ {
		if view[i:i+6] == "Ticket" {
			count++
		}
	}
	return count
}
