// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package fake

import (
	"context"
	"strings"
	"time"

	"github.com/andyrewlee/amux/internal/tickets"
)

type TicketStore struct {
	Tickets []tickets.Ticket
}

var _ tickets.TicketStore = (*TicketStore)(nil)

func (s *TicketStore) ListTickets(_ context.Context, filter tickets.TicketFilter) ([]tickets.Ticket, error) {
	var results []tickets.Ticket
	for i := range s.Tickets {
		t := &s.Tickets[i]
		if filter.Status != "" && t.Status != filter.Status {
			continue
		}
		if filter.IssueType != "" && t.IssueType != filter.IssueType {
			continue
		}
		if filter.Search != "" && !strings.Contains(strings.ToLower(t.Title), strings.ToLower(filter.Search)) {
			continue
		}
		results = append(results, *t)
		if filter.Limit > 0 && len(results) >= filter.Limit {
			break
		}
	}
	return results, nil
}

func (s *TicketStore) LatestUpdate(_ context.Context) (time.Time, error) {
	var latest time.Time
	for _, t := range s.Tickets {
		if t.UpdatedAt.After(latest) {
			latest = t.UpdatedAt
		}
	}
	return latest, nil
}

func NewWithSampleData() *TicketStore {
	now := time.Now()
	return &TicketStore{
		Tickets: []tickets.Ticket{
			{ID: "bmx-001", Title: "Bootstrap Go module", Status: "closed", Priority: 1, IssueType: "task", CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now.Add(-24 * time.Hour)},
			{ID: "bmx-002", Title: "Define core domain types", Status: "open", Priority: 1, IssueType: "task", CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now},
			{ID: "bmx-003", Title: "Implement TicketStore backend", Status: "open", Priority: 1, IssueType: "task", CreatedAt: now.Add(-12 * time.Hour), UpdatedAt: now},
			{ID: "bmx-004", Title: "Build TUI skeleton", Status: "open", Priority: 1, IssueType: "feature", CreatedAt: now.Add(-6 * time.Hour), UpdatedAt: now},
			{ID: "bmx-005", Title: "Implement tmux launcher", Status: "open", Priority: 2, IssueType: "task", CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now},
		},
	}
}
