package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/ui/center"
	"github.com/andyrewlee/amux/internal/ui/dashboard"
)

// handlePTYMessages handles PTY-related messages for center pane.
func (a *App) handlePTYMessages(msg tea.Msg) tea.Cmd {
	newCenter, cmd := a.ui.center.Update(msg)
	a.ui.center = newCenter
	return cmd
}

// handleSidebarPTYMessages handles PTY-related messages for sidebar terminal.
func (a *App) handleSidebarPTYMessages(msg tea.Msg) tea.Cmd {
	newSidebarTerminal, cmd := a.ui.sidebarTerminal.Update(msg)
	a.ui.sidebarTerminal = newSidebarTerminal
	return cmd
}

// handleTabInputFailed handles the TabInputFailed message.
func (a *App) handleTabInputFailed(msg center.TabInputFailed) []tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, a.ui.toast.ShowWarning("Session disconnected - scroll history preserved"))
	if msg.WorkspaceID != "" {
		if cmd := a.ui.center.DetachTabByID(msg.WorkspaceID, msg.TabID); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if cmd := a.persistActiveWorkspaceTabs(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// handleSpinnerTick handles the SpinnerTickMsg from dashboard.
func (a *App) handleSpinnerTick(msg dashboard.SpinnerTickMsg) []tea.Cmd {
	var cmds []tea.Cmd
	a.syncActiveWorkspacesToDashboard()
	a.ui.center.TickSpinner()
	newDashboard, cmd := a.ui.dashboard.Update(msg)
	a.ui.dashboard = newDashboard
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	if startCmd := a.ui.dashboard.StartSpinnerIfNeeded(); startCmd != nil {
		cmds = append(cmds, startCmd)
	}
	return cmds
}

// handlePTYWatchdogTick handles the PTYWatchdogTick message.
func (a *App) handlePTYWatchdogTick() []tea.Cmd {
	var cmds []tea.Cmd
	if a.ui.center != nil {
		if cmd := a.ui.center.StartPTYReaders(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if a.ui.sidebarTerminal != nil {
		if cmd := a.ui.sidebarTerminal.StartPTYReaders(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	// Keep dashboard "working" state accurate even when agents go idle.
	a.syncActiveWorkspacesToDashboard()
	cmds = append(cmds, a.startPTYWatchdog())
	return cmds
}
