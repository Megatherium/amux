// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package fake

import (
	"context"
	"testing"
	"time"

	"github.com/andyrewlee/amux/internal/tickets"
)

func TestListTicketsNoFilter(t *testing.T) {
	store := NewWithSampleData()
	ctx := context.Background()

	tickets, err := store.ListTickets(ctx, tickets.TicketFilter{})
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(tickets) != 5 {
		t.Errorf("len(tickets) = %d, want 5", len(tickets))
	}
}

func TestListTicketsStatusFilter(t *testing.T) {
	store := NewWithSampleData()
	ctx := context.Background()

	openTickets, err := store.ListTickets(ctx, tickets.TicketFilter{Status: "open"})
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(openTickets) != 4 {
		t.Errorf("len(openTickets) = %d, want 4", len(openTickets))
	}

	closedTickets, err := store.ListTickets(ctx, tickets.TicketFilter{Status: "closed"})
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(closedTickets) != 1 {
		t.Errorf("len(closedTickets) = %d, want 1", len(closedTickets))
	}
}

func TestListTicketsIssueTypeFilter(t *testing.T) {
	store := NewWithSampleData()
	ctx := context.Background()

	featureTickets, err := store.ListTickets(ctx, tickets.TicketFilter{IssueType: "feature"})
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(featureTickets) != 1 {
		t.Errorf("len(featureTickets) = %d, want 1", len(featureTickets))
	}
	if featureTickets[0].ID != "bmx-004" {
		t.Errorf("featureTickets[0].ID = %q, want %q", featureTickets[0].ID, "bmx-004")
	}
}

func TestListTicketsSearchFilter(t *testing.T) {
	store := NewWithSampleData()
	ctx := context.Background()

	results, err := store.ListTickets(ctx, tickets.TicketFilter{Search: "ti"})
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("len(results) = %d, want 1 (only TicketStore contains 'ti' - TUI is 'tui' not 'ti')", len(results))
	}

	exact, err := store.ListTickets(ctx, tickets.TicketFilter{Search: "Bootstrap"})
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(exact) != 1 {
		t.Errorf("len(exact) = %d, want 1", len(exact))
	}
	if exact[0].Title != "Bootstrap Go module" {
		t.Errorf("exact[0].Title = %q, want %q", exact[0].Title, "Bootstrap Go module")
	}
}

func TestListTicketsSearchCaseInsensitive(t *testing.T) {
	store := NewWithSampleData()
	ctx := context.Background()

	results, err := store.ListTickets(ctx, tickets.TicketFilter{Search: "BOOTSTRAP"})
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("len(results) = %d, want 1", len(results))
	}
}

func TestListTicketsLimit(t *testing.T) {
	store := NewWithSampleData()
	ctx := context.Background()

	results, err := store.ListTickets(ctx, tickets.TicketFilter{Limit: 2})
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("len(results) = %d, want 2", len(results))
	}
}

func TestListTicketsLimitZero(t *testing.T) {
	store := NewWithSampleData()
	ctx := context.Background()

	results, err := store.ListTickets(ctx, tickets.TicketFilter{Limit: 0})
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(results) != 5 {
		t.Errorf("len(results) = %d, want 5 (0 means no limit)", len(results))
	}
}

func TestListTicketsCombinedFilters(t *testing.T) {
	store := NewWithSampleData()
	ctx := context.Background()

	results, err := store.ListTickets(ctx, tickets.TicketFilter{Status: "open", IssueType: "task", Limit: 10})
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(results) != 3 {
		t.Errorf("len(results) = %d, want 3 open tasks", len(results))
	}
}

func TestLatestUpdateNoTickets(t *testing.T) {
	store := &TicketStore{Tickets: []tickets.Ticket{}}
	ctx := context.Background()

	update, err := store.LatestUpdate(ctx)
	if err != nil {
		t.Fatalf("LatestUpdate() error = %v", err)
	}
	if !update.IsZero() {
		t.Errorf("LatestUpdate() = %v, want zero time", update)
	}
}

func TestLatestUpdateWithTickets(t *testing.T) {
	store := NewWithSampleData()
	ctx := context.Background()

	update, err := store.LatestUpdate(ctx)
	if err != nil {
		t.Fatalf("LatestUpdate() error = %v", err)
	}
	if update.IsZero() {
		t.Error("LatestUpdate() returned zero time")
	}
}

func TestLatestUpdateReturnsMaxTimestamp(t *testing.T) {
	now := time.Now()
	store := &TicketStore{
		Tickets: []tickets.Ticket{
			{ID: "bmx-1", Title: "Old ticket", UpdatedAt: now.Add(-48 * time.Hour)},
			{ID: "bmx-2", Title: "New ticket", UpdatedAt: now.Add(-1 * time.Hour)},
			{ID: "bmx-3", Title: "Middle ticket", UpdatedAt: now.Add(-24 * time.Hour)},
		},
	}
	ctx := context.Background()

	update, err := store.LatestUpdate(ctx)
	if err != nil {
		t.Fatalf("LatestUpdate() error = %v", err)
	}
	expected := now.Add(-1 * time.Hour)
	if !update.Equal(expected) {
		t.Errorf("LatestUpdate() = %v, want %v", update, expected)
	}
}

func TestNewWithSampleData(t *testing.T) {
	store := NewWithSampleData()

	if len(store.Tickets) != 5 {
		t.Errorf("len(store.Tickets) = %d, want 5", len(store.Tickets))
	}

	ids := make(map[string]bool)
	for _, ticket := range store.Tickets {
		if ids[ticket.ID] {
			t.Errorf("duplicate ticket ID: %s", ticket.ID)
		}
		ids[ticket.ID] = true
	}
}
