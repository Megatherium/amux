// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// Package tickets provides domain types for bead issue tracking and ticket-aware launching.
package tickets

import (
	"context"
	"time"
)

// TicketStore abstracts ticket retrieval from the underlying data source.
type TicketStore interface {
	ListTickets(ctx context.Context, filter TicketFilter) ([]Ticket, error)
	LatestUpdate(ctx context.Context) (time.Time, error)
}

// TicketFilter controls which tickets are returned by ListTickets.
type TicketFilter struct {
	Status    string
	IssueType string
	Limit     int
	Search    string
}
