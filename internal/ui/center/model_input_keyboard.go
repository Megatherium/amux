package center

import (
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/logging"
	"github.com/andyrewlee/amux/internal/ui/common"
)

// Unexported keybindings for the center pane keyboard operations.
var (
	keyPrevTab        = key.NewBinding(key.WithKeys("ctrl+p"))
	keyNextTab        = key.NewBinding(key.WithKeys("ctrl+n"))
	keyCloseTab       = key.NewBinding(key.WithKeys("ctrl+w"))
	keyTabEscapeHatch = key.NewBinding(key.WithKeys("ctrl+]"))
	// keyEscapeHatch intercepts ctrl+[ to send a raw Escape byte (\x1b) to the PTY.
	keyEscapeHatch = key.NewBinding(key.WithKeys("ctrl+["))
	// keyEsc matches the actual physical Esc key, used to exit/close dialogs or tabs.
	keyEsc = key.NewBinding(key.WithKeys("esc"))
)

// sendInputToTab sends string/bytes to a tab, routing through the tab actor if
// ready, and falling back to a direct terminal send if not accepted.
func (m *Model) sendInputToTab(tab *Tab, input []byte, kind tabEventKind, pasteText, label string) (*Model, tea.Cmd) {
	var cmds []tea.Cmd
	payload := string(input)

	if m.isTabActorReady() {
		queued := m.sendTabEvent(tabEvent{
			tab:         tab,
			workspaceID: m.workspaceID(),
			tabID:       tab.ID,
			kind:        kind,
			input:       input,
			pasteText:   pasteText,
		})
		if queued {
			if kind == tabEventPaste {
				logging.Debug("Pasted %d bytes via bracketed paste through actor", len(pasteText))
			} else {
				logging.Debug("Sent input %q through actor", payload)
			}
			cmds = append(cmds, m.userInputActivityTagCmd(tab))
			return m, common.SafeBatch(cmds...)
		}
	}

	// Direct PTY path fallback
	if _, sent, cmd := m.directSendToTerminal(tab, payload, label); cmd != nil {
		return m, cmd
	} else if !sent {
		return m, nil
	}

	if kind == tabEventPaste {
		logging.Debug("Pasted %d bytes via bracketed paste directly to terminal", len(pasteText))
	} else {
		logging.Debug("Sent input %q directly to terminal", payload)
	}

	now := time.Now()
	cmds = append(cmds, m.noteLocalInput(tab, m.workspaceID(), payload, now))
	cmds = append(cmds, m.userInputActivityTagCmd(tab))
	return m, common.SafeBatch(cmds...)
}

// handlePaste handles bracketed paste messages (tea.PasteMsg).
func (m *Model) handlePaste(msg tea.PasteMsg) (*Model, tea.Cmd) {
	tabs := m.getTabs()
	activeIdx := m.getActiveTabIdx()
	if len(tabs) == 0 || activeIdx >= len(tabs) {
		return m, nil
	}
	tab := tabs[activeIdx]
	if !m.focused {
		return m, nil
	}

	// Dual encoding explanation:
	// - When sending via the Actor path, the actor accepts raw msg.Content and
	//   internally wraps it with bracketed paste codes.
	// - When falling back to the Direct PTY path, we must explicitly wrap msg.Content
	//   with bracketed paste wrappers (\x1b[200~ ... \x1b[201~) ourselves to indicate
	//   a block paste operation to the terminal application.
	payload := "\x1b[200~" + msg.Content + "\x1b[201~"

	return m.sendInputToTab(tab, []byte(payload), tabEventPaste, msg.Content, "Direct paste")
}

// handleKeyPress routes active tab key presses.
func (m *Model) handleKeyPress(msg tea.KeyPressMsg) (*Model, tea.Cmd) {
	tabs := m.getTabs()
	activeIdx := m.getActiveTabIdx()
	if len(tabs) == 0 || activeIdx >= len(tabs) {
		return m, nil
	}
	tab := tabs[activeIdx]

	// Route to active tab based on Kind
	switch tab.Kind {
	case DraftTab:
		if tab.Draft != nil {
			newDraft, cmd := tab.Draft.Update(msg)
			tab.Draft = newDraft
			return m, cmd
		}
		return m, nil
	case TicketViewTab:
		if key.Matches(msg, keyEsc) {
			return m, m.closeCurrentTab()
		}
		return m, nil
	}

	logging.Debug("Center received key: %s, focused=%v, hasTabs=%v, numTabs=%d",
		msg.String(), m.focused, m.hasActiveAgent(), len(tabs))

	// Check if this is Cmd+C (copy command)
	k := msg.Key()
	isCopyKey := k.Mod.Contains(tea.ModSuper) && k.Code == 'c'

	// Handle explicit Cmd+C to copy current selection
	if isCopyKey {
		if m.isTabActorReady() {
			if m.sendTabEvent(tabEvent{
				tab:         tab,
				workspaceID: m.workspaceID(),
				tabID:       tab.ID,
				kind:        tabEventSelectionCopy,
				notifyCopy:  true,
			}) {
				return m, nil
			}
		}
		tab.mu.Lock()
		if tab.Terminal != nil && tab.Terminal.HasSelection() {
			text := tab.Terminal.SelectedText()
			if text != "" {
				if err := common.CopyToClipboard(text); err != nil {
					logging.Error("Failed to copy to clipboard: %v", err)
				} else {
					logging.Info("Cmd+C copied %d chars to clipboard", len(text))
				}
			}
		}
		tab.mu.Unlock()
		return m, nil // Don't forward to terminal, don't clear selection
	}

	// Clear any selection when user types (except Cmd+C which is handled above)
	sent := false
	if m.isTabActorReady() {
		sent = m.sendTabEvent(tabEvent{
			tab:         tab,
			workspaceID: m.workspaceID(),
			tabID:       tab.ID,
			kind:        tabEventSelectionClear,
		})
	}
	if !sent {
		tab.mu.Lock()
		if tab.Terminal != nil {
			tab.Terminal.ClearSelection()
		}
		tab.Selection = common.SelectionState{}
		tab.selectionScroll.Reset()
		tab.mu.Unlock()
	}

	if !m.focused {
		logging.Debug("Center not focused, ignoring key")
		return m, nil
	}

	// When we have an active agent, handle keys
	if m.hasActiveAgent() {
		return m.handleActiveAgentKeyPress(tab, msg)
	}

	return m, nil
}

