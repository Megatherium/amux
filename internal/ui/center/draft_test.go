package center

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/config"
	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/tickets"
	"github.com/andyrewlee/amux/internal/ui/common"
)

func draftConfig() *config.Config {
	return &config.Config{
		Assistants: map[string]config.AssistantConfig{
			"claude": {
				Command:         "claude",
				SupportedModels: []string{"sonnet", "opus"},
				SupportedAgents: []string{"auto-approve", "plan"},
			},
			"opencode": {
				Command:         "opencode",
				SupportedModels: []string{"gpt-4", "sonnet"},
				SupportedAgents: []string{"auto-approve"},
			},
			"codex": {
				Command: "codex",
			},
		},
	}
}

func draftStyles() common.Styles {
	return common.DefaultStyles()
}

func draftTicket(id, title string) *tickets.Ticket {
	return &tickets.Ticket{ID: id, Title: title}
}

func draftWorkspace() *data.Workspace {
	return &data.Workspace{Name: "main", Repo: "/repo", Root: "/repo"}
}

func TestDraftNewStartsAtHarnessSlot(t *testing.T) {
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
}

func TestDraftSelectHarnessAdvancesToModel(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	_, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if d.activeSlot != SlotModel {
		t.Errorf("expected SlotModel after harness confirm, got %d", d.activeSlot)
	}
	if d.harness == "" {
		t.Error("harness should be set")
	}
}

func TestDraftHarnessPrunesAgentOptions(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	d.confirmHarness("opencode")

	if len(d.agentOptions) != 1 || d.agentOptions[0] != "auto-approve" {
		t.Errorf("expected [auto-approve], got %v", d.agentOptions)
	}
}

func TestDraftHarnessPrunesModelOptions(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	d.confirmHarness("opencode")

	if len(d.modelOptions) != 2 {
		t.Errorf("expected 2 model options for opencode, got %d", len(d.modelOptions))
	}
}

func TestDraftHarnessWithEmptyListsShowsDefault(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	d.confirmHarness("codex")

	if len(d.modelOptions) != 1 || d.modelOptions[0] != "default" {
		t.Errorf("expected [default] for harness without SupportedModels, got %v", d.modelOptions)
	}
	if len(d.agentOptions) != 1 || d.agentOptions[0] != "default" {
		t.Errorf("expected [default] for harness without SupportedAgents, got %v", d.agentOptions)
	}
}

func TestDraftCompleteReturnsAllMetadata(t *testing.T) {
	d := NewDraft(draftTicket("bmx-42", "My Ticket"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	d.confirmHarness("claude")
	d.model = "sonnet"
	d.activeSlot = SlotAgent
	d.resetFilter(d.agentOptions)
	d.agent = "auto-approve"
	d.activeSlot = SlotComplete

	cmd := d.launchCmd()
	msg := cmd()

	dc, ok := msg.(DraftComplete)
	if !ok {
		t.Fatalf("expected DraftComplete, got %T", msg)
	}
	if dc.Assistant != "claude" {
		t.Errorf("expected assistant=claude, got %s", dc.Assistant)
	}
	if dc.TicketID != "bmx-42" {
		t.Errorf("expected ticketID=bmx-42, got %s", dc.TicketID)
	}
	if dc.TicketTitle != "My Ticket" {
		t.Errorf("expected ticketTitle=My Ticket, got %s", dc.TicketTitle)
	}
	if dc.Model != "sonnet" {
		t.Errorf("expected model=sonnet, got %s", dc.Model)
	}
	if dc.AgentMode != "auto-approve" {
		t.Errorf("expected agentMode=auto-approve, got %s", dc.AgentMode)
	}
	if dc.Workspace == nil {
		t.Error("workspace should not be nil")
	}
}

func TestDraftCancelOnEscape(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	msg := cmd()
	if _, ok := msg.(DraftCancelled); !ok {
		t.Fatalf("expected DraftCancelled on esc at first slot, got %T", msg)
	}
}

func TestDraftEscapeGoesBack(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	d.confirmHarness("claude")
	if d.activeSlot != SlotModel {
		t.Fatalf("expected SlotModel, got %d", d.activeSlot)
	}

	_, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if d.activeSlot != SlotHarness {
		t.Errorf("expected goBack to SlotHarness, got %d", d.activeSlot)
	}
}

func TestDraftFuzzyFilter(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	_, _ = d.Update(tea.KeyPressMsg{Text: "o"})

	if len(d.filteredIndices) == len(d.harnessOptions) {
		t.Error("filter should narrow results")
	}
	found := false
	for _, idx := range d.filteredIndices {
		if d.harnessOptions[idx] == "opencode" {
			found = true
		}
	}
	if !found {
		t.Error("'o' filter should match 'opencode'")
	}
}

func TestDraftDefaultHarnessPreSelected(t *testing.T) {
	cfg := draftConfig()
	cfg.Defaults = &config.Defaults{Harness: "claude", Model: "sonnet", Agent: "auto-approve"}

	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), cfg, draftStyles())
	d.SetSize(80, 24)

	if d.harness != "claude" {
		t.Errorf("expected harness=claude from defaults, got %s", d.harness)
	}
	if d.model != "sonnet" {
		t.Errorf("expected model=sonnet from defaults, got %s", d.model)
	}
	if d.activeSlot != SlotAgent {
		t.Errorf("expected SlotAgent after defaults auto-filled harness+model, got %d", d.activeSlot)
	}
}

func TestDraftViewContainsStepIndicator(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	view := d.View()
	if !strings.Contains(view, "Step") {
		t.Error("View should contain step indicator")
	}
	if !strings.Contains(view, "bmx-1") {
		t.Error("View should contain ticket ID")
	}
}

func TestTruncateStrHandlesMultibyteRunes(t *testing.T) {
	cases := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 8, "hello..."},
		{"日本語テスト", 4, "日..."},
		{"abc", 2, "ab"},
		{"short", 10, "short"},
	}
	for _, tc := range cases {
		got := truncateStr(tc.input, tc.max)
		if got != tc.want {
			t.Errorf("truncateStr(%q, %d) = %q, want %q", tc.input, tc.max, got, tc.want)
		}
	}

	orig := "日本語テストデータ"
	result := truncateStr(orig, 6)
	if !strings.HasSuffix(result, "...") {
		t.Errorf("expected truncation ellipsis, got %q", result)
	}
	for _, r := range result[:len(result)-3] {
		if r == 0xFFFD {
			t.Error("truncation produced invalid UTF-8 replacement character")
		}
	}
}

func TestDraftResizePropagates(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)
	if d.width != 80 || d.height != 24 {
		t.Fatalf("expected 80x24, got %dx%d", d.width, d.height)
	}

	d.SetSize(100, 30)
	if d.width != 100 || d.height != 30 {
		t.Errorf("expected 100x30 after resize, got %dx%d", d.width, d.height)
	}
}
