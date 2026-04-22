package center

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/andyrewlee/amux/internal/tickets"
	"github.com/andyrewlee/amux/internal/ui/common"
)

// handleConfirmKey handles key presses in the confirm slot.
func (d *Draft) handleConfirmKey(msg tea.KeyPressMsg) (*Draft, tea.Cmd) {
	switch {
	case msg.String() == "enter":
		d.activeSlot = SlotComplete
		return d, d.launchCmd()

	case msg.String() == "esc":
		d.goBack()
		return d, nil

	case msg.String() == "e":
		return d, d.enterInlineEdit()

	case msg.String() == "C":
		// Only allow file picker when we have a workspace root to browse.
		if d.workspace != nil && d.workspace.Root != "" {
			return d, d.enterFilePicker()
		}
	}
	return d, nil
}

// renderConfirmView renders the confirmation screen showing selection summary
// and rendered command/prompt templates.
func (d *Draft) renderConfirmView(stepLabels []string) string {
	var b strings.Builder

	title := "Confirm Launch"
	if d.dirty {
		title += " *"
	}

	headerStyle := d.styles.Title
	b.WriteString(headerStyle.Render(title))
	b.WriteString("\n\n")

	// Render collapsed selection summary.
	for i, label := range stepLabels {
		slot := DraftSlot(i)
		value := d.slotValue(slot)
		if slot == SlotTicket && d.ticket != nil {
			value = d.ticket.ID + " — " + truncateStr(d.ticket.Title, 40)
		}
		checkStyle := lipgloss.NewStyle().Foreground(common.ColorSuccess())
		mutedStyle := lipgloss.NewStyle().Foreground(common.ColorMuted())
		b.WriteString(checkStyle.Render("  ✓ ") + mutedStyle.Render(label+": "+value))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Render command and prompt templates.
	d.renderTemplates(&b, len(stepLabels))

	// Launch button.
	b.WriteString("\n")
	launchStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.ColorBackground()).
		Background(common.ColorSuccess()).
		Padding(0, 4).
		Width(20).
		Align(lipgloss.Center)
	b.WriteString(launchStyle.Render("LAUNCH"))
	b.WriteString("\n")

	hintStyle := lipgloss.NewStyle().Foreground(common.ColorMuted())
	hints := "enter:launch  e:edit  esc:back"
	if d.workspace != nil && d.workspace.Root != "" {
		hints = "enter:launch  e:edit  C:template file  esc:back"
	}
	b.WriteString(hintStyle.Render(hints))
	b.WriteString("\n")

	return b.String()
}

// buildTemplateContext constructs a TemplateContext from the current Draft
// state, resolving local overrides (commandOverride, promptOverride) over the
// shared assistant config. Returns the zero value if the harness has no config
// entry.
func (d *Draft) buildTemplateContext() tickets.TemplateContext {
	assistantCfg, ok := d.config.Assistants[d.harness]
	if !ok {
		return tickets.TemplateContext{}
	}

	workDir := ""
	if d.workspace != nil {
		workDir = d.workspace.Root
	}

	var ticket tickets.Ticket
	if d.ticket != nil {
		ticket = *d.ticket
	}
	sel := tickets.Selection{
		Ticket:    ticket,
		Assistant: d.harness,
		Model:     d.model,
		Agent:     d.agent,
	}

	ctx := tickets.BuildTemplateContext(sel, workDir)
	// Use local overrides when present, falling back to the shared config.
	ctx.CommandTemplate = assistantCfg.CommandTemplate
	if d.commandOverride != "" {
		ctx.CommandTemplate = d.commandOverride
	}
	ctx.PromptTemplate = assistantCfg.PromptTemplate
	if d.promptOverride != "" {
		ctx.PromptTemplate = d.promptOverride
	}

	return ctx
}

// templatePreview holds the structured result of rendering command and prompt
// templates, ready for display without further computation.
type templatePreview struct {
	// NoConfig is true when the harness has no assistant config entry.
	NoConfig bool
	// HarnessName is the harness name, used in the "no config" message.
	HarnessName string

	// CommandLines holds the capped rendered command lines (nil if no command).
	CommandLines []string
	// CommandError is the command rendering error, if any.
	CommandError error

	// PromptLines holds the capped rendered prompt lines (nil if no prompt).
	PromptLines []string
	// PromptError is the prompt rendering error, if any.
	PromptError error
}

