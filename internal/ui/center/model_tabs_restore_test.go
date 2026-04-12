package center

import (
	"testing"
	"time"

	"github.com/andyrewlee/amux/internal/data"
)

func TestAddDetachedTab_SetsLastFocusedFromCreatedAt(t *testing.T) {
	m := newTestModel()
	ws := newTestWorkspace("ws", "/repo/ws")
	wsID := string(ws.ID())
	createdAt := time.Now().Add(-time.Hour).Unix()

	m.addDetachedTab(ws, data.TabInfo{
		Assistant:   "claude",
		Name:        "Claude",
		SessionName: "sess-detached",
		CreatedAt:   createdAt,
	})

	tabs := m.tabsByWorkspace[wsID]
	if len(tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(tabs))
	}
	if tabs[0].lastFocusedAt != time.Unix(createdAt, 0) {
		t.Fatalf("expected lastFocusedAt=%s, got %s", time.Unix(createdAt, 0), tabs[0].lastFocusedAt)
	}
	if tabs[0].Terminal == nil {
		t.Fatal("expected detached tab terminal")
	}
	if !tabs[0].Terminal.TreatLFAsCRLF {
		t.Fatal("expected chat detached tab to normalize LF as CRLF")
	}
}

func TestAddPlaceholderTab_SetsLastFocusedFromCreatedAt(t *testing.T) {
	m := newTestModel()
	ws := newTestWorkspace("ws", "/repo/ws")
	wsID := string(ws.ID())
	createdAt := time.Now().Add(-2 * time.Hour).Unix()

	_, _ = m.addPlaceholderTab(ws, data.TabInfo{
		Assistant: "claude",
		Name:      "Claude",
		CreatedAt: createdAt,
	})

	tabs := m.tabsByWorkspace[wsID]
	if len(tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(tabs))
	}
	if tabs[0].lastFocusedAt != time.Unix(createdAt, 0) {
		t.Fatalf("expected lastFocusedAt=%s, got %s", time.Unix(createdAt, 0), tabs[0].lastFocusedAt)
	}
	if tabs[0].Terminal == nil {
		t.Fatal("expected placeholder tab terminal")
	}
	if !tabs[0].Terminal.TreatLFAsCRLF {
		t.Fatal("expected chat placeholder tab to normalize LF as CRLF")
	}
}

func TestRestoreTabsFromWorkspace_MarksReattachInFlightForRunningTabs(t *testing.T) {
	m := newTestModel()
	ws := newTestWorkspace("ws", "/repo/ws")
	ws.OpenTabs = []data.TabInfo{
		{
			Assistant:   "claude",
			Name:        "Claude",
			Status:      "running",
			SessionName: "sess-running",
		},
	}
	wsID := string(ws.ID())

	if cmd := m.RestoreTabsFromWorkspace(ws); cmd == nil {
		t.Fatalf("expected restore command for running tab")
	}

	tabs := m.tabsByWorkspace[wsID]
	if len(tabs) != 1 {
		t.Fatalf("expected 1 restored tab, got %d", len(tabs))
	}
	tab := tabs[0]
	tab.mu.Lock()
	inFlight := tab.reattachInFlight
	detached := tab.Detached
	tab.mu.Unlock()
	if !detached {
		t.Fatalf("expected restored placeholder tab to be detached before reattach result")
	}
	if !inFlight {
		t.Fatalf("expected restored placeholder tab to start with reattachInFlight=true")
	}
}

func TestAutoReattachActiveTabOnSelection_SkipsRestoreInFlightPlaceholder(t *testing.T) {
	m := newTestModel()
	ws := newTestWorkspace("ws", "/repo/ws")
	ws.OpenTabs = []data.TabInfo{
		{
			Assistant:   "claude",
			Name:        "Claude",
			Status:      "running",
			SessionName: "sess-running",
		},
	}
	wsID := string(ws.ID())

	_ = m.RestoreTabsFromWorkspace(ws)
	m.workspace = ws
	m.activeTabByWorkspace[wsID] = 0

	if cmd := m.autoReattachActiveTabOnSelection(); cmd != nil {
		t.Fatalf("expected auto reattach to skip while restore reattach is in flight")
	}
}

func TestAddDetachedTab_PreservesTicketMetadata(t *testing.T) {
	m := newTestModel()
	ws := newTestWorkspace("ws", "/repo/ws")
	wsID := string(ws.ID())

	m.addDetachedTab(ws, data.TabInfo{
		Assistant:   "claude",
		Name:        "Claude",
		SessionName: "sess-detached",
		TicketID:    "bmx-42",
		TicketTitle: "Fix the thing",
		Model:       "claude-sonnet-4-20250514",
		Agent:       "code",
	})

	tabs := m.tabsByWorkspace[wsID]
	if len(tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(tabs))
	}
	tab := tabs[0]
	if tab.TicketID != "bmx-42" {
		t.Errorf("TicketID: got %q, want %q", tab.TicketID, "bmx-42")
	}
	if tab.TicketTitle != "Fix the thing" {
		t.Errorf("TicketTitle: got %q, want %q", tab.TicketTitle, "Fix the thing")
	}
	if tab.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model: got %q, want %q", tab.Model, "claude-sonnet-4-20250514")
	}
	if tab.AgentMode != "code" {
		t.Errorf("AgentMode: got %q, want %q", tab.AgentMode, "code")
	}
}

