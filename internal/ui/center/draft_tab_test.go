package center

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/config"
	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/tickets"
)

// --- Draft-through-DraftTab integration tests ---
// These tests exercise Draft through the tab dispatch path (Model.Update, Model.View)
// rather than calling Draft methods directly. Existing Draft unit tests in
// draft_test.go and draft_slots_test.go cover Draft in isolation and are unchanged.

func tabDispatchConfig() *config.Config {
	return &config.Config{
		Assistants: map[string]config.AssistantConfig{
			"claude": {
				Command:         "claude",
				CommandTemplate: "claude --model {{.Model}} --agent {{.Agent}}",
				PromptTemplate:  "Work on {{.TicketID}}: {{.TicketTitle}}",
				SupportedModels: []string{"sonnet", "opus"},
				SupportedAgents: []string{"auto-approve", "plan"},
			},
		},
	}
}

func tabDispatchWorkspace() *data.Workspace {
	return &data.Workspace{Name: "test", Repo: "/repo", Root: "/repo"}
}

func tabDispatchTicket() *tickets.Ticket {
	return &tickets.Ticket{ID: "bmx-99", Title: "Tab Dispatch Test"}
}

// setupDraftTab creates a Model with a DraftTab set up and active.
// Sets m.workspace so that getTabs() and getActiveTabIdx() resolve correctly.
func setupDraftTab(t *testing.T) (*Model, *Tab) {
	t.Helper()
	m := newTestModel()
	m.config = tabDispatchConfig()
	m.height = 26 // required for View() to not truncate content to 0 lines
	ws := tabDispatchWorkspace()
	m.workspace = ws // required for tab dispatch routing

	ticket := tabDispatchTicket()
	draft := NewDraft(ticket, ws, m.config, m.styles)
	draft.SetSize(80, 24)

	tab := &Tab{
		ID:        generateTabID(),
		Name:      "Draft",
		Kind:      DraftTab,
		Draft:     draft,
		Workspace: ws,
	}
	wsID := string(ws.ID())
	m.tabsByWorkspace[wsID] = []*Tab{tab}
	m.activeTabByWorkspace[wsID] = 0

	return m, tab
}

// TestDraftTabStartDraftCreatesDraftTab verifies that StartDraft creates a
// tab with Kind=DraftTab, sets tab.Draft, and makes it the active tab.
func TestDraftTabStartDraftCreatesDraftTab(t *testing.T) {
	m := newTestModel()
	m.config = tabDispatchConfig()
	ws := tabDispatchWorkspace()
	m.workspace = ws
	wsID := string(ws.ID())
	ticket := tabDispatchTicket()

	m.StartDraft(ticket, ws)

	tabs := m.tabsByWorkspace[wsID]
	if len(tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(tabs))
	}

	tab := tabs[0]
	if tab.Kind != DraftTab {
		t.Errorf("expected Kind=DraftTab, got %d", tab.Kind)
	}
	if tab.Draft == nil {
		t.Fatal("tab.Draft should not be nil")
	}
	if tab.Draft.ticket == nil || tab.Draft.ticket.ID != "bmx-99" {
		t.Error("tab.Draft.ticket not set correctly")
	}

	activeIdx := m.activeTabByWorkspace[wsID]
	if activeIdx != 0 {
		t.Errorf("expected active tab index 0, got %d", activeIdx)
	}
}

// TestDraftTabInputDispatchEnterAdvanceDraft verifies that pressing Enter
// through Model.Update routes to the Draft in the active DraftTab and advances
// from SlotHarness to SlotModel.
func TestDraftTabInputDispatchEnterAdvanceDraft(t *testing.T) {
	m, tab := setupDraftTab(t)

	if tab.Draft.activeSlot != SlotHarness {
		t.Fatalf("expected SlotHarness initially, got %d", tab.Draft.activeSlot)
	}

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if tab.Draft.activeSlot != SlotModel {
		t.Errorf("expected SlotModel after Enter via tab dispatch, got %d", tab.Draft.activeSlot)
	}
	if tab.Draft.harness == "" {
		t.Error("harness should be set after selection")
	}
}

// TestDraftTabInputDispatchEscapeCancels verifies that pressing Escape
// through Model.Update cancels the draft and closes the DraftTab.
func TestDraftTabInputDispatchEscapeCancels(t *testing.T) {
	m, _ := setupDraftTab(t)
	wsID := string(tabDispatchWorkspace().ID())

	if len(m.tabsByWorkspace[wsID]) != 1 {
		t.Fatal("expected 1 tab before cancel")
	}

	// At SlotHarness, Escape emits DraftCancelled.
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	msg := cmd()
	if _, ok := msg.(DraftCancelled); !ok {
		t.Fatalf("expected DraftCancelled, got %T", msg)
	}

	// Process DraftCancelled — should close the tab.
	m, _ = m.Update(msg)
	if len(m.tabsByWorkspace[wsID]) != 0 {
		t.Errorf("expected tab to be closed after DraftCancelled, got %d tabs", len(m.tabsByWorkspace[wsID]))
	}
	if m.draft != nil {
		t.Error("m.draft should be nil after DraftCancelled")
	}
}

// TestDraftTabViewRendersDraftContent verifies that Model.View() renders
// draft content when the active tab is a DraftTab.
func TestDraftTabViewRendersDraftContent(t *testing.T) {
	m, _ := setupDraftTab(t)

	view := m.View()

	if !strings.Contains(view, "Draft") {
		t.Error("view should contain 'Draft' tab name")
	}
	if !strings.Contains(view, "bmx-99") {
		t.Error("view should contain ticket ID 'bmx-99'")
	}
	if !strings.Contains(view, "Step") {
		t.Error("view should contain step indicator from draft")
	}
}