// computeTemplatePreview renders the command and prompt templates from ctx,
// applying line budget caps for the given terminal height and step count.
// Returns a templatePreview with all data needed for display.
func computeTemplatePreview(renderer *tickets.Renderer, ctx tickets.TemplateContext, height, numSteps int) templatePreview {
	// Reserve lines for header, summary, launch button, and hints.
	reservedLines := 4 + numSteps + 3 // header + summary + launch + hints
	maxTemplateLines := height - reservedLines
	if maxTemplateLines < 3 {
		maxTemplateLines = 3
	}

	var preview templatePreview
	linesUsed := 0

	// Render and cap command.
	if ctx.CommandTemplate != "" {
		renderedCmd, err := renderer.RenderCommand(ctx)
		if err != nil {
			preview.CommandError = err
		} else {
			lines := strings.Split(renderedCmd, "\n")
			lines = capLines(lines, maxTemplateLines-linesUsed)
			linesUsed += len(lines)
			preview.CommandLines = lines
		}
	}

	// Render and cap prompt.
	if ctx.PromptTemplate != "" {
		renderedPrompt, err := renderer.RenderPrompt(ctx)
		if err != nil {
			preview.PromptError = err
		} else if renderedPrompt != "" {
			lines := strings.Split(renderedPrompt, "\n")
			remaining := maxTemplateLines - linesUsed
			if remaining < 1 {
				remaining = 1
			}
			lines = capLines(lines, remaining)
			preview.PromptLines = lines
		}
	}

	return preview
}

// renderTemplates renders the command and prompt templates into the builder.
// numSteps is the count of selection summary lines to account for when
// capping output height on small terminals.
func (d *Draft) renderTemplates(b *strings.Builder, numSteps int) {
	if d.renderer == nil {
		return
	}

	ctx := d.buildTemplateContext()

	// If the harness has no config, show a message and stop.
	_, ok := d.config.Assistants[d.harness]
	if !ok {
		mutedStyle := lipgloss.NewStyle().Foreground(common.ColorMuted())
		b.WriteString(mutedStyle.Render("No template configured for " + d.harness))
		b.WriteString("\n")
		return
	}

	preview := computeTemplatePreview(d.renderer, ctx, d.height, numSteps)

	labelStyle := d.styles.Title
	valueStyle := lipgloss.NewStyle().Foreground(common.ColorMuted()).MarginLeft(2)

	// Display command.
	if preview.CommandError != nil {
		errStyle := lipgloss.NewStyle().Foreground(common.ColorError())
		b.WriteString(errStyle.Render("Command error: " + preview.CommandError.Error()))
		b.WriteString("\n")
	} else if preview.CommandLines != nil {
		b.WriteString(labelStyle.Render("Command:"))
		b.WriteString("\n")
		for _, line := range preview.CommandLines {
			b.WriteString(valueStyle.Render(line))
			b.WriteString("\n")
		}
	}

	// Display prompt.
	if preview.PromptError != nil {
		errStyle := lipgloss.NewStyle().Foreground(common.ColorError())
		b.WriteString(errStyle.Render("Prompt error: " + preview.PromptError.Error()))
		b.WriteString("\n")
	} else if preview.PromptLines != nil {
		b.WriteString(labelStyle.Render("Prompt:"))
		b.WriteString("\n")
		for _, line := range preview.PromptLines {
			b.WriteString(valueStyle.Render(line))
			b.WriteString("\n")
		}
	}
}

// capLines truncates a slice of lines to at most max lines, appending a
// truncation indicator when lines are dropped.
func capLines(lines []string, maxLines int) []string {
	if len(lines) <= maxLines {
		return lines
	}
	result := make([]string, maxLines)
	copy(result, lines[:maxLines-1])
	result[maxLines-1] = lipgloss.NewStyle().
		Foreground(common.ColorMuted()).
		Italic(true).
		Render(fmt.Sprintf("... (%d more lines)", len(lines)-maxLines+1))
	return result
}
