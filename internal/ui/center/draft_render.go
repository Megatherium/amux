package center

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/andyrewlee/amux/internal/ui/common"
)

func (d *Draft) slotValue(slot DraftSlot) string {
	switch slot {
	case SlotTicket:
		if d.ticket != nil {
			return d.ticket.ID
		}
	case SlotHarness:
		return d.harness
	case SlotModel:
		return d.model
	case SlotAgent:
		return d.agent
	}
	return ""
}

func (d *Draft) renderCollapsedSlot(slot DraftSlot, label string) string {
	value := d.slotValue(slot)
	if slot == SlotTicket && d.ticket != nil {
		value = d.ticket.ID + " — " + truncateStr(d.ticket.Title, 40)
	}
	checkStyle := lipgloss.NewStyle().Foreground(common.ColorSuccess())
	mutedStyle := lipgloss.NewStyle().Foreground(common.ColorMuted())
	return checkStyle.Render("  ✓ ") + mutedStyle.Render(label+": "+value)
}

func (d *Draft) renderExpandedSlot(_ DraftSlot, label string) string {
	var b strings.Builder

	arrowStyle := lipgloss.NewStyle().Foreground(common.ColorPrimary())
	b.WriteString(arrowStyle.Render("  ▸ Select " + label))
	b.WriteString("\n")

	options := d.currentOptions()
	if len(options) == 0 {
		mutedStyle := lipgloss.NewStyle().Foreground(common.ColorMuted())
		b.WriteString("    ")
		b.WriteString(mutedStyle.Render("No options available"))
		return b.String()
	}

	boxWidth := max(20, min(d.width-4, 45))

	b.WriteString("    ")
	filterView := d.filterInput.View()
	filterLine := lipgloss.NewStyle().Width(boxWidth).Render(filterView)
	b.WriteString(filterLine)
	b.WriteString("\n")

	sepStyle := lipgloss.NewStyle().Foreground(common.ColorBorder())
	sep := sepStyle.Render("    " + strings.Repeat("─", boxWidth))
	b.WriteString(sep)
	b.WriteString("\n")

	maxVisible := min(len(d.filteredIndices), d.availableOptionLines())
	start := 0
	if d.cursor >= maxVisible {
		start = d.cursor - maxVisible + 1
	}
	if start < 0 {
		start = 0
	}

	for vi := range maxVisible {
		fi := vi + start
		if fi >= len(d.filteredIndices) {
			break
		}
		origIdx := d.filteredIndices[fi]
		opt := options[origIdx]
		isCursor := fi == d.cursor

		cursor := common.Icons.CursorEmpty + " "
		nameStyle := lipgloss.NewStyle().Foreground(common.ColorForeground())
		if isCursor {
			cursor = common.Icons.Cursor + " "
			nameStyle = lipgloss.NewStyle().
				Foreground(common.ColorForeground()).
				Background(common.ColorSelection()).
				Bold(true)
		}

		indicator := lipgloss.NewStyle().Foreground(common.AgentColor(opt)).Render(common.Icons.Running)
		line := "    " + cursor + indicator + " " + nameStyle.Render(opt)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func (d *Draft) renderFutureSlot(label string) string {
	dimStyle := lipgloss.NewStyle().Foreground(common.ColorBorder())
	return dimStyle.Render("  ○ " + label)
}

func (d *Draft) availableOptionLines() int {
	linesUsed := 4 + int(d.activeSlot)*2
	remaining := max(3, d.height-linesUsed-2)
	return remaining
}

func truncateStr(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes > 3 {
		return string(runes[:maxRunes-3]) + "..."
	}
	return string(runes[:maxRunes])
}
