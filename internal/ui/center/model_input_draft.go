package center

import (
	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/tickets"
)

// handleDraftComplete replaces the active DraftTab with an AgentTab (or
// TicketViewTab) in-place at the same tab bar position.
func (m *Model) handleDraftComplete(msg DraftComplete) (*Model, tea.Cmd) {
	m.draft = nil

	// Find the active DraftTab in the current workspace to replace in-place.
	wsID := m.workspaceID()
	tabs := m.tabsByWorkspace[wsID]
	var draftTabIdx = -1
	var draftTab *Tab
	for i, tab := range tabs {
		if tab != nil && !tab.isClosed() && tab.Kind == DraftTab {
			draftTab = tab
			draftTabIdx = i
			break
		}
	}

	if draftTab != nil && msg.Assistant != "" {
		// Replace DraftTab with AgentTab in-place (same index, same TabID).
		// The PTY creation result (handlePtyTabCreated) will fill in the
		// agent, terminal, and session fields since it matches by TabID.
		draftTab.Kind = AgentTab
		draftTab.Draft = nil
		draftTab.Assistant = msg.Assistant
		draftTab.TicketID = msg.TicketID
		draftTab.TicketTitle = msg.TicketTitle
		draftTab.Model = msg.Model
		draftTab.AgentMode = msg.AgentMode
		draftTab.Name = msg.Assistant
		m.noteTabsChanged()

		return m, func() tea.Msg {
			return messages.LaunchAgent{
				Assistant:   msg.Assistant,
				Workspace:   msg.Workspace,
				TicketID:    msg.TicketID,
				TicketTitle: msg.TicketTitle,
				Model:       msg.Model,
				AgentMode:   msg.AgentMode,
				DraftTabID:  string(draftTab.ID),
			}
		}
	}

	if draftTab != nil {
		// No agent to launch — replace DraftTab with TicketViewTab.
		// Extract ticket from draft before clearing it.
		var ticket *tickets.Ticket
		if draftTab.Draft != nil {
			ticket = draftTab.Draft.ticket
		}
		draftTab.Kind = TicketViewTab
		draftTab.Draft = nil
		draftTab.Ticket = ticket
		draftTab.TicketID = msg.TicketID
		draftTab.TicketTitle = msg.TicketTitle
		if draftTab.Name == "" || draftTab.Name == "Draft" {
			if msg.TicketID != "" {
				draftTab.Name = msg.TicketID
			} else {
				draftTab.Name = "Ticket"
			}
		}
		m.noteTabsChanged()
		return m, func() tea.Msg {
			return messages.TabCreated{Index: draftTabIdx, Name: draftTab.Name}
		}
	}

	// Fallback: no DraftTab found — launch agent as a new tab.
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
}
