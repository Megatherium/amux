package center

import (
	"testing"

	appPty "github.com/andyrewlee/amux/internal/pty"
	"github.com/andyrewlee/amux/internal/tmux"
)

func TestFormatTabID_StaysUniqueAcrossCounterReset(t *testing.T) {
	first := formatTabID("proc-a", 1)
	second := formatTabID("proc-b", 1)

	if first == second {
		t.Fatalf("expected distinct tab ids across simulated restart, got %q", first)
	}
}

func TestHandlePtyTabCreated_PreservesTicketMetadata(t *testing.T) {
	m := newTestModel()
	ws := newTestWorkspace("ws", "/repo/ws")
	wsID := string(ws.ID())

	_ = m.handlePtyTabCreated(ptyTabCreateResult{
		Workspace:   ws,
		Assistant:   "claude",
		Agent:       &appPty.Agent{Session: "sess-ticket"},
		TabID:       TabID("tab-ticket"),
		Rows:        24,
		Cols:        80,
		Activate:    true,
		TicketID:    "bmx-3hd",
		TicketTitle: "Tab creation with ticket context injection",
		Model:       "claude-sonnet-4-20250514",
		AgentMode:   "code",
	})

	tabs := m.tabsByWorkspace[wsID]
	if len(tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(tabs))
	}
	tab := tabs[0]
	if tab.TicketID != "bmx-3hd" {
		t.Errorf("TicketID: got %q, want %q", tab.TicketID, "bmx-3hd")
	}
	if tab.TicketTitle != "Tab creation with ticket context injection" {
		t.Errorf("TicketTitle: got %q, want %q", tab.TicketTitle, "Tab creation with ticket context injection")
	}
	if tab.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model: got %q, want %q", tab.Model, "claude-sonnet-4-20250514")
	}
	if tab.AgentMode != "code" {
		t.Errorf("AgentMode: got %q, want %q", tab.AgentMode, "code")
	}
}

func TestHandlePtyTabCreated_PreservesEmptyTicketMetadata(t *testing.T) {
	m := newTestModel()
	ws := newTestWorkspace("ws", "/repo/ws")
	wsID := string(ws.ID())

	_ = m.handlePtyTabCreated(ptyTabCreateResult{
		Workspace: ws,
		Assistant: "claude",
		Agent:     &appPty.Agent{Session: "sess-noticket"},
		TabID:     TabID("tab-noticket"),
		Rows:      24,
		Cols:      80,
		Activate:  true,
		// No ticket metadata — all fields empty.
	})

	tabs := m.tabsByWorkspace[wsID]
	if len(tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(tabs))
	}
	tab := tabs[0]
	if tab.TicketID != "" {
		t.Errorf("TicketID: got %q, want empty", tab.TicketID)
	}
	if tab.TicketTitle != "" {
		t.Errorf("TicketTitle: got %q, want empty", tab.TicketTitle)
	}
	if tab.Model != "" {
		t.Errorf("Model: got %q, want empty", tab.Model)
	}
	if tab.AgentMode != "" {
		t.Errorf("AgentMode: got %q, want empty", tab.AgentMode)
	}
}

func TestHandlePtyTabCreated_RetargetPreservesTicketMetadata(t *testing.T) {
	m := newTestModel()
	ws := newTestWorkspace("ws", "/repo/ws")
	wsID := string(ws.ID())
	targetID := TabID("tab-target")
	existing := &Tab{
		ID:          targetID,
		Name:        "old-name",
		Assistant:   "codex",
		Workspace:   ws,
		SessionName: "sess-orig",
		Running:     true,
		// Existing tab has no ticket metadata initially
	}
	m.tabsByWorkspace[wsID] = []*Tab{existing}

	_ = m.handlePtyTabCreated(ptyTabCreateResult{
		Workspace:   ws,
		Assistant:   "claude",
		Agent:       &appPty.Agent{Session: "sess-ticket"},
		TabID:       targetID,
		Rows:        24,
		Cols:        80,
		Activate:    true,
		TicketID:    "bmx-3hd",
		TicketTitle: "Tab creation with ticket context injection",
		Model:       "claude-sonnet-4-20250514",
		AgentMode:   "code",
	})

	tabs := m.tabsByWorkspace[wsID]
	if len(tabs) != 1 {
		t.Fatalf("expected 1 tab after retarget (not a new tab), got %d", len(tabs))
	}
	tab := tabs[0]
	if tab.ID != targetID {
		t.Fatalf("expected same tab id %q, got %q", targetID, tab.ID)
	}
	// Non-ticket fields should also be updated by the retarget path.
	if tab.Assistant != "claude" {
		t.Errorf("Assistant: got %q, want %q", tab.Assistant, "claude")
	}
	// Ticket metadata must be updated.
	if tab.TicketID != "bmx-3hd" {
		t.Errorf("TicketID: got %q, want %q", tab.TicketID, "bmx-3hd")
	}
	if tab.TicketTitle != "Tab creation with ticket context injection" {
		t.Errorf("TicketTitle: got %q, want %q", tab.TicketTitle, "Tab creation with ticket context injection")
	}
	if tab.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model: got %q, want %q", tab.Model, "claude-sonnet-4-20250514")
	}
	if tab.AgentMode != "code" {
		t.Errorf("AgentMode: got %q, want %q", tab.AgentMode, "code")
	}
}

func TestHandlePtyTabCreated_DoesNotRetargetExistingTabOnSessionReuse(t *testing.T) {
	m := newTestModel()
	ws := newTestWorkspace("ws", "/repo/ws")
	wsID := string(ws.ID())
	reusedSession := tmux.SessionName("amux", wsID, "tab-reused")
	existing := &Tab{
		ID:          TabID("tab-existing"),
		Name:        "claude",
		Assistant:   "claude",
		Workspace:   ws,
		SessionName: reusedSession,
		Running:     true,
	}
	m.tabsByWorkspace[wsID] = []*Tab{existing}

	_ = m.handlePtyTabCreated(ptyTabCreateResult{
		Workspace: ws,
		Assistant: "codex",
		Agent:     &appPty.Agent{Session: reusedSession},
		TabID:     TabID("tab-new"),
		Rows:      24,
		Cols:      80,
		Activate:  true,
	})

	tabs := m.tabsByWorkspace[wsID]
	if len(tabs) != 2 {
		t.Fatalf("expected new tab to be added without mutating existing tab, got %d tabs", len(tabs))
	}
	if tabs[0].Assistant != "claude" {
		t.Fatalf("expected existing tab assistant to remain claude, got %q", tabs[0].Assistant)
	}
	if tabs[1].Assistant != "codex" {
		t.Fatalf("expected new tab assistant codex, got %q", tabs[1].Assistant)
	}
	if tabs[1].ID != TabID("tab-new") {
		t.Fatalf("expected new tab id to be preserved, got %q", tabs[1].ID)
	}
}
