package sidebar

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/andyrewlee/amux/internal/tickets"
	"github.com/andyrewlee/amux/internal/ui/common"
)

// TicketView renders ticket details in the sidebar.
// It is a read-only view; clicking items does not open new tabs.
type TicketView struct {
	ticket          *tickets.Ticket
	focused         bool
	width           int
	height          int
	showKeymapHints bool
	styles          common.Styles
}

// NewTicketView creates a new ticket view model.
func NewTicketView() *TicketView {
	return &TicketView{
		styles: common.DefaultStyles(),
	}
}

// SetShowKeymapHints controls whether helper text is rendered.
func (m *TicketView) SetShowKeymapHints(show bool) {
	m.showKeymapHints = show
}

// SetStyles updates the component's styles (for theme changes).
func (m *TicketView) SetStyles(styles common.Styles) {
	m.styles = styles
}

// SetTicket updates the displayed ticket. nil clears the view.
func (m *TicketView) SetTicket(t *tickets.Ticket) {
	m.ticket = t
}

// SetSize sets the view dimensions.
func (m *TicketView) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Focus sets the focus state.
func (m *TicketView) Focus() { m.focused = true }

// Blur removes focus.
func (m *TicketView) Blur() { m.focused = false }

// Focused returns whether the view is focused.
func (m *TicketView) Focused() bool { return m.focused }

// View renders the ticket details.
//
//nolint:funlen // legacy suppression
func (m *TicketView) View() string {
	if m.ticket == nil {
		return m.styles.Muted.Render("No ticket selected")
	}

	t := m.ticket
	var b strings.Builder

	// ID and Title
	header := m.styles.Title.Render(t.ID + ": " + t.Title)
	b.WriteString(header)
	b.WriteString("\n")

	// Status
	statusStyle := lipgloss.NewStyle().Bold(true)
	switch t.Status {
	case "open":
		statusStyle = statusStyle.Foreground(common.ColorPrimary())
	case "in_progress":
		statusStyle = statusStyle.Foreground(common.ColorSecondary())
	case "closed":
		statusStyle = statusStyle.Foreground(common.ColorMuted())
	default:
		statusStyle = statusStyle.Foreground(common.ColorForeground())
	}
	b.WriteString(m.styles.Muted.Render("Status: "))
	b.WriteString(statusStyle.Render(t.Status))

	// Priority
	b.WriteString("  ")
	b.WriteString(m.styles.Muted.Render("Pri: "))
	b.WriteString(tickets.PriorityLabelShort(t.Priority))

	b.WriteString("\n")

	// Type
	if t.IssueType != "" {
		b.WriteString(m.styles.Muted.Render("Type: "))
		b.WriteString(t.IssueType)
		b.WriteString("\n")
	}

	// Assignee
	if t.Assignee != "" {
		b.WriteString(m.styles.Muted.Render("Owner: "))
		b.WriteString(t.Assignee)
		b.WriteString("\n")
	}

	// Dates
	b.WriteString(m.styles.Muted.Render("Updated: "))
	b.WriteString(t.UpdatedAt.Format("2006-01-02"))
	b.WriteString("\n")

	// Description (truncated to fit)
	if t.Description != "" {
		b.WriteString("\n")
		descWidth := m.width - 2
		if descWidth < 10 {
			descWidth = 10
		}
		desc := truncateDescription(t.Description, descWidth, maxDescLines(m.height))
		b.WriteString(m.styles.Muted.Render(desc))
	}

	return m.renderWithHelp(b.String())
}

func (m *TicketView) renderWithHelp(content string) string {
	contentWidth := m.width
	if contentWidth < 1 {
		contentWidth = 1
	}
	helpLines := m.helpLines(contentWidth)
	if !m.showKeymapHints {
		helpLines = nil
	}

	contentHeight := 0
	if content != "" {
		contentHeight = strings.Count(content, "\n") + 1
	}

	targetHeight := m.height - len(helpLines)
	if targetHeight < 0 {
		targetHeight = 0
	}

	var b strings.Builder
	b.WriteString(content)
	if targetHeight > contentHeight {
		b.WriteString(strings.Repeat("\n", targetHeight-contentHeight))
	}
	if len(helpLines) > 0 {
		if content != "" && targetHeight == contentHeight {
			b.WriteString("\n")
		}
		b.WriteString(strings.Join(helpLines, "\n"))
	}

	result := b.String()
	if m.height > 0 {
		lines := strings.Split(result, "\n")
		if len(lines) > m.height {
			lines = lines[:m.height]
			result = strings.Join(lines, "\n")
		}
	}
	return result
}

func (m *TicketView) helpItem(key, desc string) string {
	return common.RenderHelpItem(m.styles, key, desc)
}

func (m *TicketView) helpLines(contentWidth int) []string {
	items := []string{
		m.helpItem("enter", "start agent"),
	}
	return common.WrapHelpItems(items, contentWidth)
}

// maxDescLines returns the maximum number of description lines to show
// based on the available height.
func maxDescLines(height int) int {
	if height <= 6 {
		return 2
	}
	return height - 6
}

// truncateDescription truncates a description to fit within the given width
// and maximum number of lines. Each line is word-wrapped to fit the width
// before truncating by line count.
func truncateDescription(desc string, width, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 3
	}
	if width <= 0 {
		width = 20
	}
	// First, word-wrap each line to fit the width
	rawLines := strings.Split(desc, "\n")
	var wrapped []string
	for _, line := range rawLines {
		wrapped = append(wrapped, wrapLineSimple(line, width)...)
	}
	// Then truncate by line count
	if len(wrapped) > maxLines {
		wrapped = wrapped[:maxLines]
		// Indicate truncation
		last := wrapped[maxLines-1]
		runes := []rune(last)
		if len(runes) > width-3 {
			last = string(runes[:width-3])
		}
		wrapped[maxLines-1] = last + "..."
	}
	return strings.Join(wrapped, "\n")
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
