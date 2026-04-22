package center

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/config"
)

// --- Tests for extracted state machine helpers ---

func TestSetHarnessPopulatesOptionsAndAdvances(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	d.setHarness("claude")

	if d.harness != "claude" {
		t.Errorf("expected harness=claude, got %s", d.harness)
	}
	if d.activeSlot != SlotModel {
		t.Errorf("expected SlotModel after setHarness, got %d", d.activeSlot)
	}
	if len(d.modelOptions) != 2 {
		t.Errorf("expected 2 model options, got %d", len(d.modelOptions))
	}
	if len(d.agentOptions) != 2 {
		t.Errorf("expected 2 agent options, got %d", len(d.agentOptions))
	}
	if d.model != "" || d.agent != "" {
		t.Error("setHarness should clear model and agent")
	}
}

func TestSetHarnessWithUnknownAssistant(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	d.setHarness("unknown")

	if d.modelOptions[0] != "default" || d.agentOptions[0] != "default" {
		t.Errorf("expected [default] options for unknown harness, got %v/%v", d.modelOptions, d.agentOptions)
	}
}

func TestSelectSlotOptionOutOfRange(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	if d.selectSlotOption(999) {
		t.Error("expected false for out-of-range idx")
	}
}

func TestSelectSlotOptionAdvancesHarness(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)

	if !d.selectSlotOption(0) {
		t.Fatal("expected true for valid idx")
	}
	if d.activeSlot != SlotModel {
		t.Errorf("expected SlotModel, got %d", d.activeSlot)
	}
}

func TestSelectSlotOptionAdvancesModel(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)
	d.setHarness("claude")

	if !d.selectSlotOption(0) {
		t.Fatal("expected true for valid model idx")
	}
	if d.model != "sonnet" {
		t.Errorf("expected model=sonnet, got %s", d.model)
	}
	if d.activeSlot != SlotAgent {
		t.Errorf("expected SlotAgent, got %d", d.activeSlot)
	}
}

func TestSelectSlotOptionAdvancesAgent(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)
	d.setHarness("claude")
	d.model = "sonnet"
	d.activeSlot = SlotAgent
	d.resetFilter(d.agentOptions)

	if !d.selectSlotOption(0) {
		t.Fatal("expected true for valid agent idx")
	}
	if d.agent != "auto-approve" {
		t.Errorf("expected agent=auto-approve, got %s", d.agent)
	}
	if d.activeSlot != SlotConfirm {
		t.Errorf("expected SlotConfirm, got %d", d.activeSlot)
	}
}

func TestApplyDefaultsNilDefaults(t *testing.T) {
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), draftConfig(), draftStyles())
	d.SetSize(80, 24)
	d.setHarness("claude")

	slotBefore := d.activeSlot
	d.applyDefaults()
	if d.activeSlot != slotBefore {
		t.Error("applyDefaults with nil config.Defaults should be a no-op")
	}
}

func TestApplyDefaultsModelOnly(t *testing.T) {
	cfg := draftConfig()
	cfg.Defaults = &config.Defaults{Model: "sonnet"}

	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), cfg, draftStyles())
	d.SetSize(80, 24)
	d.setHarness("claude")

	d.applyDefaults()

	if d.model != "sonnet" {
		t.Errorf("expected model=sonnet, got %s", d.model)
	}
	if d.activeSlot != SlotAgent {
		t.Errorf("expected SlotAgent after model default, got %d", d.activeSlot)
	}
}

func TestApplyDefaultsModelAndAgent(t *testing.T) {
	cfg := draftConfig()
	cfg.Defaults = &config.Defaults{Model: "sonnet", Agent: "auto-approve"}

	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), cfg, draftStyles())
	d.SetSize(80, 24)
	d.setHarness("claude")

	d.applyDefaults()

	if d.model != "sonnet" {
		t.Errorf("expected model=sonnet, got %s", d.model)
	}
	if d.agent != "auto-approve" {
		t.Errorf("expected agent=auto-approve, got %s", d.agent)
	}
	if d.activeSlot != SlotConfirm {
		t.Errorf("expected SlotConfirm after both defaults, got %d", d.activeSlot)
	}
}

func TestApplyDefaultsNonMatching(t *testing.T) {
	cfg := draftConfig()
	cfg.Defaults = &config.Defaults{Model: "nonexistent"}

	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), cfg, draftStyles())
	d.SetSize(80, 24)
	d.setHarness("claude")

	d.applyDefaults()

	if d.model != "" {
		t.Error("non-matching default should not set model")
	}
	if d.activeSlot != SlotModel {
		t.Errorf("expected SlotModel (no advancement), got %d", d.activeSlot)
	}
}

func TestApplyModelDefaultNoCascade(t *testing.T) {
	cfg := draftConfig()
	cfg.Defaults = &config.Defaults{Model: "sonnet", Agent: "auto-approve"}

	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), cfg, draftStyles())
	d.SetSize(80, 24)
	d.setHarness("claude")

	d.applyModelDefault()

	if d.model != "sonnet" {
		t.Errorf("expected model=sonnet, got %s", d.model)
	}
	if d.agent != "" {
		t.Error("applyModelDefault should NOT fill agent")
	}
	if d.activeSlot != SlotAgent {
		t.Errorf("expected SlotAgent, got %d", d.activeSlot)
	}
}

// Regression: selecting a harness with full defaults should NOT auto-fill agent.
func TestConfirmSelectionHarnessDoesNotAutoFillAgent(t *testing.T) {
	cfg := draftConfig()
	cfg.Defaults = &config.Defaults{Harness: "claude", Model: "sonnet", Agent: "auto-approve"}

	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), cfg, draftStyles())
	d.SetSize(80, 24)

	// Defaults auto-filled harness+model in NewDraft, so we're at SlotAgent.
	// Go back to harness to test the confirmSelection path.
	d.activeSlot = SlotHarness
	d.resetFilter(d.harnessOptions)

	// Select harness via confirmSelection (simulating user pressing Enter).
	_, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if d.activeSlot != SlotAgent {
		t.Errorf("expected SlotAgent after harness selection with defaults, got %d — agent should NOT auto-fill", d.activeSlot)
	}
	if d.agent != "" {
		t.Error("agent should not be auto-filled when selecting harness")
	}
}

func TestStringOrDefault(t *testing.T) {
	cases := []struct {
		input []string
		want  []string
	}{
		{nil, []string{"default"}},
		{[]string{}, []string{"default"}},
		{[]string{"a", "b"}, []string{"a", "b"}},
	}
	for _, tc := range cases {
		got := stringOrDefault(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("stringOrDefault(%v): expected %v, got %v", tc.input, tc.want, got)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("stringOrDefault(%v): expected %v, got %v", tc.input, tc.want, got)
			}
		}
	}

	// Verify it returns a copy, not the original slice.
	original := []string{"a", "b"}
	result := stringOrDefault(original)
	result[0] = "modified"
	if original[0] != "a" {
		t.Error("stringOrDefault should return a copy, not alias the input")
	}
}
