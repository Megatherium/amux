package center

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/andyrewlee/amux/internal/config"
	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/tickets"
	"github.com/andyrewlee/amux/internal/ui/common"
)

type DraftSlot int

const (
	SlotTicket DraftSlot = iota
	SlotHarness
	SlotModel
	SlotAgent
	SlotComplete
)

type DraftComplete struct {
	Assistant   string
	Workspace   *data.Workspace
	TicketID    string
	TicketTitle string
	Model       string
	AgentMode   string
}

type DraftCancelled struct{}

type Draft struct {
	ticket     *tickets.Ticket
	harness    string
	model      string
	agent      string
	activeSlot DraftSlot
	cursor     int

	harnessOptions []string
	modelOptions   []string
	agentOptions   []string

	config *config.Config

	styles  common.Styles
	width   int
	height  int
	focused bool

	filterInput     textinput.Model
	filteredIndices []int

	workspace *data.Workspace
}

func NewDraft(ticket *tickets.Ticket, ws *data.Workspace, cfg *config.Config, styles common.Styles) *Draft {
	fi := textinput.New()
	fi.Placeholder = "Type to filter..."
	fi.Focus()
	fi.CharLimit = 30
	fi.SetWidth(20)
	fi.SetVirtualCursor(false)

	d := &Draft{
		ticket:         ticket,
		activeSlot:     SlotHarness,
		config:         cfg,
		styles:         styles,
		workspace:      ws,
		filterInput:    fi,
		cursor:         0,
		harnessOptions: cfg.AssistantNames(),
	}

	if len(d.harnessOptions) == 0 {
		d.harnessOptions = []string{"claude"}
	}

	d.filteredIndices = make([]int, len(d.harnessOptions))
	for i := range d.harnessOptions {
		d.filteredIndices[i] = i
	}

	if cfg.Defaults != nil && cfg.Defaults.Harness != "" {
		for _, name := range d.harnessOptions {
			if name == cfg.Defaults.Harness {
				d.confirmHarness(name)
				break
			}
		}
	}

	return d
}

func (d *Draft) SetSize(width, height int) {
	d.width = width
	d.height = height
	d.filterInput.SetWidth(min(30, max(10, width-6)))
}

func (d *Draft) Focus() { d.focused = true; d.filterInput.Focus() }
func (d *Draft) Blur()  { d.focused = false; d.filterInput.Blur() }

func (d *Draft) Update(msg tea.Msg) (*Draft, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return d.handleKey(msg)
	}
	return d, nil
}

func (d *Draft) handleKey(msg tea.KeyPressMsg) (*Draft, tea.Cmd) {
	if d.activeSlot == SlotComplete {
		return d, nil
	}

	filterText := d.filterInput.Value()

	switch {
	case msg.String() == "esc":
		if filterText != "" {
			d.filterInput.SetValue("")
			d.applyFilter()
			return d, nil
		}
		if d.activeSlot == SlotHarness {
			return d, func() tea.Msg { return DraftCancelled{} }
		}
		d.goBack()
		return d, nil

	case msg.String() == "up":
		if d.cursor > 0 {
			d.cursor--
		}
		return d, nil

	case msg.String() == "down":
		if d.cursor < len(d.filteredIndices)-1 {
			d.cursor++
		}
		return d, nil

	case msg.String() == "tab":
		if len(d.filteredIndices) > 0 {
			d.cursor = (d.cursor + 1) % len(d.filteredIndices)
		}
		return d, nil

	case msg.String() == "shift+tab":
		if len(d.filteredIndices) > 0 {
			d.cursor = (d.cursor - 1 + len(d.filteredIndices)) % len(d.filteredIndices)
		}
		return d, nil

	case msg.String() == "enter":
		if len(d.filteredIndices) == 0 {
			return d, nil
		}
		idx := d.filteredIndices[d.cursor]
		return d.confirmSelection(idx)

	default:
		newInput, cmd := d.filterInput.Update(msg)
		d.filterInput = newInput
		d.applyFilter()
		return d, cmd
	}
}

func (d *Draft) goBack() {
	switch d.activeSlot {
	case SlotModel:
		d.activeSlot = SlotHarness
		d.model = ""
		d.modelOptions = nil
		d.agentOptions = nil
		d.resetFilter(d.harnessOptions)
	case SlotAgent:
		d.activeSlot = SlotModel
		d.agent = ""
		d.resetFilter(d.modelOptions)
	}
}

func (d *Draft) resetFilter(options []string) {
	d.filterInput.SetValue("")
	d.cursor = 0
	d.filteredIndices = make([]int, len(options))
	for i := range options {
		d.filteredIndices[i] = i
	}
	d.filterInput.Focus()
}

