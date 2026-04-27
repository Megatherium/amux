package app

import (
	"strings"
	"testing"
	"time"

	"github.com/andyrewlee/amux/internal/config"
	"github.com/andyrewlee/amux/internal/tickets"
	"github.com/andyrewlee/amux/internal/ui/common"
	"github.com/andyrewlee/amux/internal/ui/layout"
)

func TestRenderTicketPreview(t *testing.T) {
	app := testAppForTicketPreview()
	app.previewTicket = &tickets.Ticket{
		ID:          "bmx-preview",
		Title:       "Preview ticket test",
		Status:      "open",
		Priority:    2,
		IssueType:   "bug",
		Assignee:    "bob",
		Description: "This is a test description for the preview.",
		CreatedAt:   time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 1, 14, 0, 0, 0, time.UTC),
	}

	content := app.renderTicketPreview()
	if !strings.Contains(content, "bmx-preview") {
		t.Fatalf("expected ticket ID in preview: %q", content)
	}
	if !strings.Contains(content, "Preview ticket test") {
		t.Fatalf("expected title in preview: %q", content)
	}
	if !strings.Contains(content, "open") {
		t.Fatalf("expected status in preview: %q", content)
	}
	if !strings.Contains(content, "P1") {
		t.Fatalf("expected priority in preview: %q", content)
	}
	if !strings.Contains(content, "bug") {
		t.Fatalf("expected issue type in preview: %q", content)
	}
	if !strings.Contains(content, "bob") {
		t.Fatalf("expected assignee in preview: %q", content)
	}
	if !strings.Contains(content, "test description") {
		t.Fatalf("expected description in preview: %q", content)
	}
	if !strings.Contains(content, "Enter") {
		t.Fatalf("expected action hint in preview: %q", content)
	}
}

func TestRenderTicketPreviewNil(t *testing.T) {
	app := testAppForTicketPreview()
	app.previewTicket = nil
	content := app.renderTicketPreview()
	if content != "" {
		t.Fatalf("expected empty string for nil ticket, got: %q", content)
	}
}

func TestRenderTicketPreviewClosed(t *testing.T) {
	app := testAppForTicketPreview()
	app.previewTicket = &tickets.Ticket{
		ID:     "bmx-closed",
		Title:  "Closed ticket",
		Status: "closed",
	}
	content := app.renderTicketPreview()
	if !strings.Contains(content, "closed") {
		t.Fatalf("expected closed status: %q", content)
	}
}

func TestRenderTicketPreviewNoDescription(t *testing.T) {
	app := testAppForTicketPreview()
	app.previewTicket = &tickets.Ticket{
		ID:     "bmx-nodesc",
		Title:  "No description",
		Status: "open",
	}
	content := app.renderTicketPreview()
	if !strings.Contains(content, "bmx-nodesc") {
		t.Fatalf("expected ticket ID: %q", content)
	}
}

func TestPriorityLabel(t *testing.T) {
	tests := []struct {
		priority int
		want     string
	}{
		{0, "P?"},
		{1, "P0 critical"},
		{2, "P1 high"},
		{3, "P2 medium"},
		{4, "P3 low"},
		{5, "P4"},
		{10, "P9"},
	}
	for _, tt := range tests {
		got := tickets.PriorityLabel(tt.priority)
		if got != tt.want {
			t.Errorf("PriorityLabel(%d) = %q, want %q", tt.priority, got, tt.want)
		}
	}
}

func TestWordWrap(t *testing.T) {
	tests := []struct {
		text      string
		width     int
		wantWraps int
	}{
		{"short", 80, 0},
		{"a b c d e f g h i j k l m n o p q r s t u v w x y z", 10, 2},
		{"line1\nline2\nline3", 80, 0},
	}
	for _, tt := range tests {
		result := wordWrap(tt.text, tt.width)
		lines := strings.Split(result, "\n")
		gotWraps := len(lines) - 1
		if tt.wantWraps > 0 && gotWraps < tt.wantWraps {
			t.Errorf("wordWrap(%q, %d) = %q, expected at least %d wraps, got %d", tt.text, tt.width, result, tt.wantWraps, gotWraps)
		}
	}
}

// testAppForTicketPreview creates a minimal App for testing ticket preview rendering.
func testAppForTicketPreview() *App {
	lm := layout.NewManager()
	lm.Resize(160, 40)

	return &App{
		config: &config.Config{
			UI: config.UISettings{},
		},
		ui:     &UICompositor{layout: lm},
		styles: common.DefaultStyles(),
	}
}
