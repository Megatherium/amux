package center

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/config"
)

func TestDraftLoadTemplateFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	content := "echo hello {{.TicketID}}"
	tmpFile := tmpDir + "/test.tmpl"
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	d := draftAtConfirm()

	cmd := d.loadTemplateFromFile(tmpFile)
	msg := cmd()

	loaded, ok := msg.(draftTemplateLoadedMsg)
	if !ok {
		t.Fatalf("expected draftTemplateLoadedMsg, got %T", msg)
	}
	if loaded.content != content {
		t.Errorf("expected content %q, got %q", content, loaded.content)
	}
}

func TestDraftLoadTemplateFromBinaryFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.bin"
	binaryContent := []byte{0x00, 0xFF, 0x00, 0xFF}
	if err := os.WriteFile(tmpFile, binaryContent, 0o644); err != nil {
		t.Fatal(err)
	}

	d := draftAtConfirm()

	cmd := d.loadTemplateFromFile(tmpFile)
	msg := cmd()

	errMsg, ok := msg.(draftTemplateErrorMsg)
	if !ok {
		t.Fatalf("expected draftTemplateErrorMsg for binary file, got %T", msg)
	}
	if !strings.Contains(errMsg.err.Error(), "binary") {
		t.Errorf("expected binary data error, got: %v", errMsg.err)
	}
}

func TestDraftTemplateLoadedOpensInlineEdit(t *testing.T) {
	d := draftAtConfirm()

	_, _ = d.Update(draftTemplateLoadedMsg{content: "echo {{.TicketID}}"})

	if !d.inlineEditActive {
		t.Fatal("template loaded should open inline editor")
	}
	// With agent selected, it should edit prompt template.
	if d.inlineEditMode != editModePrompt {
		t.Errorf("expected editModePrompt, got %d", d.inlineEditMode)
	}
	if d.inlineEditTA.Value() != "echo {{.TicketID}}" {
		t.Errorf("textarea should contain loaded content, got %q", d.inlineEditTA.Value())
	}
}

func TestDraftTemplateLoadedNoAgentEditsCommand(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	d.confirmHarness("opencode")
	d.model = "gpt-4"
	d.activeSlot = SlotAgent
	d.resetFilter(d.agentOptions)
	d.agent = ""
	d.activeSlot = SlotConfirm

	_, _ = d.Update(draftTemplateLoadedMsg{content: "custom cmd {{.TicketID}}"})

	if !d.inlineEditActive {
		t.Fatal("template loaded should open inline editor")
	}
	if d.inlineEditMode != editModeCommand {
		t.Errorf("expected editModeCommand when no agent, got %d", d.inlineEditMode)
	}
}

// --- Full Flow Integration Tests ---

func TestDraftFullFlowThroughConfirm(t *testing.T) {
	d := NewDraft(draftTicket("bmx-99", "Integration Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	// Step 1: Select harness (claude is first).
	_, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if d.activeSlot != SlotModel {
		t.Fatalf("step 1: expected SlotModel, got %d", d.activeSlot)
	}

	// Step 2: Select model (sonnet is first).
	_, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if d.activeSlot != SlotAgent {
		t.Fatalf("step 2: expected SlotAgent, got %d", d.activeSlot)
	}

	// Step 3: Select agent (auto-approve is first).
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if d.activeSlot != SlotConfirm {
		t.Fatalf("step 3: expected SlotConfirm, got %d", d.activeSlot)
	}
	if cmd != nil {
		t.Fatal("step 3: should NOT auto-launch")
	}

	// Verify confirm view content.
	view := d.View()
	if !strings.Contains(view, "Confirm Launch") {
		t.Error("step 3: confirm view should show header")
	}
	if !strings.Contains(view, "bmx-99") {
		t.Error("step 3: confirm view should show ticket ID")
	}

	// Step 4: Confirm launch.
	_, cmd = d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if d.activeSlot != SlotComplete {
		t.Fatalf("step 4: expected SlotComplete, got %d", d.activeSlot)
	}

	msg := cmd()
	dc, ok := msg.(DraftComplete)
	if !ok {
		t.Fatalf("step 4: expected DraftComplete, got %T", msg)
	}
	if dc.Assistant != "claude" {
		t.Errorf("step 4: expected assistant=claude, got %s", dc.Assistant)
	}
	if dc.TicketID != "bmx-99" {
		t.Errorf("step 4: expected ticketID=bmx-99, got %s", dc.TicketID)
	}
}

func TestDraftFullFlowWithEdit(t *testing.T) {
	d := draftAtConfirm()

	// Edit prompt template.
	_, _ = d.Update(tea.KeyPressMsg{Text: "e"})
	if !d.inlineEditActive {
		t.Fatal("inline edit should be active")
	}

	// Accept edit.
	_, _ = d.Update(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})
	if d.inlineEditActive {
		t.Fatal("inline edit should be closed")
	}
	if !d.Dirty() {
		t.Fatal("draft should be dirty after edit")
	}

	// Confirm view should show dirty indicator.
	view := d.View()
	if !strings.Contains(view, "Confirm Launch *") {
		t.Error("confirm view should show dirty indicator")
	}

	// Launch.
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	if _, ok := msg.(DraftComplete); !ok {
		t.Fatalf("expected DraftComplete, got %T", msg)
	}
}

func TestDraftDefaultsAdvanceToConfirm(t *testing.T) {
	cfg := draftConfig()
	cfg.Defaults = &config.Defaults{Harness: "claude", Model: "sonnet", Agent: "auto-approve"}

	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), cfg, draftStyles())
	d.SetSize(80, 24)

	// After NewDraft, harness+model are auto-filled. Agent slot is active.
	if d.activeSlot != SlotAgent {
		t.Fatalf("expected SlotAgent, got %d", d.activeSlot)
	}

	// Select agent.
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if d.activeSlot != SlotConfirm {
		t.Fatalf("expected SlotConfirm after agent selection, got %d", d.activeSlot)
	}
	if cmd != nil {
		t.Fatal("should not auto-launch even with defaults")
	}
}
