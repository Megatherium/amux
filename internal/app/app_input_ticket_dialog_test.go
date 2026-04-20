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

	cmd := h.app.handleShowSelectTicketDialog()
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

	cmd := h.app.handleShowSelectTicketDialog()
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
	h.app.handleTicketsForPickerLoaded(ticketsForPickerLoaded{tickets: ts})

	if h.app.dialog == nil {
		t.Fatal("expected dialog to be created")
	}
	if !h.app.dialog.Visible() {
		t.Fatal("expected dialog to be visible")
	}
	view := h.app.dialog.View()
	if !strings.Contains(view, "Select Ticket") {
		t.Fatalf("expected dialog view to contain 'Select Ticket', got:\n%s", view)
	}
	if len(h.app.pendingTickets) != 2 {
		t.Fatalf("expected 2 pending tickets, got %d", len(h.app.pendingTickets))
	}
}

func TestTicketPickerResult_StoresTicketAndChains(t *testing.T) {
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
	h.app.pendingTickets = []tickets.Ticket{
		{ID: "bmx-1", Title: "Fix bug", Status: "open"},
	}

	cmd := h.app.handleDialogResult(common.DialogResult{
		ID:        "ticket-picker",
		Confirmed: true,
		Value:     "bmx-1 Fix bug",
		Index:     0,
	})
	if cmd == nil {
		t.Fatal("expected cmd chaining to TicketSelectedMsg")
	}
	msg := cmd()
	sel, ok := msg.(messages.TicketSelectedMsg)
	if !ok {
		t.Fatalf("expected TicketSelectedMsg, got %T", msg)
	}
	if sel.Ticket == nil || sel.Ticket.ID != "bmx-1" {
		t.Fatalf("expected ticket bmx-1, got %v", sel.Ticket)
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
	h.app.pendingTickets = []tickets.Ticket{
		{ID: "bmx-1", Title: "Fix bug", Status: "open"},
	}

	cmd := h.app.handleDialogResult(common.DialogResult{
		ID:        "ticket-picker",
		Confirmed: true,
		Value:     "no-ticket",
		Index:     1,
	})
	if cmd != nil {
		t.Fatal("expected nil cmd for out of bounds ticket option")
	}
	if h.app.selectedTicket != nil {
		t.Fatalf("expected nil selectedTicket for no-ticket, got %v", h.app.selectedTicket)
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
	h.app.pendingTickets = []tickets.Ticket{{ID: "bmx-1"}}
	h.app.selectedTicket = nil

	_ = h.app.handleDialogResult(common.DialogResult{
		ID:        "ticket-picker",
		Confirmed: false,
	})
	if h.app.selectedTicket != nil {
		t.Fatal("expected nil selectedTicket on cancel")
	}
	if h.app.pendingTickets != nil {
		t.Fatal("expected nil pendingTickets on cancel")
	}
}