// TestDraftTabInputDispatchFilter verifies that typing filter text through
// Model.Update routes to the Draft in the active DraftTab.
func TestDraftTabInputDispatchFilter(t *testing.T) {
	m, tab := setupDraftTab(t)
	// Use a config with multiple harnesses so filtering actually narrows.
	m.config = &config.Config{
		Assistants: map[string]config.AssistantConfig{
			"claude":   {Command: "claude", SupportedModels: []string{"sonnet"}},
			"opencode": {Command: "opencode", SupportedModels: []string{"gpt-4"}},
			"codex":    {Command: "codex"},
		},
	}
	// Rebuild draft with the multi-harness config.
	ws := tabDispatchWorkspace()
	d := NewDraft(tabDispatchTicket(), ws, m.config, m.styles)
	d.SetSize(80, 24)
	tab.Draft = d

	_, _ = m.Update(tea.KeyPressMsg{Text: "op"})

	// "op" should narrow to only "opencode" (fuzzy match: sequential chars).
	if len(tab.Draft.filteredIndices) != 1 {
		t.Errorf("filter 'op' should narrow to 1 option, got %d: %v",
			len(tab.Draft.filteredIndices), tab.Draft.filteredIndices)
	}
	if tab.Draft.harnessOptions[tab.Draft.filteredIndices[0]] != "opencode" {
		t.Errorf("expected opencode, got %s",
			tab.Draft.harnessOptions[tab.Draft.filteredIndices[0]])
	}
}

// TestDraftTabCompleteReturnsMetadata verifies full draft flow through tab
// dispatch produces a DraftComplete with correct metadata.
func TestDraftTabCompleteReturnsMetadata(t *testing.T) {
	m, tab := setupDraftTab(t)

	// Step through the draft: harness → model → agent → confirm
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // select harness ("claude")
	if tab.Draft.harness == "" {
		t.Fatal("harness not selected")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // select model ("sonnet")
	if tab.Draft.model == "" {
		t.Fatal("model not selected")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // select agent ("auto-approve")
	if tab.Draft.agent == "" {
		t.Fatal("agent not selected")
	}

	// Now at SlotConfirm — Enter should emit DraftComplete
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	msg := cmd()
	dc, ok := msg.(DraftComplete)
	if !ok {
		t.Fatalf("expected DraftComplete, got %T", msg)
	}
	if dc.Assistant != "claude" {
		t.Errorf("expected assistant=claude, got %s", dc.Assistant)
	}
	if dc.TicketID != "bmx-99" {
		t.Errorf("expected ticketID=bmx-99, got %s", dc.TicketID)
	}
	if dc.TicketTitle != "Tab Dispatch Test" {
		t.Errorf("expected ticketTitle, got %s", dc.TicketTitle)
	}
	if dc.Workspace == nil {
		t.Error("workspace should not be nil")
	}

	// DraftComplete should clear m.draft and emit LaunchAgent
	m, cmd2 := m.Update(msg)
	if m.draft != nil {
		t.Error("m.draft should be nil after DraftComplete processed")
	}
	_ = cmd2 // LaunchAgent command (ignored in test)
}

// TestDraftTabMouseBlocked verifies that mouse events are blocked when the
// active tab is a DraftTab.
func TestDraftTabMouseBlocked(t *testing.T) {
	m, _ := setupDraftTab(t)

	_, cmd := m.Update(tea.MouseWheelMsg{})
	if cmd != nil {
		t.Error("mouse wheel should return nil cmd when DraftTab is active")
	}

	_, cmd = m.Update(tea.MouseClickMsg{})
	if cmd != nil {
		t.Error("mouse click should return nil cmd when DraftTab is active")
	}
}

// TestDraftTabKeepsIsolatedDraftUnitTests verifies that existing Draft unit
// tests (Draft in isolation, no Model) still work unchanged. This is mandated
// by the acceptance criteria: "Existing Draft unit tests still pass".
func TestDraftTabKeepsIsolatedDraftUnitTests(t *testing.T) {
	// Same as TestDraftNewStartsAtHarnessSlot — Draft created directly, no Model.
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	if d.activeSlot != SlotHarness {
		t.Errorf("expected activeSlot=SlotHarness, got %d", d.activeSlot)
	}
	if d.ticket == nil || d.ticket.ID != "bmx-1" {
		t.Error("ticket not set")
	}
	if len(d.harnessOptions) != 3 {
		t.Errorf("expected 3 harness options, got %d", len(d.harnessOptions))
	}

	// Same as TestDraftCompleteReturnsAllMetadata — Draft in isolation.
	d2 := NewDraft(draftTicket("bmx-42", "My Ticket"), draftWorkspace(), draftConfig(), draftStyles())
	d2.SetSize(80, 24)
	d2.confirmHarness("claude")
	d2.model = "sonnet"
	d2.activeSlot = SlotAgent
	d2.resetFilter(d2.agentOptions)
	d2.agent = "auto-approve"
	d2.activeSlot = SlotComplete

	cmd := d2.launchCmd()
	msg := cmd()
	dc, ok := msg.(DraftComplete)
	if !ok {
		t.Fatalf("expected DraftComplete, got %T", msg)
	}
	if dc.Assistant != "claude" || dc.TicketID != "bmx-42" || dc.TicketTitle != "My Ticket" ||
		dc.Model != "sonnet" || dc.AgentMode != "auto-approve" || dc.Workspace == nil {
		t.Error("DraftComplete metadata mismatch")
	}
}
