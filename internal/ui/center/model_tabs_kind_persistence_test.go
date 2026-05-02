package center

import (
	"strings"
	"testing"
	"time"

	"github.com/andyrewlee/amux/internal/config"
	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/tickets"
)

// --- Tab kind persistence and TicketViewTab rendering tests (bmx-ywa.5) ---

func TestGetTabsInfoPreservesTabKind(t *testing.T) {
	m := newTestModel()
	m.config = &config.Config{
		Assistants: map[string]config.AssistantConfig{
			"claude": {Command: "claude"},
		},
	}
	ws := &data.Workspace{Name: "test", Repo: "/repo", Root: "/repo"}
	wsID := string(ws.ID())
	now := time.Now()

	// Create tabs of each kind.
	m.tabsByWorkspace[wsID] = []*Tab{
		{ID: "a", Assistant: "claude", Kind: AgentTab, Workspace: ws, createdAt: now.Unix()},
		{ID: "d", Assistant: "claude", Kind: DraftTab, Workspace: ws, createdAt: now.Unix()},
		{
			ID: "t", Assistant: "claude", Kind: TicketViewTab, Workspace: ws, createdAt: now.Unix(),
			TicketID: "bmx-1", TicketTitle: "Test",
		},
	}
	m.activeTabByWorkspace[wsID] = 0

	infos, activeIdx := m.GetTabsInfoForWorkspace(wsID)
	if len(infos) != 3 {
		t.Fatalf("expected 3 infos, got %d", len(infos))
	}
	if activeIdx != 0 {
		t.Errorf("expected activeIdx=0, got %d", activeIdx)
	}

	kinds := []int{int(AgentTab), int(DraftTab), int(TicketViewTab)}
	for i, info := range infos {
		if info.Kind != kinds[i] {
			t.Errorf("info[%d].Kind: got %d, want %d", i, info.Kind, kinds[i])
		}
	}
}

func TestTabInfoRoundTripPreservesKind(t *testing.T) {
	m := newTestModel()
	m.config = &config.Config{
		Assistants: map[string]config.AssistantConfig{
			"claude": {Command: "claude"},
		},
	}
	ws := &data.Workspace{Name: "test", Repo: "/repo", Root: "/repo"}
	m.workspace = ws
	m.height = 26
	wsID := string(ws.ID())

	// Create DraftTab and TicketViewTab via TabInfo.
	info := data.TabInfo{
		Assistant:   "claude",
		Name:        "Draft",
		Kind:        int(DraftTab),
		CreatedAt:   time.Now().Unix(),
		TicketID:    "bmx-99",
		TicketTitle: "Round Trip",
	}
	tabID, _ := m.addPlaceholderTab(ws, info)
	tab := m.tabsByWorkspace[wsID][0]

	if tab.Kind != DraftTab {
		t.Errorf("round-trip: expected DraftTab kind, got %d", tab.Kind)
	}
	if tab.TicketID != "bmx-99" {
		t.Errorf("round-trip: expected TicketID=bmx-99, got %s", tab.TicketID)
	}

	// Now export via GetTabsInfo and verify Kind preserved.
	infos, _ := m.GetTabsInfoForWorkspace(wsID)
	if len(infos) != 1 {
		t.Fatalf("expected 1 info, got %d", len(infos))
	}
	if infos[0].Kind != int(DraftTab) {
		t.Errorf("exported Kind: got %d, want %d", infos[0].Kind, int(DraftTab))
	}
	if infos[0].TicketID != "bmx-99" {
		t.Errorf("exported TicketID: got %s, want bmx-99", infos[0].TicketID)
	}
	if infos[0].TicketTitle != "Round Trip" {
		t.Errorf("exported TicketTitle: got %s, want Round Trip", infos[0].TicketTitle)
	}
	_ = tabID
}

func TestTabInfoDefaultKindIsAgentTab(t *testing.T) {
	m := newTestModel()
	m.config = &config.Config{
		Assistants: map[string]config.AssistantConfig{
			"claude": {Command: "claude"},
		},
	}
	ws := &data.Workspace{Name: "test", Repo: "/repo", Root: "/repo"}

	// TabInfo with no Kind field set (zero value) — should default to AgentTab.
	info := data.TabInfo{
		Assistant: "claude",
		Name:      "Terminal",
		CreatedAt: time.Now().Unix(),
	}
	tabID, _ := m.addPlaceholderTab(ws, info)
	tab := m.tabsByWorkspace[string(ws.ID())][0]

	if tab.Kind != AgentTab {
		t.Errorf("zero Kind should default to AgentTab, got %d", tab.Kind)
	}
	_ = tabID
}

