package common

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewTicketPicker(t *testing.T) {
	items := []TicketPickerItem{
		{ID: "bmx-1", Title: "Fix bug", Status: "open", IssueType: "bug", Priority: 1},
		{ID: "bmx-2", Title: "Add feature", Status: "in_progress", IssueType: "task", Priority: 2},
	}
	d := NewTicketPicker(items)
	if d.id != "ticket-picker" {
		t.Fatalf("expected id ticket-picker, got %q", d.id)
	}
	if d.dtype != DialogSelect {
		t.Fatal("expected DialogSelect type")
	}
	if !d.filterEnabled {
		t.Fatal("expected filter enabled")
	}
	if len(d.options) != 3 {
		t.Fatalf("expected 3 options (2 tickets + no-ticket), got %d", len(d.options))
	}
	if d.options[0] != "bmx-1 Fix bug" {
		t.Fatalf("expected first option to be 'bmx-1 Fix bug', got %q", d.options[0])
	}
	if d.options[2] != "no-ticket" {
		t.Fatalf("expected last option to be 'no-ticket', got %q", d.options[2])
	}
	if len(d.filteredIndices) != 3 {
		t.Fatalf("expected 3 filtered indices, got %d", len(d.filteredIndices))
	}
}

func TestNewTicketPickerEmpty(t *testing.T) {
	d := NewTicketPicker(nil)
	if len(d.options) != 1 {
		t.Fatalf("expected 1 option (no-ticket only), got %d", len(d.options))
	}
	if d.options[0] != "no-ticket" {
		t.Fatalf("expected no-ticket option, got %q", d.options[0])
	}
}

func TestTicketPickerFilter(t *testing.T) {
	items := []TicketPickerItem{
		{ID: "bmx-1", Title: "Fix login bug", Status: "open", IssueType: "bug", Priority: 1},
		{ID: "bmx-2", Title: "Wire service", Status: "in_progress", IssueType: "task", Priority: 2},
	}
	d := NewTicketPicker(items)
	d.Show()

	d.filterInput.SetValue("login")
	d.applyFilter()
	if len(d.filteredIndices) != 1 {
		t.Fatalf("expected 1 filtered result for 'login', got %d", len(d.filteredIndices))
	}
	if d.filteredIndices[0] != 0 {
		t.Fatalf("expected filtered index 0, got %d", d.filteredIndices[0])
	}

	d.filterInput.SetValue("zzz")
	d.applyFilter()
	if len(d.filteredIndices) != 0 {
		t.Fatalf("expected 0 filtered results for 'zzz', got %d", len(d.filteredIndices))
	}
}

func TestTicketPickerSelectFirstTicket(t *testing.T) {
	items := []TicketPickerItem{
		{ID: "bmx-1", Title: "Fix bug", Status: "open", IssueType: "bug", Priority: 1},
	}
	d := NewTicketPicker(items)
	d.SetSize(80, 24)
	d.Show()

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command on enter")
	}
	result := cmd()
	dr, ok := result.(DialogResult)
	if !ok {
		t.Fatalf("expected DialogResult, got %T", result)
	}
	if !dr.Confirmed {
		t.Fatal("expected confirmed")
	}
	if dr.Index != 0 {
		t.Fatalf("expected index 0, got %d", dr.Index)
	}
	if dr.Value != "bmx-1 Fix bug" {
		t.Fatalf("expected value 'bmx-1 Fix bug', got %q", dr.Value)
	}
}

func TestTicketPickerSelectNoTicket(t *testing.T) {
	d := NewTicketPicker(nil)
	d.SetSize(80, 24)
	d.Show()

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command on enter")
	}
	result := cmd()
	dr, ok := result.(DialogResult)
	if !ok {
		t.Fatalf("expected DialogResult, got %T", result)
	}
	if !dr.Confirmed {
		t.Fatal("expected confirmed for no-ticket")
	}
	if dr.Value != "no-ticket" {
		t.Fatalf("expected value 'no-ticket', got %q", dr.Value)
	}
}

func TestTicketPickerCancel(t *testing.T) {
	d := NewTicketPicker([]TicketPickerItem{
		{ID: "bmx-1", Title: "Fix bug", Status: "open", IssueType: "bug", Priority: 1},
	})
	d.SetSize(80, 24)
	d.Show()

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected command on escape")
	}
	result := cmd()
	dr, ok := result.(DialogResult)
	if !ok {
		t.Fatalf("expected DialogResult, got %T", result)
	}
	if dr.Confirmed {
		t.Fatal("expected not confirmed on cancel")
	}
}

func TestTicketPickerViewRendersRows(t *testing.T) {
	items := []TicketPickerItem{
		{ID: "bmx-1", Title: "Fix bug", Status: "open", IssueType: "bug", Priority: 1},
		{ID: "bmx-2", Title: "Add feature", Status: "in_progress", IssueType: "task", Priority: 2},
	}
	d := NewTicketPicker(items)
	d.SetSize(80, 24)
	d.Show()

	view := d.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}
	if !strings.Contains(view, "bmx-1") {
		t.Fatal("expected view to contain ticket ID bmx-1")
	}
	if !strings.Contains(view, "No ticket") {
		t.Fatal("expected view to contain No ticket option")
	}
}

func TestStatusIconMapping(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"open", Icons.Idle},
		{"in_progress", Icons.Pending},
		{"closed", Icons.Clean},
		{"unknown", Icons.Idle},
	}
	for _, tt := range tests {
		got := statusIcon(tt.status)
		if got != tt.want {
			t.Errorf("statusIcon(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
	if got := truncate("hello world", 6); got != "hello…" {
		t.Fatalf("expected 'hello…', got %q", got)
	}
	if got := truncate("", 5); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	if got := truncate("hi", 0); got != "" {
		t.Fatalf("expected empty for maxLen 0, got %q", got)
	}
}
