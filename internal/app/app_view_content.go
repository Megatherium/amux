package app

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/andyrewlee/amux/internal/tickets"
	"github.com/andyrewlee/amux/internal/ui/common"
)

func (a *App) centerPaneStyle() lipgloss.Style {
	width := a.ui.layout.CenterWidth()
	height := a.ui.layout.Height()

	return lipgloss.NewStyle().
		Width(width-2).
		Height(height-2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(common.ColorBorder()).
		Padding(0, 1)
}

// renderCenterPaneContent renders the center pane content when no tabs (raw content, no borders)
func (a *App) renderCenterPaneContent() string {
	if a.showWelcome {
		return a.renderWelcome()
	}

	// Ticket preview takes priority when no workspace is active or when the
	// active workspace has no tabs (the center is showing the info screen).
	if a.previewTicket != nil && !a.ui.center.HasTabs() && !a.ui.center.HasDraft() {
		return a.renderTicketPreview()
	}

	if a.activeWorkspace != nil {
		return a.renderWorkspaceInfo()
	}

	return "Select a workspace from the dashboard"
}

func (a *App) centerPaneContentOrigin() (x, y int) {
	if a.ui == nil || a.ui.layout == nil {
		return 0, 0
	}
	frameX, frameY := a.centerPaneStyle().GetFrameSize()
	gapX := 0
	if a.ui.layout.ShowCenter() {
		gapX = a.ui.layout.GapX()
	}
	return a.ui.layout.LeftGutter() + a.ui.layout.DashboardWidth() + gapX + frameX/2, a.ui.layout.TopGutter() + frameY/2
}

func (a *App) goHome() {
	a.showWelcome = true
	a.activeWorkspace = nil
	if a.ui.center != nil {
		a.ui.center.SetWorkspace(nil)
	}
	if a.ui.sidebar != nil {
		a.ui.sidebar.SetWorkspace(nil)
		a.ui.sidebar.SetGitStatus(nil)
	}
	if a.ui.sidebarTerminal != nil {
		_ = a.ui.sidebarTerminal.SetWorkspace(nil)
	}
	if a.ui.dashboard != nil {
		a.ui.dashboard.ClearActiveRoot()
	}
	a.centerBtnFocused = false
	a.centerBtnIndex = 0
	a.previewTicket = nil
	a.previewProject = nil
}

// hasTicketService reports whether the active project has a ticket service configured.
func (a *App) hasTicketService() bool {
	return a.activeProject != nil && a.ticketServices != nil && a.ticketServices[a.activeProject.Path] != nil
}

// renderWorkspaceInfo renders information about the active workspace
func (a *App) renderWorkspaceInfo() string {
	ws := a.activeWorkspace

	title := a.styles.Title.Render(ws.Name)
	content := title + "\n\n"
	content += fmt.Sprintf("Branch: %s\n", ws.Branch)
	content += fmt.Sprintf("Path: %s\n", ws.Root)

	if a.activeProject != nil {
		content += fmt.Sprintf("Project: %s\n", a.activeProject.Name)
	}

	activeStyle := lipgloss.NewStyle().Foreground(common.ColorForeground()).Bold(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(common.ColorMuted())

	hasBeads := a.hasTicketService()

	if hasBeads {
		ticketBtnStyle := inactiveStyle
		if a.centerBtnFocused && a.centerBtnIndex == 0 {
			ticketBtnStyle = activeStyle
		}
		agentBtnStyle := inactiveStyle
		if a.centerBtnFocused && a.centerBtnIndex == 1 {
			agentBtnStyle = activeStyle
		}

		ticketBtn := ticketBtnStyle.Render("[New Agent with Ticket]")
		agentBtn := agentBtnStyle.Render("[New Agent]")
		content += "\n" + ticketBtn + "  " + agentBtn
	} else {
		btnStyle := inactiveStyle
		if a.centerBtnFocused && a.centerBtnIndex == 0 {
			btnStyle = activeStyle
		}
		agentBtn := btnStyle.Render("[New Agent]")
		content += "\n" + agentBtn
	}

	if a.config.UI.ShowKeymapHints {
		helpText := a.prefixHelpLabel + " t a:agent"
		if hasBeads {
			helpText = a.prefixHelpLabel + " t b:agent+ticket  " + "t a:agent"
		}
		content += "\n" + a.styles.Help.Render(helpText)
	}

	return content
}

// renderWelcome renders the welcome screen
func (a *App) renderWelcome() string {
	content := a.welcomeContent()

	// Center the content in the pane
	width := a.ui.layout.CenterWidth() - 4 // Account for borders/padding
	height := a.ui.layout.Height() - 2

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

func (a *App) welcomeContent() string {
	logo, logoStyle := a.welcomeLogo()
	var b strings.Builder
	b.WriteString(logoStyle.Render(logo))
	b.WriteString("\n\n")

	activeStyle := lipgloss.NewStyle().Foreground(common.ColorForeground()).Bold(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(common.ColorMuted())

	addProjectStyle := inactiveStyle
	settingsStyle := inactiveStyle
	if a.centerBtnFocused {
		switch a.centerBtnIndex {
		case 0:
			addProjectStyle = activeStyle
		case 1:
			settingsStyle = activeStyle
		}
	}
	addProject := addProjectStyle.Render("[Add project]")
	settingsBtn := settingsStyle.Render("[Settings]")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, addProject, "  ", settingsBtn))
	b.WriteString("\n")
	if a.config.UI.ShowKeymapHints {
		b.WriteString(a.styles.Help.Render("Dashboard: j/k to move • Enter to select"))
	}
	return b.String()
}

// renderTicketPreview renders ticket info in the center pane when the cursor
// hovers over a ticket row in the dashboard and no agent tabs are open.
func (a *App) renderTicketPreview() string {
	t := a.previewTicket
	if t == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")

	// Title with ticket ID
	header := a.styles.Title.Render(t.ID + ": " + t.Title)
	b.WriteString(header)
	b.WriteString("\n\n")

	// Status badge
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
	b.WriteString(a.styles.Muted.Render("Status: "))
	b.WriteString(statusStyle.Render(t.Status))

	// Priority
	b.WriteString("  ")
	b.WriteString(a.styles.Muted.Render("Priority: "))
	b.WriteString(tickets.PriorityLabel(t.Priority))

	// Type
	if t.IssueType != "" {
		b.WriteString("  ")
		b.WriteString(a.styles.Muted.Render("Type: "))
		b.WriteString(t.IssueType)
	}

	b.WriteString("\n")

	// Assignee
	if t.Assignee != "" {
		b.WriteString(a.styles.Muted.Render("Assignee: "))
		b.WriteString(t.Assignee)
		b.WriteString("\n")
	}

	// Dates
	b.WriteString(a.styles.Muted.Render("Created: "))
	b.WriteString(t.CreatedAt.Format("2006-01-02 15:04"))
	b.WriteString("  ")
	b.WriteString(a.styles.Muted.Render("Updated: "))
	b.WriteString(t.UpdatedAt.Format("2006-01-02 15:04"))
	b.WriteString("\n")

	// Description
	if t.Description != "" {
		b.WriteString("\n")
		// Word-wrap the description to fit the pane
		descWidth := a.ui.layout.CenterWidth() - 6
		if descWidth < 20 {
			descWidth = 20
		}
		desc := wordWrap(t.Description, descWidth)
		b.WriteString(a.styles.Muted.Render(desc))
		b.WriteString("\n")
	}

	// Action hint
	b.WriteString("\n")
	helpStyle := lipgloss.NewStyle().Foreground(common.ColorMuted())
	b.WriteString(helpStyle.Render("Enter: start agent with ticket"))

	return b.String()
}

// wordWrap wraps text to the given width, preserving existing newlines.
func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}
	var result strings.Builder
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}
		wrapped := wrapLine(line, width)
		result.WriteString(wrapped)
	}
	return result.String()
}

// wrapLine wraps a single line to the given width.
func wrapLine(line string, width int) string {
	if utf8.RuneCountInString(line) <= width {
		return line
	}
	var result strings.Builder
	runes := []rune(line)
	for len(runes) > 0 {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		if len(runes) <= width {
			result.WriteString(string(runes))
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
		result.WriteString(string(runes[:breakAt]))
		runes = runes[breakAt:]
		// Skip leading space on next line
		if len(runes) > 0 && runes[0] == ' ' {
			runes = runes[1:]
		}
	}
	return result.String()
}

func (a *App) welcomeLogo() (string, lipgloss.Style) {
	logo := `
 8888b.  88888b.d88b.  888  888 888  888
    "88b 888 "888 "88b 888  888  Y8bd8P
.d888888 888  888  888 888  888   X88K
888  888 888  888  888 Y88b 888 .d8""8b.
"Y888888 888  888  888  "Y88888 888  888`

	logoStyle := lipgloss.NewStyle().
		Foreground(common.ColorPrimary()).
		Bold(true)
	return logo, logoStyle
}
