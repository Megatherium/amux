package center

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/andyrewlee/amux/internal/tickets"
	"github.com/andyrewlee/amux/internal/ui/common"
)

// newTextarea creates a pre-configured textarea for template editing.
func newTextarea(content string, width, height int) textarea.Model {
	ta := textarea.New()
	ta.SetValue(content)
	ta.SetWidth(min(60, max(20, width-10)))
	ta.SetHeight(min(10, max(3, height/3)))
	return ta
}

// enterInlineEdit creates the inline editor for the current template.
func (d *Draft) enterInlineEdit() tea.Cmd {
	assistantCfg, ok := d.config.Assistants[d.harness]
	if !ok {
		return nil
	}

	// If agent is selected, edit prompt template; otherwise edit command template.
	var content string
	var mode editMode
	if d.agent != "" && (d.promptOverride != "" || assistantCfg.PromptTemplate != "") {
		content = d.promptOverride
		if content == "" {
			content = assistantCfg.PromptTemplate
		}
		mode = editModePrompt
	} else {
		content = d.commandOverride
		if content == "" {
			content = assistantCfg.CommandTemplate
		}
		mode = editModeCommand
	}

	d.inlineEditMode = mode
	d.inlineEditError = ""

	ta := newTextarea(content, d.width, d.height)
	ta.Focus()

	d.inlineEditTA = ta
	d.inlineEditActive = true

	return nil
}

// handleInlineEditMsg routes messages through the inline editor.
func (d *Draft) handleInlineEditMsg(msg tea.Msg) (*Draft, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyPressMsg)
	if !isKey {
		return d, nil
	}

	switch {
	case keyMsg.String() == "esc":
		d.inlineEditActive = false
		d.inlineEditTA.Blur()
		d.inlineEditError = ""
		return d, nil

	case keyMsg.String() == "ctrl+y":
		return d, d.acceptInlineEdit()
	}

	// Forward all other keys to the textarea.
	newTA, cmd := d.inlineEditTA.Update(msg)
	d.inlineEditTA = newTA
	return d, cmd
}

// acceptInlineEdit validates and applies the inline edit.
func (d *Draft) acceptInlineEdit() tea.Cmd {
	content := d.inlineEditTA.Value()
	d.inlineEditTA.Blur()

	// Validate by dry-running the template.
	var ticket tickets.Ticket
	if d.ticket != nil {
		ticket = *d.ticket
	}
	workDir := ""
	if d.workspace != nil {
		workDir = d.workspace.Root
	}
	sel := tickets.Selection{
		Ticket:    ticket,
		Assistant: d.harness,
		Model:     d.model,
		Agent:     d.agent,
	}
	ctx := tickets.BuildTemplateContext(sel, workDir)

	if d.inlineEditMode == editModePrompt {
		ctx.PromptTemplate = content
		if _, err := d.renderer.RenderPrompt(ctx); err != nil {
			d.inlineEditError = err.Error()
			d.inlineEditTA.Focus()
			return nil
		}
	} else {
		ctx.CommandTemplate = content
		if _, err := d.renderer.RenderCommand(ctx); err != nil {
			d.inlineEditError = err.Error()
			d.inlineEditTA.Focus()
			return nil
		}
	}

	// Apply to local override fields — never mutate the shared config.
	if d.inlineEditMode == editModePrompt {
		d.promptOverride = content
	} else {
		d.commandOverride = content
	}

	d.inlineEditActive = false
	d.inlineEditError = ""
	d.dirty = true
	return nil
}

// renderInlineEdit renders the inline template editor overlay.
func (d *Draft) renderInlineEdit() string {
	var b strings.Builder

	titleText := "Edit Command Template"
	if d.inlineEditMode == editModePrompt {
		titleText = "Edit Prompt Template"
	}

	b.WriteString("\n")
	b.WriteString(d.styles.Title.Render(titleText))
	b.WriteString("\n\n")

	hintStyle := lipgloss.NewStyle().Foreground(common.ColorMuted())
	b.WriteString(hintStyle.Render("Enter=newline  ctrl+y=accept  esc=cancel"))
	b.WriteString("\n\n")

	if d.inlineEditError != "" {
		errStyle := lipgloss.NewStyle().Foreground(common.ColorError()).Bold(true)
		b.WriteString(errStyle.Render("Error: " + d.inlineEditError))
		b.WriteString("\n\n")
	}

	taStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(common.ColorPrimary()).
		Padding(0, 1)
	b.WriteString(taStyle.Render(d.inlineEditTA.View()))
	b.WriteString("\n")

	return b.String()
}