func TestAddPlaceholderTab_PreservesTicketMetadata(t *testing.T) {
	m := newTestModel()
	ws := newTestWorkspace("ws", "/repo/ws")
	wsID := string(ws.ID())

	_, _ = m.addPlaceholderTab(ws, data.TabInfo{
		Assistant:   "claude",
		Name:        "Claude",
		SessionName: "sess-placeholder",
		TicketID:    "bmx-99",
		TicketTitle: "Implement feature",
		Model:       "gpt-4o",
		Agent:       "plan",
	})

	tabs := m.tabsByWorkspace[wsID]
	if len(tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(tabs))
	}
	tab := tabs[0]
	if tab.TicketID != "bmx-99" {
		t.Errorf("TicketID: got %q, want %q", tab.TicketID, "bmx-99")
	}
	if tab.TicketTitle != "Implement feature" {
		t.Errorf("TicketTitle: got %q, want %q", tab.TicketTitle, "Implement feature")
	}
	if tab.Model != "gpt-4o" {
		t.Errorf("Model: got %q, want %q", tab.Model, "gpt-4o")
	}
	if tab.AgentMode != "plan" {
		t.Errorf("AgentMode: got %q, want %q", tab.AgentMode, "plan")
	}
}

func TestGetTabsInfoForWorkspace_PreservesTicketMetadata(t *testing.T) {
	m := newTestModel()
	ws := newTestWorkspace("ws", "/repo/ws")
	wsID := string(ws.ID())

	m.addDetachedTab(ws, data.TabInfo{
		Assistant:   "claude",
		Name:        "Claude",
		SessionName: "sess-1",
		TicketID:    "bmx-7",
		TicketTitle: "Roundtrip test",
		Model:       "claude-sonnet-4-20250514",
		Agent:       "code",
	})
	m.addDetachedTab(ws, data.TabInfo{
		Assistant:   "codex",
		Name:        "Codex",
		SessionName: "sess-2",
	})

	infos, _ := m.GetTabsInfoForWorkspace(wsID)
	if len(infos) != 2 {
		t.Fatalf("expected 2 tab infos, got %d", len(infos))
	}

	got := infos[0]
	if got.TicketID != "bmx-7" {
		t.Errorf("TicketID: got %q, want %q", got.TicketID, "bmx-7")
	}
	if got.TicketTitle != "Roundtrip test" {
		t.Errorf("TicketTitle: got %q, want %q", got.TicketTitle, "Roundtrip test")
	}
	if got.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model: got %q, want %q", got.Model, "claude-sonnet-4-20250514")
	}
	if got.Agent != "code" {
		t.Errorf("Agent: got %q, want %q", got.Agent, "code")
	}

	empty := infos[1]
	if empty.TicketID != "" {
		t.Errorf("TicketID: got %q, want empty", empty.TicketID)
	}
	if empty.Agent != "" {
		t.Errorf("Agent: got %q, want empty", empty.Agent)
	}
}

func TestGetTabsInfo_PreservesTicketMetadata(t *testing.T) {
	m := newTestModel()
	ws := newTestWorkspace("ws", "/repo/ws")

	m.workspace = ws
	m.addDetachedTab(ws, data.TabInfo{
		Assistant:   "claude",
		Name:        "Claude",
		SessionName: "sess-1",
		TicketID:    "bmx-55",
		TicketTitle: "GetTabsInfo roundtrip",
		Model:       "gemini-pro",
		Agent:       "plan",
	})

	infos, _ := m.GetTabsInfo()
	if len(infos) != 1 {
		t.Fatalf("expected 1 tab info, got %d", len(infos))
	}
	got := infos[0]
	if got.TicketID != "bmx-55" {
		t.Errorf("TicketID: got %q, want %q", got.TicketID, "bmx-55")
	}
	if got.TicketTitle != "GetTabsInfo roundtrip" {
		t.Errorf("TicketTitle: got %q, want %q", got.TicketTitle, "GetTabsInfo roundtrip")
	}
	if got.Model != "gemini-pro" {
		t.Errorf("Model: got %q, want %q", got.Model, "gemini-pro")
	}
	if got.Agent != "plan" {
		t.Errorf("Agent: got %q, want %q", got.Agent, "plan")
	}
}
