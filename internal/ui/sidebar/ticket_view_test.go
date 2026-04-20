package sidebar

import (
	"strings"
	"testing"
	"time"

	"github.com/andyrewlee/amux/internal/tickets"
)

func TestTicketViewNil(t *testing.T) {
	tv := NewTicketView()
	tv.SetSize(40, 20)
	view := tv.View()
	if !strings.Contains(view, "No ticket selected") {
		t.Fatalf("expected 'No ticket selected' for nil ticket, got: %q", view)
	}
}

func TestTicketViewRendersBasicInfo(t *testing.T) {
	tv := NewTicketView()
	tv.SetSize(60, 20)
	tv.SetTicket(&tickets.Ticket{
		ID:        "bmx-100",
		Title:     "Fix something",
		Status:    "open",
		Priority:  2,
		IssueType: "bug",
		Assignee:  "alice",
		CreatedAt: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 1, 14, 0, 0, 0, time.UTC),
	})
	view := tv.View()
	if !strings.Contains(view, "bmx-100") {
		t.Fatalf("expected ticket ID in view: %q", view)
	}
	if !strings.Contains(view, "Fix something") {
		t.Fatalf("expected ticket title in view: %q", view)
	}
	if !strings.Contains(view, "open") {
		t.Fatalf("expected status in view: %q", view)
	}
	if !strings.Contains(view, "P1") {
		t.Fatalf("expected priority in view: %q", view)
	}
	if !strings.Contains(view, "bug") {
		t.Fatalf("expected issue type in view: %q", view)
	}
	if !strings.Contains(view, "alice") {
		t.Fatalf("expected assignee in view: %q", view)
	}
}

func TestTicketViewClosedStatus(t *testing.T) {
	tv := NewTicketView()
	tv.SetSize(60, 20)
	tv.SetTicket(&tickets.Ticket{
		ID:     "bmx-200",
		Title:  "Done task",
		Status: "closed",
	})
	view := tv.View()
	if !strings.Contains(view, "closed") {
		t.Fatalf("expected closed status in view: %q", view)
	}
}

func TestTicketViewInProgressStatus(t *testing.T) {
	tv := NewTicketView()
	tv.SetSize(60, 20)
	tv.SetTicket(&tickets.Ticket{
		ID:     "bmx-300",
		Title:  "Active task",
		Status: "in_progress",
	})
	view := tv.View()
	if !strings.Contains(view, "in_progress") {
		t.Fatalf("expected in_progress status in view: %q", view)
	}
}

func TestTicketViewWithDescription(t *testing.T) {
	tv := NewTicketView()
	tv.SetSize(60, 20)
	tv.SetTicket(&tickets.Ticket{
		ID:          "bmx-400",
		Title:       "With desc",
		Status:      "open",
		Description: "This is a detailed description of the ticket.",
	})
	view := tv.View()
	if !strings.Contains(view, "detailed description") {
		t.Fatalf("expected description in view: %q", view)
	}
}

func TestTicketViewTruncatesLongDescription(t *testing.T) {
	tv := NewTicketView()
	tv.SetSize(40, 8) // Very small height
	longDesc := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7\nLine 8"
	tv.SetTicket(&tickets.Ticket{
		ID:          "bmx-500",
		Title:       "Long desc",
		Status:      "open",
		Description: longDesc,
	})
	view := tv.View()
	// Should not contain all lines
	if strings.Contains(view, "Line 8") {
		t.Fatalf("long description should be truncated in small view: %q", view)
	}
}

func TestTicketViewNoAssignee(t *testing.T) {
	tv := NewTicketView()
	tv.SetSize(60, 20)
	tv.SetTicket(&tickets.Ticket{
		ID:     "bmx-600",
		Title:  "No owner",
		Status: "open",
	})
	view := tv.View()
	if strings.Contains(view, "Owner:") {
		t.Fatalf("should not show assignee when empty: %q", view)
	}
}

func TestTicketViewNoType(t *testing.T) {
	tv := NewTicketView()
	tv.SetSize(60, 20)
	tv.SetTicket(&tickets.Ticket{
		ID:     "bmx-700",
		Title:  "No type",
		Status: "open",
	})
	view := tv.View()
	if strings.Contains(view, "Type:") {
		t.Fatalf("should not show type when empty: %q", view)
	}
}

func TestTruncateDescriptionWrapsLongLines(t *testing.T) {
	longLine := "This is a very long single line that should be wrapped to fit within the given width before truncation happens"
	result := truncateDescription(longLine, 20, 5)
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected long line to be wrapped into multiple lines, got: %q", result)
	}
	for _, line := range lines {
		// Each line should not exceed width significantly (allowing for the "..." suffix)
		runes := []rune(line)
		if len(runes) > 25 {
			t.Errorf("wrapped line too long (%d chars): %q", len(runes), line)
		}
	}
}

func TestWrapLineSimple(t *testing.T) {
	tests := []struct {
		line  string
		width int
		min   int // minimum expected number of lines
	}{
		{"short", 80, 1},
		{"a b c d e f g h i j k l m n o p", 10, 2},
		{"", 20, 1},
	}
	for _, tt := range tests {
		result := wrapLineSimple(tt.line, tt.width)
		if len(result) < tt.min {
			t.Errorf("wrapLineSimple(%q, %d) = %v, expected at least %d lines", tt.line, tt.width, result, tt.min)
		}
	}
}

func TestTicketViewSetTicketClears(t *testing.T) {
	tv := NewTicketView()
	tv.SetSize(60, 20)
	tv.SetTicket(&tickets.Ticket{
		ID:     "bmx-800",
		Title:  "Temporary",
		Status: "open",
	})
	view1 := tv.View()
	if !strings.Contains(view1, "bmx-800") {
		t.Fatalf("expected ticket ID in view: %q", view1)
	}

	tv.SetTicket(nil)
	view2 := tv.View()
	if !strings.Contains(view2, "No ticket selected") {
		t.Fatalf("expected cleared view: %q", view2)
	}
}

func TestTicketViewFocus(t *testing.T) {
	tv := NewTicketView()
	if tv.Focused() {
		t.Fatal("should start unfocused")
	}
	tv.Focus()
	if !tv.Focused() {
		t.Fatal("should be focused after Focus()")
	}
	tv.Blur()
	if tv.Focused() {
		t.Fatal("should be unfocused after Blur()")
	}
}

func TestPriorityLabelShort(t *testing.T) {
	tests := []struct {
		priority int
		want     string
	}{
		{0, "?"},
		{1, "P0"},
		{2, "P1"},
		{3, "P2"},
		{4, "P3"},
		{5, "P4"},
	}
	for _, tt := range tests {
		got := tickets.PriorityLabelShort(tt.priority)
		if got != tt.want {
			t.Errorf("PriorityLabelShort(%d) = %q, want %q", tt.priority, got, tt.want)
		}
	}
}
