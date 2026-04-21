package center

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// draftAtConfirm creates a Draft that has progressed to SlotConfirm.
func draftAtConfirm() *Draft {
	d := NewDraft(draftTicket("bmx-42", "My Ticket"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)
	d.confirmHarness("claude")
	d.model = "sonnet"
	d.activeSlot = SlotAgent
	d.resetFilter(d.agentOptions)
	d.agent = "auto-approve"
	d.activeSlot = SlotConfirm
	return d
}

func TestDraftAgentSelectionGoesToConfirm(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	d.confirmHarness("claude")
	d.model = "sonnet"
	d.activeSlot = SlotAgent
	d.resetFilter(d.agentOptions)

	// Select first agent option.
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if d.activeSlot != SlotConfirm {
		t.Fatalf("expected SlotConfirm after agent selection, got %d", d.activeSlot)
	}
	if cmd != nil {
		t.Fatal("expected no auto-launch cmd at confirm step")
	}
}

func TestDraftConfirmEnterLaunches(t *testing.T) {
	d := draftAtConfirm()

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if d.activeSlot != SlotComplete {
		t.Fatalf("expected SlotComplete after confirm enter, got %d", d.activeSlot)
	}
	if cmd == nil {
		t.Fatal("expected launch command after confirm enter")
	}

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
	if dc.Model != "sonnet" {
		t.Errorf("expected model=sonnet, got %s", dc.Model)
	}
	if dc.AgentMode != "auto-approve" {
		t.Errorf("expected agentMode=auto-approve, got %s", dc.AgentMode)
	}
}

func TestDraftConfirmEscGoesBackToAgent(t *testing.T) {
	d := draftAtConfirm()

	_, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if d.activeSlot != SlotAgent {
		t.Fatalf("expected SlotAgent after esc from confirm, got %d", d.activeSlot)
	}
}

func TestDraftConfirmViewContainsLaunchButton(t *testing.T) {
	d := draftAtConfirm()

	view := d.View()
	if !strings.Contains(view, "LAUNCH") {
		t.Error("confirm view should contain LAUNCH button")
	}
	if !strings.Contains(view, "bmx-42") {
		t.Error("confirm view should contain ticket ID")
	}
	if !strings.Contains(view, "claude") {
		t.Error("confirm view should contain harness name")
	}
	if !strings.Contains(view, "sonnet") {
		t.Error("confirm view should contain model name")
	}
}

func TestDraftConfirmViewShowsRenderedCommand(t *testing.T) {
	d := draftAtConfirm()

	view := d.View()
	// The claude config has command_template: "claude --model {{.Model}} --agent {{.Agent}}"
	if !strings.Contains(view, "Command:") {
		t.Error("confirm view should show Command label")
	}
	if !strings.Contains(view, "claude --model sonnet --agent auto-approve") {
		t.Error("confirm view should show rendered command")
	}
}

func TestDraftConfirmViewShowsRenderedPrompt(t *testing.T) {
	d := draftAtConfirm()

	view := d.View()
	// The claude config has prompt_template: "Work on {{.TicketID}}: {{.TicketTitle}}"
	if !strings.Contains(view, "Prompt:") {
		t.Error("confirm view should show Prompt label")
	}
	if !strings.Contains(view, "Work on bmx-42: My Ticket") {
		t.Error("confirm view should show rendered prompt")
	}
}

func TestDraftConfirmViewNoTemplateConfigured(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	// codex has no command_template or prompt_template
	d.confirmHarness("codex")
	d.model = "default"
	d.activeSlot = SlotAgent
	d.resetFilter(d.agentOptions)
	d.agent = "default"
	d.activeSlot = SlotConfirm

	view := d.View()
	if !strings.Contains(view, "Confirm Launch") {
		t.Error("confirm view should show header")
	}
}

func TestDraftConfirmViewDirtyIndicator(t *testing.T) {
	d := draftAtConfirm()

	if d.Dirty() {
		t.Fatal("new draft should not be dirty")
	}

	view := d.View()
	if strings.Contains(view, "Confirm Launch *") {
		t.Error("clean draft should not show dirty indicator")
	}

	// Simulate dirty state.
	d.dirty = true
	view = d.View()
	if !strings.Contains(view, "Confirm Launch *") {
		t.Error("dirty draft should show * indicator")
	}
}

func TestDraftConfirmViewHintsHideCWithoutWorkspace(t *testing.T) {
	cfg := draftConfig()
	d := NewDraft(draftTicket("bmx-1", "Test"), nil, cfg, draftStyles())
	d.SetSize(80, 24)
	d.confirmHarness("claude")
	d.model = "sonnet"
	d.activeSlot = SlotAgent
	d.resetFilter(d.agentOptions)
	d.agent = "auto-approve"
	d.activeSlot = SlotConfirm

	view := d.View()
	if strings.Contains(view, "C:template file") {
		t.Error("confirm view should NOT show C:template file hint when no workspace")
	}
	if !strings.Contains(view, "enter:launch") {
		t.Error("confirm view should still show enter:launch hint")
	}
}

func TestDraftConfirmCKeyNoOpWithoutWorkspace(t *testing.T) {
	cfg := draftConfig()
	d := NewDraft(draftTicket("bmx-1", "Test"), nil, cfg, draftStyles())
	d.SetSize(80, 24)
	d.confirmHarness("claude")
	d.model = "sonnet"
	d.activeSlot = SlotAgent
	d.resetFilter(d.agentOptions)
	d.agent = "auto-approve"
	d.activeSlot = SlotConfirm

	_, _ = d.Update(tea.KeyPressMsg{Text: "C"})

	if d.filePickerActive {
		t.Error("C key should be no-op without workspace")
	}
}

func TestDraftConfirmCKeyOpensWithWorkspace(t *testing.T) {
	d := draftAtConfirm()

	_, _ = d.Update(tea.KeyPressMsg{Text: "C"})

	if !d.filePickerActive {
		t.Error("C key should open file picker with workspace")
	}
}

func TestCapLines(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}

	// No truncation needed.
	result := capLines(lines, 5)
	if len(result) != 5 {
		t.Errorf("expected 5 lines, got %d", len(result))
	}

	// Truncation needed.
	result = capLines(lines, 3)
	if len(result) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(result))
	}
	if result[0] != "a" || result[1] != "b" {
		t.Errorf("first 2 lines should be preserved, got %v", result[:2])
	}
	if !strings.Contains(result[2], "3 more lines") {
		t.Errorf("last line should be truncation indicator, got %q", result[2])
	}
}

func TestDraftConfirmViewCapsOnSmallTerminal(t *testing.T) {
	cfg := draftConfig()
	// Create a config with a very long command template.
	longCmd := strings.Repeat("line {{.TicketID}}\n", 50)
	ac := cfg.Assistants["claude"]
	ac.CommandTemplate = longCmd
	cfg.Assistants["claude"] = ac

	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), cfg, draftStyles())
	d.SetSize(80, 8) // Very small terminal.
	d.confirmHarness("claude")
	d.model = "sonnet"
	d.activeSlot = SlotAgent
	d.resetFilter(d.agentOptions)
	d.agent = "auto-approve"
	d.activeSlot = SlotConfirm

	view := d.View()
	// Should contain truncation indicator instead of all 50 lines.
	if !strings.Contains(view, "more lines") {
		t.Errorf("small terminal should show truncation indicator, got:\n%s", view)
	}
}
