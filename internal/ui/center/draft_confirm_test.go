package center

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/tickets"
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

// --- Tests for extracted business logic ---

func TestBuildTemplateContext(t *testing.T) {
	d := draftAtConfirm()

	ctx := d.buildTemplateContext()

	// Verify the context was built from the Draft state.
	if ctx.Assistant != "claude" {
		t.Errorf("expected assistant=claude, got %s", ctx.Assistant)
	}
	if ctx.Model.ModelID() != "sonnet" {
		t.Errorf("expected model=sonnet, got %s", ctx.Model.ModelID())
	}
	if ctx.Agent != "auto-approve" {
		t.Errorf("expected agent=auto-approve, got %s", ctx.Agent)
	}
	if ctx.Ticket.ID != "bmx-42" {
		t.Errorf("expected ticket ID=bmx-42, got %s", ctx.Ticket.ID)
	}
	if ctx.Ticket.Title != "My Ticket" {
		t.Errorf("expected ticket title=My Ticket, got %s", ctx.Ticket.Title)
	}
	if ctx.WorkDir != "/repo" {
		t.Errorf("expected workDir=/repo, got %s", ctx.WorkDir)
	}

	// Templates should come from the shared config.
	if ctx.CommandTemplate != "claude --model {{.Model}} --agent {{.Agent}}" {
		t.Errorf("unexpected command template: %s", ctx.CommandTemplate)
	}
	if ctx.PromptTemplate != "Work on {{.TicketID}}: {{.TicketTitle}}" {
		t.Errorf("unexpected prompt template: %s", ctx.PromptTemplate)
	}
}

func TestBuildTemplateContextOverrides(t *testing.T) {
	d := draftAtConfirm()

	// Set local overrides — they should take precedence.
	d.commandOverride = "custom --cmd {{.Model}}"
	d.promptOverride = "custom prompt {{.TicketID}}"

	ctx := d.buildTemplateContext()

	if ctx.CommandTemplate != "custom --cmd {{.Model}}" {
		t.Errorf("expected command override, got %s", ctx.CommandTemplate)
	}
	if ctx.PromptTemplate != "custom prompt {{.TicketID}}" {
		t.Errorf("expected prompt override, got %s", ctx.PromptTemplate)
	}
}

func TestBuildTemplateContextNoAssistant(t *testing.T) {
	cfg := draftConfig()
	d := NewDraft(draftTicket("bmx-1", "Test"), draftWorkspace(), cfg, draftStyles())
	d.SetSize(80, 24)

	// Simulate a harness that doesn't exist in config.
	d.harness = "nonexistent"
	d.model = "default"
	d.agent = "default"

	ctx := d.buildTemplateContext()

	// Should return zero value when harness has no config entry.
	if ctx.CommandTemplate != "" || ctx.PromptTemplate != "" {
		t.Error("expected empty templates for unknown harness")
	}
}

func TestBuildTemplateContextNoWorkspace(t *testing.T) {
	cfg := draftConfig()
	d := NewDraft(draftTicket("bmx-1", "Test"), nil, cfg, draftStyles())
	d.SetSize(80, 24)
	d.confirmHarness("claude")
	d.model = "sonnet"
	d.agent = "auto-approve"

	ctx := d.buildTemplateContext()

	if ctx.WorkDir != "" {
		t.Errorf("expected empty workDir without workspace, got %s", ctx.WorkDir)
	}
}

func TestBuildTemplateContextNoTicket(t *testing.T) {
	cfg := draftConfig()
	d := NewDraft(nil, draftWorkspace(), cfg, draftStyles())
	d.SetSize(80, 24)
	d.confirmHarness("claude")
	d.model = "sonnet"
	d.agent = "auto-approve"

	ctx := d.buildTemplateContext()

	if ctx.Ticket.ID != "" || ctx.Ticket.Title != "" {
		t.Error("expected empty ticket when nil")
	}
}

func TestComputeTemplatePreview(t *testing.T) {
	d := draftAtConfirm()
	ctx := d.buildTemplateContext()
	renderer := tickets.NewRenderer()

	preview := computeTemplatePreview(renderer, ctx, 24, 4)

	if preview.CommandError != nil {
		t.Fatalf("unexpected command error: %v", preview.CommandError)
	}
	if preview.PromptError != nil {
		t.Fatalf("unexpected prompt error: %v", preview.PromptError)
	}
	if len(preview.CommandLines) == 0 {
		t.Error("expected command lines")
	}
	if len(preview.PromptLines) == 0 {
		t.Error("expected prompt lines")
	}

	// Verify the rendered command content.
	cmdStr := strings.Join(preview.CommandLines, "\n")
	if !strings.Contains(cmdStr, "claude --model sonnet --agent auto-approve") {
		t.Errorf("expected rendered command, got %s", cmdStr)
	}

	// Verify the rendered prompt content.
	promptStr := strings.Join(preview.PromptLines, "\n")
	if !strings.Contains(promptStr, "Work on bmx-42: My Ticket") {
		t.Errorf("expected rendered prompt, got %s", promptStr)
	}
}

func TestComputeTemplatePreviewEmptyTemplates(t *testing.T) {
	renderer := tickets.NewRenderer()
	ctx := tickets.TemplateContext{} // No templates set.

	preview := computeTemplatePreview(renderer, ctx, 24, 4)

	if preview.CommandLines != nil {
		t.Error("expected no command lines for empty template")
	}
	if preview.PromptLines != nil {
		t.Error("expected no prompt lines for empty template")
	}
	if preview.CommandError != nil {
		t.Errorf("expected no error for empty template, got %v", preview.CommandError)
	}
	if preview.PromptError != nil {
		t.Errorf("expected no error for empty template, got %v", preview.PromptError)
	}
}

func TestComputeTemplatePreviewBadTemplate(t *testing.T) {
	renderer := tickets.NewRenderer()
	ctx := tickets.TemplateContext{
		CommandTemplate: "{{.NonexistentField}}",
		Selection: tickets.Selection{
			Assistant: "test",
		},
	}

	preview := computeTemplatePreview(renderer, ctx, 24, 4)

	if preview.CommandError == nil {
		t.Fatal("expected error for bad template")
	}
}

func TestComputeTemplatePreviewLineCapping(t *testing.T) {
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
	d.agent = "auto-approve"

	ctx := d.buildTemplateContext()
	renderer := tickets.NewRenderer()

	preview := computeTemplatePreview(renderer, ctx, 8, 4)

	if preview.CommandError != nil {
		t.Fatalf("unexpected error: %v", preview.CommandError)
	}
	// With height=8 and 4 steps: reservedLines = 4+4+3 = 11, maxTemplateLines = 8-11 = -3 → clamped to 3.
	// So command lines should be capped to 3.
	if len(preview.CommandLines) > 3 {
		t.Errorf("expected at most 3 command lines on small terminal, got %d", len(preview.CommandLines))
	}
	// Last line should contain truncation indicator.
	if len(preview.CommandLines) == 3 {
		if !strings.Contains(preview.CommandLines[2], "more lines") {
			t.Errorf("expected truncation indicator in last line, got %q", preview.CommandLines[2])
		}
	}
}
