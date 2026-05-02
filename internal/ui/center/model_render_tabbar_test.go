package center

import (
	"strings"
	"testing"
	"time"

	"github.com/andyrewlee/amux/internal/config"
	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/tickets"
)

// --- Tab bar rendering tests for mixed tab kinds (bmx-ywa.5) ---

func TestTabBarRendersMixedKinds(t *testing.T) {
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
	now := time.Now()

	m.tabsByWorkspace[wsID] = []*Tab{
		{ID: "a", Name: "Agent", Kind: AgentTab, Workspace: ws, createdAt: now.Unix()},
		{
			ID: "d", Name: "Draft", Kind: DraftTab, Workspace: ws, createdAt: now.Unix(),
			Draft: &Draft{ticket: &tickets.Ticket{ID: "bmx-1", Title: "Test"}},
		},
		{
			ID: "t", Name: "Ticket", Kind: TicketViewTab, Workspace: ws, createdAt: now.Unix(),
			Ticket: &tickets.Ticket{ID: "bmx-2", Title: "View", Status: "open"},
		},
	}
	m.activeTabByWorkspace[wsID] = 0

	view := m.View()

	// Tab bar should show all three tab names.
	if !strings.Contains(view, "Agent") {
		t.Error("tab bar should contain 'Agent' name")
	}
	if !strings.Contains(view, "Draft") {
		t.Error("tab bar should contain 'Draft' name")
	}
	if !strings.Contains(view, "Ticket") {
		t.Error("tab bar should contain 'Ticket' name")
	}
	// Active tab should render content (AgentTab without terminal shows empty).
	// View should not panic with mixed kinds.
}

func TestTabBarActiveInactiveStyling(t *testing.T) {
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
	now := time.Now()

	m.tabsByWorkspace[wsID] = []*Tab{
		{ID: "a", Name: "First", Kind: AgentTab, Workspace: ws, createdAt: now.Unix()},
		{ID: "b", Name: "Second", Kind: AgentTab, Workspace: ws, createdAt: now.Unix()},
	}
	// Activate second tab.
	m.activeTabByWorkspace[wsID] = 1

	view := m.View()

	// Both tabs should appear (active styling is via ANSI, not structural).
	if !strings.Contains(view, "First") || !strings.Contains(view, "Second") {
		t.Error("tab bar should show both tab names")
	}
	// "+ New" button should be present.
	if !strings.Contains(view, "New") {
		t.Error("tab bar should show 'New' button")
	}
}

func TestTabBarEmptyShowsNewAgentOnly(t *testing.T) {
	m := newTestModel()
	m.config = &config.Config{
		Assistants: map[string]config.AssistantConfig{
			"claude": {Command: "claude"},
		},
	}
	m.height = 26
	ws := &data.Workspace{Name: "test", Repo: "/repo", Root: "/repo"}
	m.workspace = ws

	// No tabs — should show "New agent" prompt.
	view := m.View()

	if !strings.Contains(view, "New agent") {
		t.Error("empty tab bar should show 'New agent' prompt")
	}
}
