package app

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/app/orchestrator"
	"github.com/andyrewlee/amux/internal/logging"
	"github.com/andyrewlee/amux/internal/messages"
)

// syncActiveWorkspacesToDashboard syncs the active workspace state from center to dashboard.
// This ensures the dashboard has current data for spinner state decisions.
func (a *App) syncActiveWorkspacesToDashboard() {
	if a.ui.dashboard == nil {
		return
	}
	activeWorkspaces := make(map[string]bool)
	if !a.tmuxActivitySettled {
		a.ui.dashboard.SetActiveWorkspaces(activeWorkspaces)
		return
	}
	for wsID := range a.tmuxActiveWorkspaceIDs {
		activeWorkspaces[wsID] = true
	}
	a.ui.dashboard.SetActiveWorkspaces(activeWorkspaces)
}

// handleKeyPress handles keyboard input
func (a *App) handleKeyPress(msg tea.KeyPressMsg) tea.Cmd {
	// Dismiss error on any key
	if a.err != nil {
		a.err = nil
		return nil
	}

	// 1. Handle prefix key
	if a.isPrefixKey(msg) {
		if a.oc().Prefix.Active {
			if len(a.oc().Prefix.Sequence) == 0 {
				// Prefix + Prefix = send literal prefix key to terminal.
				a.sendPrefixToTerminal()
				a.exitPrefix()
				return nil
			}
			// Restart narrowing from the root command list.
			a.oc().Prefix.Sequence = nil
			return a.oc().Prefix.RefreshTimeout()
		}
		// Enter prefix mode
		return a.enterPrefix()
	}

	// 2. If prefix is active, handle mux commands
	if a.oc().Prefix.Active {
		// Esc cancels prefix mode without forwarding
		code := msg.Key().Code
		if code == tea.KeyEsc || code == tea.KeyEscape {
			a.exitPrefix()
			return nil
		}

		status, cmd := a.handlePrefixCommand(msg)
		switch status {
		case orchestrator.PrefixMatchComplete:
			a.exitPrefix()
			return cmd
		case orchestrator.PrefixMatchPartial:
			// Keep prefix mode open while the sequence narrows.
			return a.oc().Prefix.RefreshTimeout()
		}
		// Unknown key in prefix mode: exit prefix and pass through
		a.exitPrefix()
		// Fall through to normal handling below
	}

	// 3. Passthrough mode - route keys to focused pane
	// Handle button navigation when center pane is focused and showing welcome/workspace info (no tabs, no draft)
	if a.oc().Focus.FocusedPane == messages.PaneCenter && !a.ui.center.HasTabs() && !a.ui.center.HasDraft() {
		maxIndex := a.centerButtonCount() - 1
		switch {
		case key.Matches(msg, a.keymap.Left), key.Matches(msg, a.keymap.Up):
			if a.ui.centerBtnFocused {
				if a.ui.centerBtnIndex > 0 {
					a.ui.centerBtnIndex--
				} else {
					a.ui.centerBtnFocused = false
				}
			} else {
				// Enter from the right/bottom - focus last button
				a.ui.centerBtnFocused = true
				a.ui.centerBtnIndex = maxIndex
			}
			return nil
		case key.Matches(msg, a.keymap.Right), key.Matches(msg, a.keymap.Down):
			if a.ui.centerBtnFocused {
				if a.ui.centerBtnIndex < maxIndex {
					a.ui.centerBtnIndex++
				} else {
					a.ui.centerBtnFocused = false
				}
			} else {
				// Enter from the left/top - focus first button
				a.ui.centerBtnFocused = true
				a.ui.centerBtnIndex = 0
			}
			return nil
		case key.Matches(msg, a.keymap.Enter):
			if a.ui.centerBtnFocused {
				return a.activateCenterButton()
			}
		}
	}

	// Route to focused pane
	switch a.oc().Focus.FocusedPane {
	case messages.PaneDashboard:
		newDashboard, cmd := a.ui.dashboard.Update(msg)
		a.ui.dashboard = newDashboard
		return cmd
	case messages.PaneCenter:
		newCenter, cmd := a.ui.center.Update(msg)
		a.ui.center = newCenter
		return cmd
	case messages.PaneSidebar:
		newSidebar, cmd := a.ui.sidebar.Update(msg)
		a.ui.sidebar = newSidebar
		return cmd
	case messages.PaneSidebarTerminal:
		newSidebarTerminal, cmd := a.ui.sidebarTerminal.Update(msg)
		a.ui.sidebarTerminal = newSidebarTerminal
		return cmd
	}
	return nil
}

func (a *App) handleKeyboardEnhancements(msg tea.KeyboardEnhancementsMsg) {
	a.keyboardEnhancements = msg
	logging.Info("Keyboard enhancements: disambiguation=%t event_types=%t", msg.SupportsKeyDisambiguation(), msg.SupportsEventTypes())
}

func (a *App) handleWindowSize(msg tea.WindowSizeMsg) {
	a.ui.width = msg.Width
	a.ui.height = msg.Height
	a.ui.ready = true
	a.ui.layout.Resize(msg.Width, msg.Height)
	a.updateLayout()
}

func (a *App) handlePaste(msg tea.PasteMsg) tea.Cmd {
	switch a.oc().Focus.FocusedPane {
	case messages.PaneCenter:
		newCenter, cmd := a.ui.center.Update(msg)
		a.ui.center = newCenter
		return cmd
	case messages.PaneSidebarTerminal:
		newTerm, cmd := a.ui.sidebarTerminal.Update(msg)
		a.ui.sidebarTerminal = newTerm
		return cmd
	}
	return nil
}

func (a *App) handlePrefixTimeout(msg orchestrator.PrefixTimeoutMsg) {
	if msg.Token == a.oc().Prefix.Token && a.oc().Prefix.Active {
		a.exitPrefix()
	}
}

// centerButtonCount returns the number of buttons shown on the current center screen
func (a *App) centerButtonCount() int {
	if a.showWelcome {
		return 2 // [Add project], [Settings]
	}
	if a.activeWorkspace != nil {
		if a.hasTicketService() {
			return 2 // [New Agent with Ticket], [New Agent]
		}
		return 1 // [New Agent]
	}
	return 0
}

// activateCenterButton activates the currently focused center button
func (a *App) activateCenterButton() tea.Cmd {
	if a.showWelcome {
		switch a.ui.centerBtnIndex {
		case 0:
			return func() tea.Msg { return messages.ShowAddProjectDialog{} }
		case 1:
			return func() tea.Msg { return messages.ShowSettingsDialog{} }
		}
	} else if a.activeWorkspace != nil {
		if a.hasTicketService() {
			switch a.ui.centerBtnIndex {
			case 0:
				return func() tea.Msg { return messages.ShowSelectTicketDialog{} }
			case 1:
				return func() tea.Msg { return messages.ShowSelectAssistantDialog{} }
			}
		}
		return func() tea.Msg { return messages.ShowSelectAssistantDialog{} }
	}
	return nil
}
