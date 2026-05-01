package center

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/andyrewlee/amux/internal/tickets"
	"github.com/andyrewlee/amux/internal/ui/common"
)

// renderTicketView renders ticket details for a TicketViewTab.
// availableHeight is the total number of content lines available (innerHeight - helpLineCount).
func (m *Model) renderTicketView(tab *Tab) string {
	t := tab.Ticket
	if t == nil {
		return m.styles.Muted.Render("No ticket selected")
	}

	contentWidth := m.contentWidth()
	if contentWidth < 10 {
		contentWidth = 10
	}

	// Build header lines first (non-scrollable)
	var headerLines []string

	// ID + Title
	headerLines = append(headerLines, m.styles.Title.Render(t.ID+": "+t.Title))

	// Status + Priority line
	var spLine strings.Builder
	statusStyle := lipgloss.NewStyle().Bold(true)
	switch t.Status {
	case "open":
		statusStyle = statusStyle.Foreground(common.ColorPrimary())
	case "in_progress":
		statusStyle = statusStyle.Foreground(common.ColorSecondary())
	case "closed":
		statusStyle = statusStyle.Foreground(common.ColorMuted())
	case "blocked":
		statusStyle = statusStyle.Foreground(common.ColorError())
	default:
		statusStyle = statusStyle.Foreground(common.ColorForeground())
	}
	spLine.WriteString(m.styles.Muted.Render("Status: "))
	spLine.WriteString(statusStyle.Render(t.Status))
	spLine.WriteString("  ")
	spLine.WriteString(m.styles.Muted.Render("Priority: "))
	spLine.WriteString(tickets.PriorityLabel(t.Priority))
	headerLines = append(headerLines, spLine.String())

	// Type
	if t.IssueType != "" {
		headerLines = append(headerLines, m.styles.Muted.Render("Type: ")+t.IssueType)
	}

	// Assignee
	if t.Assignee != "" {
		headerLines = append(headerLines, m.styles.Muted.Render("Owner: ")+t.Assignee)
	}

	// Parent epic
	if t.ParentID != "" {
		headerLines = append(headerLines, m.styles.Muted.Render("Epic: ")+t.ParentID)
	}

	// Timestamps
	tsLine := m.styles.Muted.Render("Created: ") + t.CreatedAt.Format("2006-01-02") +
		"  " + m.styles.Muted.Render("Updated: ") + t.UpdatedAt.Format("2006-01-02")
	headerLines = append(headerLines, tsLine)

	// Separator
	sepStyle := lipgloss.NewStyle().Foreground(common.ColorBorder())
	headerLines = append(headerLines, sepStyle.Render(strings.Repeat("─", contentWidth)))

	// Calculate available height for description
	availableHeight := m.height - 2 // inner pane height
	if availableHeight < 0 {
		availableHeight = 0
	}
	// Subtract help lines
	helpLineCount := len(m.helpLines(contentWidth))
	availableHeight -= helpLineCount
	if availableHeight < 0 {
		availableHeight = 0
	}

	// Build the content: header lines first, then description window
	var allLines []string
	allLines = append(allLines, headerLines...)

	if t.Description != "" {
		descLines := ticketViewWrapDescription(t.Description, contentWidth)
		if len(descLines) > 0 {
			descAvailable := availableHeight - len(headerLines)
			if descAvailable < 1 {
				descAvailable = 1
			}

			// Clamp scroll offset
			maxOffset := len(descLines) - descAvailable
			if maxOffset < 0 {
				maxOffset = 0
			}
			if m.ticketViewScrollOffset > maxOffset {
				m.ticketViewScrollOffset = maxOffset
			}
			if m.ticketViewScrollOffset < 0 {
				m.ticketViewScrollOffset = 0
			}

			start := m.ticketViewScrollOffset
			end := start + descAvailable
			if end > len(descLines) {
				end = len(descLines)
			}

			for i := start; i < end; i++ {
				allLines = append(allLines, m.styles.Body.Render(descLines[i]))
			}

			// Scroll indicator if scrolled
			if start > 0 || end < len(descLines) {
				scrollStyle := lipgloss.NewStyle().
					Foreground(common.ColorMuted())
				indicator := formatTicketScrollPos(start+1, len(descLines), start+1, end)
				allLines = append(allLines, scrollStyle.Render(indicator))
			}
		}
	}

	return strings.Join(allLines, "\n")
}

// formatTicketScrollPos formats the scroll position for the ticket view.
func formatTicketScrollPos(start, total, topVis, botVis int) string {
	if total <= 1 {
		return ""
	}
	if botVis >= total {
		return "=== END ==="
	}
	if topVis <= 1 {
		return "=== TOP ==="
	}
	// Scroll position as lines
	ratio := float64(start) / float64(total)
	pct := int(ratio * 100)
	if pct > 99 {
		pct = 99
	}
	return "----- " + itoa(pct) + "% -----"
}

// itoa is a simple int-to-string helper to avoid importing strconv for this one use.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ticketViewWrapDescription wraps description text to the given width.
func ticketViewWrapDescription(desc string, width int) []string {
	if desc == "" {
		return nil
	}
	if width < 10 {
		width = 10
	}

	rawLines := strings.Split(desc, "\n")
	var wrapped []string
	for _, line := range rawLines {
		wrapped = append(wrapped, wrapLineSimple(line, width)...)
	}
	return wrapped
}

// handleTicketViewKey handles key events for TicketViewTab.
func (m *Model) handleTicketViewKey(msg tea.KeyPressMsg) (*Model, tea.Cmd) {
	k := msg.Key()

	switch k.Code {
	case tea.KeyEsc:
		return m, m.closeCurrentTab()

	case tea.KeyEnter:
		// Start draft for the current ticket
		tabs := m.getTabs()
		activeIdx := m.getActiveTabIdx()
		if activeIdx < len(tabs) {
			tab := tabs[activeIdx]
			if tab.Ticket != nil && m.workspace != nil {
				m.StartDraft(tab.Ticket, m.workspace)
				return m, nil
			}
		}
		return m, nil

	case tea.KeyPgUp:
		m.ticketViewScrollOffset -= 5
		if m.ticketViewScrollOffset < 0 {
			m.ticketViewScrollOffset = 0
		}
		return m, nil

	case tea.KeyPgDown:
		m.ticketViewScrollOffset += 5
		return m, nil

	case tea.KeyUp:
		m.ticketViewScrollOffset--
		if m.ticketViewScrollOffset < 0 {
			m.ticketViewScrollOffset = 0
		}
		return m, nil

	case tea.KeyDown:
		m.ticketViewScrollOffset++
		return m, nil

	default:
		// Other keys pass through (no-op for ticket view)
		return m, nil
	}
}

// wrapLineSimple wraps a single line to the given width by breaking at spaces.
func wrapLineSimple(line string, width int) []string {
	if width <= 0 || len(line) == 0 {
		return []string{line}
	}
	runes := []rune(line)
	if len(runes) <= width {
		return []string{line}
	}
	var result []string
	for len(runes) > 0 {
		if len(runes) <= width {
			result = append(result, string(runes))
			break
		}
		// Find a good break point
		breakAt := width
		for j := width; j > width/2; j-- {
			if j < len(runes) && (runes[j] == ' ' || runes[j] == '-') {
				breakAt = j + 1
				break
			}
		}
		result = append(result, string(runes[:breakAt]))
		runes = runes[breakAt:]
		// Skip leading space on next line
		if len(runes) > 0 && runes[0] == ' ' {
			runes = runes[1:]
		}
	}
	return result
}
