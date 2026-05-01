package center

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/config"
	"github.com/andyrewlee/amux/internal/data" //nolint:depguard // existing architectural import, see bmx-zlc.2
	"github.com/andyrewlee/amux/internal/tickets"
	"github.com/andyrewlee/amux/internal/ui/common"
)

type DraftSlot int

const (
	SlotTicket DraftSlot = iota
	SlotHarness
	SlotModel
	SlotAgent
	SlotConfirm
	SlotComplete
)

// editMode determines which template the inline editor is editing.
type editMode int

const (
	editModeCommand editMode = iota
	editModePrompt
)

type DraftComplete struct {
	Assistant   string
	Workspace   *data.Workspace
	TicketID    string
	TicketTitle string
	Model       string
	AgentMode   string
}

// DraftCancelled is emitted when the user cancels the draft flow.
type DraftCancelled struct{}

type draftTemplateLoadedMsg struct {
	content string
}

type draftTemplateErrorMsg struct {
	err error
}

// filePickerID is the dialog ID for the template file picker.
const filePickerID = "draft-template-picker"

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

	// Confirm state
	renderer *tickets.Renderer
	dirty    bool // true if templates were edited or loaded from file

	// Template overrides — populated by inline edit or file picker.
	// Non-empty values take precedence over the shared config, so canceling
	// the draft discards all edits without leaking mutations.
	commandOverride string
	promptOverride  string

	// Inline editor state
	inlineEditActive bool
	inlineEditTA     textarea.Model
	inlineEditMode   editMode
	inlineEditError  string

	// File picker state
	filePickerActive bool
	filePicker       *common.FilePicker
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
		renderer:       tickets.NewRenderer(),
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
	if d.filePickerActive {
		d.filePicker.SetSize(width, height)
	}
	if d.inlineEditActive {
		d.inlineEditTA.SetWidth(min(60, max(20, width-10)))
		d.inlineEditTA.SetHeight(min(10, max(3, height/3)))
	}
}

// Dirty reports whether the draft has unsaved template edits.
func (d *Draft) Dirty() bool { return d.dirty }

func (d *Draft) Focus() { d.focused = true; d.filterInput.Focus() }
func (d *Draft) Blur()  { d.focused = false; d.filterInput.Blur() }

func (d *Draft) Update(msg tea.Msg) (*Draft, tea.Cmd) {
	// File picker takes priority when active.
	if d.filePickerActive {
		return d.handleFilePickerMsg(msg)
	}
	// Inline editor takes priority when active.
	if d.inlineEditActive {
		return d.handleInlineEditMsg(msg)
	}

	switch msg := msg.(type) {
	case draftTemplateLoadedMsg:
		return d.handleTemplateLoaded(msg)
	case draftTemplateErrorMsg:
		// Re-show confirm view; the error was already shown as a warning.
		return d, nil
	case common.DialogResult:
		if msg.ID == filePickerID {
			return d.handleFilePickerResult(msg)
		}
	case tea.KeyPressMsg:
		return d.handleKey(msg)
	}
	return d, nil
}

func (d *Draft) handleKey(msg tea.KeyPressMsg) (*Draft, tea.Cmd) {
	if d.activeSlot == SlotComplete {
		return d, nil
	}

	// Confirm slot has its own key handling.
	if d.activeSlot == SlotConfirm {
		return d.handleConfirmKey(msg)
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
	case SlotConfirm:
		d.activeSlot = SlotAgent
		d.resetFilter(d.agentOptions)
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
	// File picker overlay takes over the entire view.
	if d.filePickerActive {
		return d.renderFilePicker()
	}
	// Inline editor overlay takes over the entire view.
	if d.inlineEditActive {
		return d.renderInlineEdit()
	}

	var b strings.Builder

	b.WriteString("\n")

	stepLabels := []string{"Ticket", "Harness", "Model", "Agent"}

	if d.activeSlot == SlotConfirm || d.activeSlot == SlotComplete {
		b.WriteString(d.renderConfirmView(stepLabels))
		return b.String()
	}

	stepNum := int(d.activeSlot) + 1

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

// HelpLines returns the help lines for the drafting flow.
func (d *Draft) HelpLines(_ int) []string {
	return nil
}
