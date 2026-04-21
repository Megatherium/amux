package center

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/ui/common"
)

// enterFilePicker opens the file picker for selecting a template file.
func (d *Draft) enterFilePicker() tea.Cmd {
	dir := "."
	if d.workspace != nil && d.workspace.Root != "" {
		dir = d.workspace.Root
	}

	fp := common.NewFilePicker(filePickerID, dir, false)
	fp.SetTitle("Select Template File")
	fp.SetSize(d.width, d.height)
	fp.Show()
	d.filePicker = fp
	d.filePickerActive = true

	return nil
}

// handleFilePickerMsg routes messages through the file picker.
func (d *Draft) handleFilePickerMsg(msg tea.Msg) (*Draft, tea.Cmd) {
	newFP, cmd := d.filePicker.Update(msg)
	d.filePicker = newFP

	// FilePicker emits DialogResult via tea.Cmd, so we don't check for
	// selection here. The result arrives as a regular message handled
	// in Update() via the common.DialogResult case.
	return d, cmd
}

// handleFilePickerResult processes the result from the file picker.
func (d *Draft) handleFilePickerResult(msg common.DialogResult) (*Draft, tea.Cmd) {
	d.filePickerActive = false

	if !msg.Confirmed || msg.Value == "" {
		// User canceled.
		return d, nil
	}

	return d, d.loadTemplateFromFile(msg.Value)
}

// loadTemplateFromFile reads a template file and feeds it into the inline editor.
func (d *Draft) loadTemplateFromFile(path string) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(path)
		if err != nil {
			return draftTemplateErrorMsg{err: fmt.Errorf("read template: %w", err)}
		}

		content := string(data)

		// Basic binary detection.
		for _, b := range data {
			if b == 0 {
				return draftTemplateErrorMsg{err: fmt.Errorf("file contains binary data: %s", path)}
			}
		}

		return draftTemplateLoadedMsg{content: content}
	}
}

// handleTemplateLoaded opens the inline editor with the loaded template content.
func (d *Draft) handleTemplateLoaded(msg draftTemplateLoadedMsg) (*Draft, tea.Cmd) {
	// Determine which template to edit based on agent selection.
	mode := editModeCommand
	if d.agent != "" {
		mode = editModePrompt
	}

	d.inlineEditMode = mode
	d.inlineEditError = ""

	ta := newTextarea(msg.content, d.width, d.height)
	ta.Focus()

	d.inlineEditTA = ta
	d.inlineEditActive = true

	return d, nil
}

// renderFilePicker renders the file picker overlay.
func (d *Draft) renderFilePicker() string {
	var b strings.Builder

	b.WriteString(d.styles.Title.Render("Select Template File"))
	b.WriteString("\n")
	b.WriteString(d.filePicker.View())

	return b.String()
}
