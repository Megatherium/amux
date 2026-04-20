// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package messages

import (
	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/tickets"
)

// TicketsLoadedMsg is sent when tickets have been loaded or refreshed from the beads store.
type TicketsLoadedMsg struct {
	ProjectPath string
	Tickets     []tickets.Ticket
}

// TicketSelectedMsg is sent when the user selects a ticket for context.
type TicketSelectedMsg struct {
	Ticket  *tickets.Ticket
	Project *data.Project
}

// TicketPreviewMsg is sent when the cursor hovers over a ticket row in the
// dashboard. It signals that ticket info should be shown in the center pane
// (when no agent is active) or in the sidebar ticket tab (when an agent is
// active). Ticket is nil when the cursor moves away from a ticket row.
type TicketPreviewMsg struct {
	Ticket  *tickets.Ticket
	Project *data.Project
}

// TicketRefreshMsg requests a refresh of the ticket list.
type TicketRefreshMsg struct{}

// DiscoveryLoadedMsg is sent when the model/agent discovery registry has been loaded.
type DiscoveryLoadedMsg struct{}

// ModelSelectedMsg is sent when the user selects a model for the launch.
type ModelSelectedMsg struct {
	Model tickets.ModelContext
}

// AgentModeSelectedMsg is sent when the user selects an agent mode (e.g., auto-approve).
type AgentModeSelectedMsg struct {
	AgentMode string
}

// LaunchReadyMsg is sent when the launch specification is ready for execution.
type LaunchReadyMsg struct {
	LaunchSpec *tickets.LaunchSpec
}