// handleActiveAgentKeyPress handles key routing when an active agent is present.
func (m *Model) handleActiveAgentKeyPress(tab *Tab, msg tea.KeyPressMsg) (*Model, tea.Cmd) {
	logging.Debug("Has active agent, Agent=%v, Terminal=%v", tab.Agent != nil, tab.Agent != nil && tab.Agent.Terminal != nil)

	tab.mu.Lock()
	dv := tab.DiffViewer
	tab.mu.Unlock()
	if dv != nil {
		return m.handleDiffViewerKeys(tab, msg)
	}

	if tab.Agent != nil && tab.Agent.Terminal != nil {
		return m.handleTerminalKeys(tab, msg)
	}

	return m, nil
}

// handleDiffViewerKeys processes keyboard input for a tab active in DiffViewer mode.
func (m *Model) handleDiffViewerKeys(tab *Tab, msg tea.KeyPressMsg) (*Model, tea.Cmd) {
	// Handle ctrl+w for closing tab
	if key.Matches(msg, keyCloseTab) {
		return m, m.closeCurrentTab()
	}
	// Handle ctrl+n/p for tab switching
	if key.Matches(msg, keyNextTab) {
		before := m.getActiveTabIdx()
		m.nextTab()
		return m, m.tabSelectionChangedCmd(m.getActiveTabIdx() != before)
	}
	if key.Matches(msg, keyPrevTab) {
		before := m.getActiveTabIdx()
		m.prevTab()
		return m, m.tabSelectionChangedCmd(m.getActiveTabIdx() != before)
	}
	// Forward all other keys to diff viewer
	if handled, cmd := m.dispatchDiffInput(tab, msg); handled {
		return m, cmd
	}
	return m, nil
}

// handleTerminalKeys processes keyboard input when a tab contains an active Agent/Terminal.
func (m *Model) handleTerminalKeys(tab *Tab, msg tea.KeyPressMsg) (*Model, tea.Cmd) {
	// Only intercept these specific keys - everything else goes to terminal
	switch {
	case key.Matches(msg, keyNextTab):
		before := m.getActiveTabIdx()
		m.nextTab()
		return m, m.tabSelectionChangedCmd(m.getActiveTabIdx() != before)

	case key.Matches(msg, keyPrevTab):
		before := m.getActiveTabIdx()
		m.prevTab()
		return m, m.tabSelectionChangedCmd(m.getActiveTabIdx() != before)

	case key.Matches(msg, keyCloseTab):
		return m, m.closeCurrentTab()

	case key.Matches(msg, keyTabEscapeHatch):
		before := m.getActiveTabIdx()
		m.nextTab()
		return m, m.tabSelectionChangedCmd(m.getActiveTabIdx() != before)

	case key.Matches(msg, keyEscapeHatch):
		// This is ctrl+[ -> send raw Escape byte (\x1b)
		return m.sendInputToTab(tab, []byte("\x1b"), tabEventSendInput, "", "Escape key")
	}

	// PgUp/PgDown for scrollback (these don't conflict with embedded TUIs)
	switch msg.Key().Code {
	case tea.KeyPgUp:
		if m.isTabActorReady() {
			if m.sendTabEvent(tabEvent{
				tab:         tab,
				workspaceID: m.workspaceID(),
				tabID:       tab.ID,
				kind:        tabEventScrollPage,
				scrollPage:  1,
			}) {
				return m, nil
			}
		}
		tab.mu.Lock()
		if tab.Terminal != nil {
			tab.Terminal.ScrollView(tab.Terminal.Height / 4)
		}
		tab.mu.Unlock()
		return m, nil

	case tea.KeyPgDown:
		if m.isTabActorReady() {
			if m.sendTabEvent(tabEvent{
				tab:         tab,
				workspaceID: m.workspaceID(),
				tabID:       tab.ID,
				kind:        tabEventScrollPage,
				scrollPage:  -1,
			}) {
				return m, nil
			}
		}
		tab.mu.Lock()
		if tab.Terminal != nil {
			tab.Terminal.ScrollView(-tab.Terminal.Height / 4)
		}
		tab.mu.Unlock()
		return m, nil
	}

	// If scrolled, any typing goes back to live
	sent := false
	if m.isTabActorReady() {
		sent = m.sendTabEvent(tabEvent{
			tab:         tab,
			workspaceID: m.workspaceID(),
			tabID:       tab.ID,
			kind:        tabEventScrollToBottom,
		})
	}
	if !sent {
		tab.mu.Lock()
		if tab.Terminal != nil && tab.Terminal.IsScrolled() {
			tab.Terminal.ScrollViewToBottom()
		}
		tab.mu.Unlock()
	}

	// Forward ALL other keys to terminal
	input := common.KeyToBytes(msg)
	if len(input) > 0 {
		return m.sendInputToTab(tab, input, tabEventSendInput, "", "Direct input")
	}
	logging.Debug("keyToBytes returned empty for: %s", msg.String())
	return m, nil
}
