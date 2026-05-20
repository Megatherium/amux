package center

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/logging"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/perf"
	"github.com/andyrewlee/amux/internal/ui/common"
)

// directSendToTerminal sends data directly to the terminal, handling errors.
// Returns whether data was actually sent and an optional command for failures.
func (m *Model) directSendToTerminal(tab *Tab, data, label string) (*Model, bool, tea.Cmd) {
	if tab.Agent == nil || tab.Agent.Terminal == nil {
		return m, false, nil
	}
	if err := tab.Agent.Terminal.SendString(data); err != nil {
		logging.Warn("%s failed for tab %s: %v", label, tab.ID, err)
		tab.mu.Lock()
		tab.Running = false
		tab.Detached = true
		tab.mu.Unlock()
		wsID := m.workspaceID()
		return m, false, func() tea.Msg {
			return TabInputFailed{TabID: tab.ID, WorkspaceID: wsID, Err: err}
		}
	}
	return m, true, nil
}

// noteLocalInput records local typing/editing activity for activity suppression
// and chat cursor tracking, and schedules a redraw for timer-driven cursor
// state changes.
func (m *Model) noteLocalInput(tab *Tab, workspaceID, data string, now time.Time) tea.Cmd {
	if tab == nil {
		return nil
	}
	recordLocalInputEchoWindow(tab, data, now)
	return m.scheduleChatCursorRefresh(tab, workspaceID, now)
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	defer perf.Time("center_update")()
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		tabs := m.getTabs()
		activeIdx := m.getActiveTabIdx()
		if len(tabs) > 0 && activeIdx < len(tabs) && tabs[activeIdx].Kind == DraftTab {
			return m, nil
		}
		return m.updateMouseClick(msg)

	case tea.MouseMotionMsg:
		return m.updateMouseMotion(msg)

	case tea.MouseReleaseMsg:
		return m.updateMouseRelease(msg)

	case tea.MouseWheelMsg:
		tabs := m.getTabs()
		activeIdx := m.getActiveTabIdx()
		if len(tabs) > 0 && activeIdx < len(tabs) && tabs[activeIdx].Kind == DraftTab {
			return m, nil
		}
		return m.updateMouseWheel(msg)

	case tea.PasteMsg:
		return m.handlePaste(msg)

	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)

	case DraftComplete:
		m.draft = nil
		return m, func() tea.Msg {
			return messages.LaunchAgent{
				Assistant:   msg.Assistant,
				Workspace:   msg.Workspace,
				TicketID:    msg.TicketID,
				TicketTitle: msg.TicketTitle,
				Model:       msg.Model,
				AgentMode:   msg.AgentMode,
			}
		}

	case DraftCancelled:
		m.draft = nil
		tabs := m.getTabs()
		activeIdx := m.getActiveTabIdx()
		if activeIdx < len(tabs) && tabs[activeIdx].Kind == DraftTab {
			return m, m.closeTabAt(activeIdx)
		}
		return m, nil

	case messages.LaunchAgent:
		return m.updateLaunchAgent(msg)

	case messages.OpenFileInVim:
		return m.updateOpenFileInVim(msg)

	case ptyTabCreateResult:
		return m.updatePtyTabCreateResult(msg)

	case ptyTabReattachResult:
		return m.updatePtyTabReattachResult(msg)

	case ptyTabReattachFailed:
		return m.updatePtyTabReattachFailed(msg)

	case messages.TabSessionStatus:
		return m.updateTabSessionStatus(msg)

	case messages.OpenDiff:
		return m.updateOpenDiff(msg)

	case messages.WorkspaceDeleted:
		return m.updateWorkspaceDeleted(msg)

	case tabSelectionResult:
		return m.updateTabSelectionResult(msg)

	case selectionTickRequest:
		return m.updateSelectionTickRequest(msg)

	case tabDiffCmd:
		return m.updateTabDiffCmd(msg)

	case tabActorSignal:
		switch msg.kind {
		case "redraw":
			return m, nil
		case "started":
			m.tabActorRunning = true
			m.tabActorLastBeat = time.Now()
			return m, nil
		case "heartbeat":
			m.tabActorLastBeat = time.Now()
			return m, nil
		}

	case PTYOutput:
		cmd := m.updatePTYOutput(msg)
		cmds = append(cmds, cmd)

	case PTYFlush:
		cmd := m.updatePTYFlush(msg)
		cmds = append(cmds, cmd)

	case PTYCursorRefresh:
		cmd := m.updatePTYCursorRefresh(msg)
		cmds = append(cmds, cmd)

	case PTYStopped:
		cmd := m.updatePTYStopped(msg)
		cmds = append(cmds, cmd)

	case PTYRestart:
		cmd := m.updatePTYRestart(msg)
		cmds = append(cmds, cmd)

	case selectionScrollTick:
		cmd := m.updateSelectionScrollTick(msg)
		cmds = append(cmds, cmd)

	default:
		// Forward unknown messages to active viewer if one exists
		tabs := m.getTabs()
		activeIdx := m.getActiveTabIdx()
		if len(tabs) > 0 && activeIdx < len(tabs) {
			tab := tabs[activeIdx]
			if handled, cmd := m.dispatchDiffInput(tab, msg); handled {
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
	}

	return m, common.SafeBatch(cmds...)
}
