package messages

import (
	"testing"
	"time"

	"github.com/andyrewlee/amux/internal/tickets"
)

func TestTicketsLoadedMsg(t *testing.T) {
	msg := TicketsLoadedMsg{
		Tickets: []tickets.Ticket{
			{
				ID:          "bmx-123",
				Title:       "Test ticket",
				Description: "Test description",
				Status:      "open",
				Priority:    2,
				IssueType:   "task",
				Assignee:    "user",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		},
	}

	if len(msg.Tickets) != 1 {
		t.Fatalf("expected 1 ticket, got %d", len(msg.Tickets))
	}
	if msg.Tickets[0].ID != "bmx-123" {
		t.Fatalf("expected ticket ID bmx-123, got %s", msg.Tickets[0].ID)
	}
}

func TestTicketSelectedMsg(t *testing.T) {
	ticket := &tickets.Ticket{
		ID:    "bmx-456",
		Title: "Selected ticket",
	}
	msg := TicketSelectedMsg{Ticket: ticket}

	if msg.Ticket == nil {
		t.Fatal("expected non-nil ticket")
	}
	if msg.Ticket.ID != "bmx-456" {
		t.Fatalf("expected ticket ID bmx-456, got %s", msg.Ticket.ID)
	}
}

func TestTicketPreviewMsg(t *testing.T) {
	ticket := &tickets.Ticket{
		ID:    "bmx-preview",
		Title: "Preview ticket",
	}
	msg := TicketPreviewMsg{Ticket: ticket}

	if msg.Ticket == nil {
		t.Fatal("expected non-nil ticket")
	}
	if msg.Ticket.ID != "bmx-preview" {
		t.Fatalf("expected ticket ID bmx-preview, got %s", msg.Ticket.ID)
	}

	// Nil ticket means cursor moved away
	clearMsg := TicketPreviewMsg{Ticket: nil}
	if clearMsg.Ticket != nil {
		t.Fatal("expected nil ticket for clear message")
	}
}

func TestTicketRefreshMsg(t *testing.T) {
	msg := TicketRefreshMsg{}
	_ = msg
}

func TestDiscoveryLoadedMsg(t *testing.T) {
	msg := DiscoveryLoadedMsg{}
	_ = msg
}

func TestModelSelectedMsg(t *testing.T) {
	msg := ModelSelectedMsg{Model: tickets.NewModelContext("anthropic/claude/claude-sonnet-4")}

	if msg.Model.Provider() != "anthropic" {
		t.Fatalf("expected provider anthropic, got %s", msg.Model.Provider())
	}
	if msg.Model.Org() != "claude" {
		t.Fatalf("expected org claude, got %s", msg.Model.Org())
	}
	if msg.Model.Name() != "claude-sonnet-4" {
		t.Fatalf("expected name claude-sonnet-4, got %s", msg.Model.Name())
	}
}

func TestAgentModeSelectedMsg(t *testing.T) {
	msg := AgentModeSelectedMsg{AgentMode: "auto-approve"}

	if msg.AgentMode != "auto-approve" {
		t.Fatalf("expected agent mode auto-approve, got %s", msg.AgentMode)
	}
}

func TestLaunchReadyMsg(t *testing.T) {
	launchSpec := &tickets.LaunchSpec{
		Selection: tickets.Selection{
			Ticket: tickets.Ticket{
				ID:    "bmx-789",
				Title: "Launch ticket",
			},
			Assistant: "opencode",
			Model:     "anthropic/claude/claude-sonnet-4",
			Agent:     "coder",
		},
		RenderedCommand: "opencode --agent coder",
		RenderedPrompt:  "Work on bmx-789",
		LauncherID:      "bmx-789",
		WorkDir:         "/tmp/work",
	}

	msg := LaunchReadyMsg{LaunchSpec: launchSpec}

	if msg.LaunchSpec == nil {
		t.Fatal("expected non-nil launch spec")
	}
	if msg.LaunchSpec.LauncherID != "bmx-789" {
		t.Fatalf("expected launcher ID bmx-789, got %s", msg.LaunchSpec.LauncherID)
	}
	if msg.LaunchSpec.Selection.Ticket.ID != "bmx-789" {
		t.Fatalf("expected ticket ID bmx-789, got %s", msg.LaunchSpec.Selection.Ticket.ID)
	}
}