func TestTicketViewTabRendersTicketContent(t *testing.T) {
	m := newTestModel()
	m.config = &config.Config{
		Assistants: map[string]config.AssistantConfig{
			"claude": {Command: "claude"},
		},
	}
	m.height = 26
	ws := &data.Workspace{Name: "test", Repo: "/repo", Root: "/repo"}
	m.workspace = ws
	wsID := string(ws.ID())

	ticket := &tickets.Ticket{
		ID:        "bmx-100",
		Title:     "Fix render bug",
		Status:    "open",
		Priority:  2,
		IssueType: "bug",
		Assignee:  "sloth",
	}

	tab := &Tab{
		ID:        "view",
		Name:      "bmx-100",
		Kind:      TicketViewTab,
		Ticket:    ticket,
		Workspace: ws,
	}
	m.tabsByWorkspace[wsID] = []*Tab{tab}
	m.activeTabByWorkspace[wsID] = 0

	view := m.View()

	// Verify ticket content is rendered.
	if !strings.Contains(view, "bmx-100") {
		t.Error("view should contain ticket ID 'bmx-100'")
	}
	if !strings.Contains(view, "Fix render bug") {
		t.Error("view should contain ticket title")
	}
	if !strings.Contains(view, "open") {
		t.Error("view should contain status 'open'")
	}
	if !strings.Contains(view, "bug") {
		t.Error("view should contain issue type 'bug'")
	}
	if !strings.Contains(view, "sloth") {
		t.Error("view should contain assignee 'sloth'")
	}
}

func TestTicketViewTabRendersMinimalTicket(t *testing.T) {
	m := newTestModel()
	m.config = &config.Config{
		Assistants: map[string]config.AssistantConfig{
			"claude": {Command: "claude"},
		},
	}
	m.height = 26
	ws := &data.Workspace{Name: "test", Repo: "/repo", Root: "/repo"}
	m.workspace = ws
	wsID := string(ws.ID())

	// Minimal ticket: no description, no assignee, no type.
	ticket := &tickets.Ticket{
		ID:     "bmx-200",
		Title:  "Minimal",
		Status: "open",
	}

	tab := &Tab{
		ID:        "min",
		Name:      "bmx-200",
		Kind:      TicketViewTab,
		Ticket:    ticket,
		Workspace: ws,
	}
	m.tabsByWorkspace[wsID] = []*Tab{tab}
	m.activeTabByWorkspace[wsID] = 0

	view := m.View()

	if !strings.Contains(view, "bmx-200") {
		t.Error("view should contain ticket ID for minimal ticket")
	}
	if !strings.Contains(view, "Minimal") {
		t.Error("view should contain title for minimal ticket")
	}
	// Should not crash on missing fields.
	if view == "" {
		t.Error("view should not be empty for minimal ticket")
	}
}

func TestTicketViewTabNilTicketRendersEmpty(t *testing.T) {
	m := newTestModel()
	m.config = &config.Config{
		Assistants: map[string]config.AssistantConfig{
			"claude": {Command: "claude"},
		},
	}
	m.height = 26
	ws := &data.Workspace{Name: "test", Repo: "/repo", Root: "/repo"}
	m.workspace = ws
	wsID := string(ws.ID())

	// TicketViewTab with nil ticket — should render empty without panic.
	tab := &Tab{
		ID:        "nil-ticket",
		Name:      "nil",
		Kind:      TicketViewTab,
		Ticket:    nil,
		Workspace: ws,
	}
	m.tabsByWorkspace[wsID] = []*Tab{tab}
	m.activeTabByWorkspace[wsID] = 0

	view := m.View()
	// Should not panic and should render tab bar (but empty content).
	if strings.Contains(view, "panic") || strings.Contains(view, "nil pointer") {
		t.Error("nil ticket should not cause panic in view")
	}
}
