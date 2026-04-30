package app

import (
	"strings"
	"testing"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/tickets"
	"github.com/andyrewlee/amux/internal/ui/common"
)

func TestHandleShowSelectTicketDialog_NoTicketService_FallbackToAssistant(t *testing.T) {
	h, err := NewHarness(HarnessOptions{
		Mode:   HarnessCenter,
		Width:  120,
		Height: 40,
	})
	if err != nil {
		t.Fatalf("NewHarness: %v", err)
	}
	h.app.activeWorkspace = &data.Workspace{Name: "ws", Repo: "/r", Root: "/r/ws"}
	h.app.activeProject = &data.Project{Name: "p", Path: "/r"}
	h.app.ticketServices = nil

	cmd := h.app.ui.ShowSelectTicketDialog(h.app.activeWorkspace, h.app.activeProject, nil)
	if cmd == nil {
		t.Fatal("expected cmd when no ticket service")
	}
	msg := cmd()
	if _, ok := msg.(messages.ShowSelectAssistantDialog); !ok {
		t.Fatalf("expected ShowSelectAssistantDialog fallback, got %T", msg)
	}
}

func TestHandleShowSelectTicketDialog_NoActiveWorkspace(t *testing.T) {
	h, err := NewHarness(HarnessOptions{
		Mode:   HarnessCenter,
		Width:  120,
		Height: 40,
	})
	if err != nil {
		t.Fatalf("NewHarness: %v", err)
	}
	h.app.activeWorkspace = nil
	h.app.activeProject = nil

	cmd := h.app.ui.ShowSelectTicketDialog(h.app.activeWorkspace, h.app.activeProject, nil)
	if cmd != nil {
		t.Fatal("expected nil cmd when no active workspace")
	}
}

func TestHandleTicketsForPickerLoaded(t *testing.T) {
	h, err := NewHarness(HarnessOptions{
		Mode:   HarnessCenter,
		Width:  120,
		Height: 40,
	})
	if err != nil {
		t.Fatalf("NewHarness: %v", err)
	}

	ts := []tickets.Ticket{
		{ID: "bmx-1", Title: "Fix bug", Status: "open", IssueType: "bug", Priority: 1},
		{ID: "bmx-2", Title: "Add feature", Status: "in_progress", IssueType: "task", Priority: 2},
	}
	h.app.ui.HandleTicketsForPickerLoaded(ticketsForPickerLoaded{tickets: ts})

	if h.app.ui.dialog == nil {
		t.Fatal("expected dialog to be created")
	}
	if !h.app.ui.dialog.Visible() {
		t.Fatal("expected dialog to be visible")
	}
	view := h.app.ui.dialog.View()
	if !strings.Contains(view, "Select Ticket") {
		t.Fatalf("expected dialog view to contain 'Select Ticket', got:\n%s", view)
	}
	if len(h.app.ui.pendingTickets) != 2 {
		t.Fatalf("expected 2 pending tickets, got %d", len(h.app.ui.pendingTickets))
	}
}

func TestTicketPickerResult_StartsDraftFlow(t *testing.T) {
	h, err := NewHarness(HarnessOptions{
		Mode:   HarnessCenter,
		Width:  120,
		Height: 40,
	})
	if err != nil {
		t.Fatalf("NewHarness: %v", err)
	}
	h.app.activeWorkspace = &data.Workspace{Name: "ws", Repo: "/r", Root: "/r/ws"}
	h.app.activeProject = &data.Project{Name: "p", Path: "/r"}
	h.app.ui.pendingTickets = []tickets.Ticket{
		{ID: "bmx-1", Title: "Fix bug", Status: "open"},
	}

	cmd := h.app.handleDialogResult(common.DialogResult{
		ID:        "ticket-picker",
		Confirmed: true,
		Value:     "bmx-1 Fix bug",
		Index:     0,
	})
	if cmd == nil {
		t.Fatal("expected cmd emitting TicketSelectedMsg")
	}
	msg := cmd()
	tsMsg, ok := msg.(messages.TicketSelectedMsg)
	if !ok {
		t.Fatalf("expected TicketSelectedMsg, got %T", msg)
	}
	if tsMsg.Ticket == nil || tsMsg.Ticket.ID != "bmx-1" {
		t.Fatalf("expected ticket bmx-1, got %v", tsMsg.Ticket)
	}
	if tsMsg.Project == nil || tsMsg.Project.Name != "p" {
		t.Fatalf("expected project p, got %v", tsMsg.Project)
	}
	if h.app.ui.pendingTickets != nil {
		t.Fatal("expected pendingTickets to be cleared")
	}
}

func TestTicketPickerResult_NoTicketOption(t *testing.T) {
	h, err := NewHarness(HarnessOptions{
		Mode:   HarnessCenter,
		Width:  120,
		Height: 40,
	})
	if err != nil {
		t.Fatalf("NewHarness: %v", err)
	}
	h.app.activeWorkspace = &data.Workspace{Name: "ws", Repo: "/r", Root: "/r/ws"}
	h.app.ui.pendingTickets = []tickets.Ticket{
		{ID: "bmx-1", Title: "Fix bug", Status: "open"},
	}

	cmd := h.app.handleDialogResult(common.DialogResult{
		ID:        "ticket-picker",
		Confirmed: true,
		Value:     "no-ticket",
		Index:     1,
	})
	if cmd == nil {
		t.Fatal("expected cmd falling back to assistant picker")
	}
	msg := cmd()
	if _, ok := msg.(messages.ShowSelectAssistantDialog); !ok {
		t.Fatalf("expected ShowSelectAssistantDialog for no-ticket, got %T", msg)
	}
}

func TestTicketPickerResult_Cancel(t *testing.T) {
	h, err := NewHarness(HarnessOptions{
		Mode:   HarnessCenter,
		Width:  120,
		Height: 40,
	})
	if err != nil {
		t.Fatalf("NewHarness: %v", err)
	}
	h.app.ui.pendingTickets = []tickets.Ticket{{ID: "bmx-1"}}

	_ = h.app.handleDialogResult(common.DialogResult{
		ID:        "ticket-picker",
		Confirmed: false,
	})
	if h.app.ui.pendingTickets != nil {
		t.Fatal("expected nil pendingTickets on cancel")
	}
}