func (d *Draft) applyFilter() {
	options := d.currentOptions()
	query := d.filterInput.Value()
	d.filteredIndices = nil
	for i, opt := range options {
		if fuzzyMatch(query, opt) {
			d.filteredIndices = append(d.filteredIndices, i)
		}
	}
	if d.cursor >= len(d.filteredIndices) {
		d.cursor = max(0, len(d.filteredIndices)-1)
	}
}

func fuzzyMatch(pattern, target string) bool {
	if pattern == "" {
		return true
	}
	pattern = strings.ToLower(pattern)
	target = strings.ToLower(target)
	pi := 0
	for ti := 0; ti < len(target) && pi < len(pattern); ti++ {
		if target[ti] == pattern[pi] {
			pi++
		}
	}
	return pi == len(pattern)
}

func (d *Draft) currentOptions() []string {
	switch d.activeSlot {
	case SlotHarness:
		return d.harnessOptions
	case SlotModel:
		return d.modelOptions
	case SlotAgent:
		return d.agentOptions
	}
	return nil
}

func (d *Draft) confirmSelection(idx int) (*Draft, tea.Cmd) {
	switch d.activeSlot {
	case SlotHarness:
		if idx >= len(d.harnessOptions) {
			return d, nil
		}
		name := d.harnessOptions[idx]
		d.confirmHarness(name)
		if d.activeSlot == SlotComplete {
			return d, d.launchCmd()
		}
		return d, nil

	case SlotModel:
		if idx >= len(d.modelOptions) {
			return d, nil
		}
		d.model = d.modelOptions[idx]
		d.activeSlot = SlotAgent
		d.resetFilter(d.agentOptions)
		if d.config.Defaults != nil && d.config.Defaults.Agent != "" {
			for _, a := range d.agentOptions {
				if a == d.config.Defaults.Agent {
					d.agent = a
					d.activeSlot = SlotComplete
					return d, d.launchCmd()
				}
			}
		}
		return d, nil

	case SlotAgent:
		if idx >= len(d.agentOptions) {
			return d, nil
		}
		d.agent = d.agentOptions[idx]
		d.activeSlot = SlotComplete
		return d, d.launchCmd()
	}
	return d, nil
}

func (d *Draft) confirmHarness(name string) {
	d.harness = name
	harnessCfg, ok := d.config.Assistants[name]
	if !ok {
		d.modelOptions = []string{"default"}
		d.agentOptions = []string{"default"}
	} else {
		if len(harnessCfg.SupportedModels) > 0 {
			d.modelOptions = make([]string, len(harnessCfg.SupportedModels))
			copy(d.modelOptions, harnessCfg.SupportedModels)
		} else {
			d.modelOptions = []string{"default"}
		}
		if len(harnessCfg.SupportedAgents) > 0 {
			d.agentOptions = make([]string, len(harnessCfg.SupportedAgents))
			copy(d.agentOptions, harnessCfg.SupportedAgents)
		} else {
			d.agentOptions = []string{"default"}
		}
	}

	d.model = ""
	d.agent = ""
	d.activeSlot = SlotModel
	d.resetFilter(d.modelOptions)

	if d.config.Defaults != nil {
		if d.config.Defaults.Model != "" {
			for _, m := range d.modelOptions {
				if m == d.config.Defaults.Model {
					d.model = m
					d.activeSlot = SlotAgent
					d.resetFilter(d.agentOptions)
					break
				}
			}
		}
	}
}

func (d *Draft) launchCmd() tea.Cmd {
	ticketID, ticketTitle := "", ""
	if d.ticket != nil {
		ticketID = d.ticket.ID
		ticketTitle = d.ticket.Title
	}
	return func() tea.Msg {
		return DraftComplete{
			Assistant:   d.harness,
			Workspace:   d.workspace,
			TicketID:    ticketID,
			TicketTitle: ticketTitle,
			Model:       d.model,
			AgentMode:   d.agent,
		}
	}
}

func (d *Draft) View() string {
	var b strings.Builder

	b.WriteString("\n")

	stepLabels := []string{"Ticket", "Harness", "Model", "Agent"}
	stepNum := int(d.activeSlot) + 1
	if d.activeSlot >= SlotComplete {
		stepNum = 4
	}

	headerStyle := d.styles.Title
	b.WriteString(headerStyle.Render(fmt.Sprintf("Configure Agent   Step %d/4", stepNum)))
	b.WriteString("\n\n")

	for i, label := range stepLabels {
		slot := DraftSlot(i)
		switch {
		case slot == SlotTicket || (slot < d.activeSlot && d.slotValue(slot) != ""):
			b.WriteString(d.renderCollapsedSlot(slot, label))
		case slot == d.activeSlot:
			b.WriteString(d.renderExpandedSlot(slot, label))
		default:
			b.WriteString(d.renderFutureSlot(label))
		}
		if i < len(stepLabels)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

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

func (d *Draft) renderExpandedSlot(slot DraftSlot, label string) string {
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
