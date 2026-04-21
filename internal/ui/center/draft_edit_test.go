package center

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestDraftEKeyEntersInlineEdit(t *testing.T) {
	d := draftAtConfirm()

	_, cmd := d.Update(tea.KeyPressMsg{Text: "e"})

	if !d.inlineEditActive {
		t.Fatal("expected inline edit to be active after 'e' key")
	}
	if d.inlineEditMode != editModePrompt {
		t.Errorf("expected editModePrompt (agent is selected), got %d", d.inlineEditMode)
	}
	if cmd != nil {
		t.Fatal("expected nil cmd from entering inline edit")
	}
}

func TestDraftEKeyEditsCommandWhenNoPrompt(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	// opencode has command_template but no prompt_template
	d.confirmHarness("opencode")
	d.model = "gpt-4"
	d.activeSlot = SlotAgent
	d.resetFilter(d.agentOptions)
	d.agent = "auto-approve"
	d.activeSlot = SlotConfirm

	_, _ = d.Update(tea.KeyPressMsg{Text: "e"})

	if !d.inlineEditActive {
		t.Fatal("expected inline edit to be active")
	}
	if d.inlineEditMode != editModeCommand {
		t.Errorf("expected editModeCommand (no prompt template), got %d", d.inlineEditMode)
	}
}

func TestDraftInlineEditEscCancels(t *testing.T) {
	d := draftAtConfirm()

	// Enter inline edit.
	_, _ = d.Update(tea.KeyPressMsg{Text: "e"})
	if !d.inlineEditActive {
		t.Fatal("inline edit should be active")
	}

	// Cancel with Esc.
	_, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if d.inlineEditActive {
		t.Error("inline edit should be canceled after Esc")
	}
	if d.Dirty() {
		t.Error("canceled edit should not mark draft as dirty")
	}
}

func TestDraftInlineEditCtrlYAccepts(t *testing.T) {
	d := draftAtConfirm()

	// Enter inline edit.
	_, _ = d.Update(tea.KeyPressMsg{Text: "e"})

	// Accept with Ctrl-Y.
	_, cmd := d.Update(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})

	if d.inlineEditActive {
		t.Error("inline edit should be closed after Ctrl-Y")
	}
	if !d.Dirty() {
		t.Error("accepted edit should mark draft as dirty")
	}
	if cmd != nil {
		t.Fatal("expected nil cmd from accepting inline edit")
	}
}

func TestDraftInlineEditInvalidTemplate(t *testing.T) {
	d := draftAtConfirm()

	// Enter inline edit.
	_, _ = d.Update(tea.KeyPressMsg{Text: "e"})

	// Clear the textarea and type an invalid template.
	d.inlineEditTA.SetValue("{{.BadField")

	// Try to accept.
	_, _ = d.Update(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})

	if !d.inlineEditActive {
		t.Error("inline edit should stay active on validation error")
	}
	if d.inlineEditError == "" {
		t.Error("expected validation error message")
	}
	if d.Dirty() {
		t.Error("failed edit should not mark draft as dirty")
	}
}

func TestDraftInlineEditView(t *testing.T) {
	d := draftAtConfirm()

	_, _ = d.Update(tea.KeyPressMsg{Text: "e"})

	view := d.View()
	if !strings.Contains(view, "Edit Prompt Template") {
		t.Errorf("inline edit view should show prompt template title, got:\n%s", view)
	}
	if !strings.Contains(view, "ctrl+y=accept") {
		t.Error("inline edit view should show keybinding hints")
	}
}

func TestDraftInlineEditCommandTitle(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	d.confirmHarness("opencode")
	d.model = "gpt-4"
	d.activeSlot = SlotAgent
	d.resetFilter(d.agentOptions)
	d.agent = "auto-approve"
	d.activeSlot = SlotConfirm

	_, _ = d.Update(tea.KeyPressMsg{Text: "e"})

	view := d.View()
	if !strings.Contains(view, "Edit Command Template") {
		t.Errorf("inline edit view should show command template title, got:\n%s", view)
	}
}

func TestDraftEditDoesNotMutateSharedConfig(t *testing.T) {
	cfg := draftConfig()
	originalPrompt := cfg.Assistants["claude"].PromptTemplate

	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), cfg, draftStyles())
	d.SetSize(80, 24)
	d.confirmHarness("claude")
	d.model = "sonnet"
	d.activeSlot = SlotAgent
	d.resetFilter(d.agentOptions)
	d.agent = "auto-approve"
	d.activeSlot = SlotConfirm

	// Edit prompt template.
	_, _ = d.Update(tea.KeyPressMsg{Text: "e"})
	d.inlineEditTA.SetValue("modified {{.TicketID}}")
	_, _ = d.Update(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})

	if !d.Dirty() {
		t.Fatal("draft should be dirty after edit")
	}

	// Verify the override was stored locally, not on the shared config.
	if d.promptOverride != "modified {{.TicketID}}" {
		t.Errorf("expected promptOverride to be set, got %q", d.promptOverride)
	}
	if cfg.Assistants["claude"].PromptTemplate != originalPrompt {
		t.Error("shared config should NOT be mutated — mutation leak detected")
	}
}

func TestDraftEditCancelDiscardsOverrides(t *testing.T) {
	cfg := draftConfig()
	originalPrompt := cfg.Assistants["claude"].PromptTemplate

	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), cfg, draftStyles())
	d.SetSize(80, 24)
	d.confirmHarness("claude")
	d.model = "sonnet"
	d.activeSlot = SlotAgent
	d.resetFilter(d.agentOptions)
	d.agent = "auto-approve"
	d.activeSlot = SlotConfirm

	// Edit prompt template.
	_, _ = d.Update(tea.KeyPressMsg{Text: "e"})
	d.inlineEditTA.SetValue("modified {{.TicketID}}")

	// Cancel instead of accepting.
	_, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if d.Dirty() {
		t.Error("canceled edit should not mark draft as dirty")
	}
	if d.promptOverride != "" {
		t.Error("canceled edit should not set promptOverride")
	}
	if cfg.Assistants["claude"].PromptTemplate != originalPrompt {
		t.Error("shared config should NOT be mutated after cancel")
	}
}

func TestDraftEditOverrideShowsInConfirmView(t *testing.T) {
	d := draftAtConfirm()

	// Edit prompt template and accept.
	_, _ = d.Update(tea.KeyPressMsg{Text: "e"})
	d.inlineEditTA.SetValue("custom {{.TicketID}} prompt")
	_, _ = d.Update(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})

	view := d.View()
	if !strings.Contains(view, "custom bmx-42 prompt") {
		t.Errorf("confirm view should show overridden template, got:\n%s", view)
	}
}

func TestDraftEditReEditShowsOverride(t *testing.T) {
	d := draftAtConfirm()

	// First edit.
	_, _ = d.Update(tea.KeyPressMsg{Text: "e"})
	d.inlineEditTA.SetValue("first edit {{.TicketID}}")
	_, _ = d.Update(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})

	// Re-enter edit — should show the override, not the original config.
	_, _ = d.Update(tea.KeyPressMsg{Text: "e"})
	if d.inlineEditTA.Value() != "first edit {{.TicketID}}" {
		t.Errorf("re-editing should show override, got %q", d.inlineEditTA.Value())
	}
}
