package common

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

type TicketPickerItem struct {
	ID        string
	Title     string
	Status    string
	IssueType string
	Priority  int
}

func NewTicketPicker(items []TicketPickerItem) *Dialog {
	options := make([]string, 0, len(items)+1)
	for _, it := range items {
		options = append(options, it.ID+" "+it.Title)
	}
	options = append(options, "no-ticket")

	allIndices := make([]int, len(options))
	for i := range options {
		allIndices[i] = i
	}

	fi := textinput.New()
	fi.Placeholder = "Type to filter..."
	fi.Focus()
	fi.CharLimit = 20
	fi.SetWidth(30)
	fi.SetVirtualCursor(false)

	return &Dialog{
		id:              "ticket-picker",
		dtype:           DialogSelect,
		title:           "Select Ticket",
		message:         "Choose a ticket for context (or skip):",
		options:         options,
		cursor:          0,
		filterEnabled:   true,
		filterInput:     fi,
		filteredIndices: allIndices,
		ticketItems:     items,
	}
}

func (d *Dialog) renderTicketPickerOptions(baseLine int) []string {
	lines := []string{}
	lineIndex := baseLine

	if d.filterEnabled {
		inputLines := strings.Split(d.filterInput.View(), "\n")
		lines = append(lines, inputLines...)
		lineIndex += len(inputLines)
		lines = append(lines, "", "")
		lineIndex += 2
	}

	if d.filterEnabled && len(d.filteredIndices) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorMuted()).Render("No matches"))
		return lines
	}

	for cursorIdx, originalIdx := range d.filteredIndices {
		width := d.dialogContentWidth()
		var line string
		if originalIdx < len(d.ticketItems) {
			line = d.renderTicketRow(cursorIdx, d.ticketItems[originalIdx])
		} else {
			line = d.renderNoTicketRow(cursorIdx)
		}
		d.addOptionHit(cursorIdx, originalIdx, lineIndex, 0, width)
		lines = append(lines, line)
		lineIndex++
	}
	return lines
}

func (d *Dialog) renderTicketRow(cursorIdx int, item TicketPickerItem) string {
	cursor := Icons.CursorEmpty + " "
	if cursorIdx == d.cursor {
		cursor = Icons.Cursor + " "
	}

	statusIcon := statusIcon(item.Status)
	statusStyle := lipgloss.NewStyle().Foreground(statusColor(item.Status))
	idStyle := lipgloss.NewStyle().Foreground(ColorForeground()).Bold(cursorIdx == d.cursor)
	titleStyle := lipgloss.NewStyle().Foreground(ColorMuted())
	metaStyle := lipgloss.NewStyle().Foreground(ColorMuted())

	priorityStr := fmt.Sprintf("P%d", item.Priority)
	meta := metaStyle.Render(item.IssueType + " " + priorityStr)

	label := idStyle.Render(item.ID)
	title := titleStyle.Render(truncate(item.Title, 30))

	return cursor + statusStyle.Render(statusIcon) + " " + label + "  " + title + "  " + meta
}

func (d *Dialog) renderNoTicketRow(cursorIdx int) string {
	cursor := Icons.CursorEmpty + " "
	if cursorIdx == d.cursor {
		cursor = Icons.Cursor + " "
	}
	mutedStyle := lipgloss.NewStyle().Foreground(ColorMuted())
	return cursor + mutedStyle.Render("── No ticket ──")
}

func statusIcon(status string) string {
	switch status {
	case "open":
		return Icons.Idle
	case "in_progress":
		return Icons.Pending
	case "closed":
		return Icons.Clean
	default:
		return Icons.Idle
	}
}

func statusColor(status string) color.Color {
	switch status {
	case "open":
		return ColorForeground()
	case "in_progress":
		return ColorPrimary()
	case "closed":
		return ColorMuted()
	default:
		return ColorMuted()
	}
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen < 1 {
		return ""
	}
	return string(runes[:maxLen-1]) + "…"
}
