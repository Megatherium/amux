// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package tickets

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestTicket(t *testing.T) {
	t.Run("NewTicket valid", func(t *testing.T) {
		ticket, err := NewTicket("bmx-123", "Fix bug")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ticket.ID != "bmx-123" {
			t.Errorf("expected ID bmx-123, got %s", ticket.ID)
		}
		if ticket.Title != "Fix bug" {
			t.Errorf("expected title 'Fix bug', got %s", ticket.Title)
		}
	})

	t.Run("NewTicket empty id", func(t *testing.T) {
		_, err := NewTicket("", "Fix bug")
		if err == nil {
			t.Fatal("expected error for empty id")
		}
	})

	t.Run("NewTicket empty title", func(t *testing.T) {
		_, err := NewTicket("bmx-123", "")
		if err == nil {
			t.Fatal("expected error for empty title")
		}
	})
}

func TestTicketJSON(t *testing.T) {
	now := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
	ticket := Ticket{
		ID:          "bmx-123",
		Title:       "Fix bug",
		Description: "A description",
		Status:      "open",
		Priority:    1,
		IssueType:   "bug",
		Assignee:    "alice",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	data, err := json.Marshal(ticket)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled Ticket
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.ID != ticket.ID {
		t.Errorf("ID mismatch: got %s, want %s", unmarshaled.ID, ticket.ID)
	}
	if unmarshaled.Title != ticket.Title {
		t.Errorf("Title mismatch: got %s, want %s", unmarshaled.Title, ticket.Title)
	}
}

func TestSelection(t *testing.T) {
	t.Run("NewSelection valid", func(t *testing.T) {
		ticket, _ := NewTicket("bmx-123", "Fix bug")
		sel, err := NewSelection(ticket, "claude", "anthropic/claude-sonnet-4", "coder")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if sel.Assistant != "claude" {
			t.Errorf("expected assistant claude, got %s", sel.Assistant)
		}
		if sel.Model != "anthropic/claude-sonnet-4" {
			t.Errorf("expected model anthropic/claude-sonnet-4, got %s", sel.Model)
		}
		if sel.Agent != "coder" {
			t.Errorf("expected agent coder, got %s", sel.Agent)
		}
	})

	t.Run("NewSelection empty assistant", func(t *testing.T) {
		ticket, _ := NewTicket("bmx-123", "Fix bug")
		_, err := NewSelection(ticket, "", "model", "agent")
		if err == nil {
			t.Fatal("expected error for empty assistant")
		}
	})
}

func TestSelectionJSON(t *testing.T) {
	ticket, _ := NewTicket("bmx-123", "Fix bug")
	sel := Selection{
		Ticket:    ticket,
		Assistant: "claude",
		Model:     "anthropic/claude-sonnet-4",
		Agent:     "coder",
	}

	data, err := json.Marshal(sel)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled Selection
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.Assistant != sel.Assistant {
		t.Errorf("Assistant mismatch: got %s, want %s", unmarshaled.Assistant, sel.Assistant)
	}
}

func TestLaunchSpec(t *testing.T) {
	t.Run("NewLaunchSpec valid", func(t *testing.T) {
		ticket, _ := NewTicket("bmx-123", "Fix bug")
		sel, _ := NewSelection(ticket, "claude", "model", "agent")
		spec, err := NewLaunchSpec(sel, "echo hello", "prompt", "launcher-1", "/work")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if spec.RenderedCommand != "echo hello" {
			t.Errorf("expected rendered command 'echo hello', got %s", spec.RenderedCommand)
		}
	})

	t.Run("NewLaunchSpec empty command", func(t *testing.T) {
		ticket, _ := NewTicket("bmx-123", "Fix bug")
		sel, _ := NewSelection(ticket, "claude", "model", "agent")
		_, err := NewLaunchSpec(sel, "", "prompt", "launcher-1", "/work")
		if err == nil {
			t.Fatal("expected error for empty command")
		}
	})
}

func TestLaunchResult(t *testing.T) {
	t.Run("LaunchResult JSON excludes error", func(t *testing.T) {
		result := LaunchResult{
			LauncherID: "win-1",
			PID:        12345,
			Error:      errors.New("test error"),
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled LaunchResult
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if unmarshaled.LauncherID != result.LauncherID {
			t.Errorf("LauncherID mismatch: got %s, want %s", unmarshaled.LauncherID, result.LauncherID)
		}
		if unmarshaled.PID != result.PID {
			t.Errorf("PID mismatch: got %d, want %d", unmarshaled.PID, result.PID)
		}
	})
}

func TestPriorityLabel(t *testing.T) {
	tests := []struct {
		p    int
		want string
	}{
		{0, "P?"},
		{-1, "P?"},
		{1, "P0 critical"},
		{2, "P1 high"},
		{3, "P2 medium"},
		{4, "P3 low"},
		{5, "P4"},
		{10, "P9"},
	}
	for _, tt := range tests {
		got := PriorityLabel(tt.p)
		if got != tt.want {
			t.Errorf("PriorityLabel(%d) = %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestPriorityLabelShort(t *testing.T) {
	tests := []struct {
		p    int
		want string
	}{
		{0, "?"},
		{-1, "?"},
		{1, "P0"},
		{2, "P1"},
		{3, "P2"},
		{4, "P3"},
		{5, "P4"},
		{10, "P9"},
	}
	for _, tt := range tests {
		got := PriorityLabelShort(tt.p)
		if got != tt.want {
			t.Errorf("PriorityLabelShort(%d) = %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestModelContext(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		m := NewModelContext("anthropic/claude/claude-sonnet-4")
		if m.String() != "anthropic/claude/claude-sonnet-4" {
			t.Errorf("String() = %s, want %s", m.String(), "anthropic/claude/claude-sonnet-4")
		}
	})

	t.Run("ModelID", func(t *testing.T) {
		m := NewModelContext("anthropic/claude/claude-sonnet-4")
		if m.ModelID() != "anthropic/claude/claude-sonnet-4" {
			t.Errorf("ModelID() = %s, want %s", m.ModelID(), "anthropic/claude/claude-sonnet-4")
		}
	})

	t.Run("empty model", func(t *testing.T) {
		var m ModelContext
		if m.Provider() != "" {
			t.Errorf("empty Provider() = %q, want %q", m.Provider(), "")
		}
		if m.Org() != "" {
			t.Errorf("empty Org() = %q, want %q", m.Org(), "")
		}
		if m.Name() != "" {
			t.Errorf("empty Name() = %q, want %q", m.Name(), "")
		}
	})

	t.Run("single segment", func(t *testing.T) {
		m := NewModelContext("claude-sonnet-4")
		if m.Provider() != "" {
			t.Errorf("Provider() = %q, want %q", m.Provider(), "")
		}
		if m.Org() != "" {
			t.Errorf("Org() = %q, want %q", m.Org(), "")
		}
		if m.Name() != "claude-sonnet-4" {
			t.Errorf("Name() = %q, want %q", m.Name(), "claude-sonnet-4")
		}
	})

	t.Run("two segments", func(t *testing.T) {
		m := NewModelContext("anthropic/claude")
		if m.Provider() != "anthropic" {
			t.Errorf("Provider() = %q, want %q", m.Provider(), "anthropic")
		}
		if m.Org() != "" {
			t.Errorf("Org() = %q, want %q", m.Org(), "")
		}
		if m.Name() != "claude" {
			t.Errorf("Name() = %q, want %q", m.Name(), "claude")
		}
	})

	t.Run("three segments", func(t *testing.T) {
		m := NewModelContext("anthropic/claude/claude-sonnet-4")
		if m.Provider() != "anthropic" {
			t.Errorf("Provider() = %q, want %q", m.Provider(), "anthropic")
		}
		if m.Org() != "claude" {
			t.Errorf("Org() = %q, want %q", m.Org(), "claude")
		}
		if m.Name() != "claude-sonnet-4" {
			t.Errorf("Name() = %q, want %q", m.Name(), "claude-sonnet-4")
		}
	})

	t.Run("four segments - name joins remaining", func(t *testing.T) {
		m := NewModelContext("a/b/c/d")
		if m.Provider() != "a" {
			t.Errorf("Provider() = %q, want %q", m.Provider(), "a")
		}
		if m.Org() != "b" {
			t.Errorf("Org() = %q, want %q", m.Org(), "b")
		}
		if m.Name() != "c/d" {
			t.Errorf("Name() = %q, want %q", m.Name(), "c/d")
		}
	})

	t.Run("Organization alias", func(t *testing.T) {
		m := NewModelContext("anthropic/claude/claude-sonnet-4")
		if m.Organization() != m.Org() {
			t.Errorf("Organization() = %q, Org() = %q, want equal", m.Organization(), m.Org())
		}
	})
}

func TestModelContextJSON(t *testing.T) {
	m := NewModelContext("anthropic/claude/claude-sonnet-4")

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled ModelContext
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled != m {
		t.Errorf("unmarshaled = %q, want %q", unmarshaled, m)
	}
}
