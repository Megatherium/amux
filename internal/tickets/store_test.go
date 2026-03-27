// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package tickets

import (
	"context"
	"testing"
	"time"
)

type fakeTicketStore struct {
	tickets []Ticket
	update  time.Time
}

func (f *fakeTicketStore) ListTickets(ctx context.Context, filter TicketFilter) ([]Ticket, error) {
	return f.tickets, nil
}

func (f *fakeTicketStore) LatestUpdate(ctx context.Context) (time.Time, error) {
	return f.update, nil
}

func TestTicketStoreInterface(t *testing.T) {
	var store TicketStore = &fakeTicketStore{
		tickets: []Ticket{
			{ID: "bmx-1", Title: "Test ticket"},
		},
		update: time.Now(),
	}

	ctx := context.Background()

	tickets, err := store.ListTickets(ctx, TicketFilter{})
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(tickets) != 1 {
		t.Fatalf("expected 1 ticket, got %d", len(tickets))
	}
	if tickets[0].ID != "bmx-1" {
		t.Errorf("ticket ID = %q, want %q", tickets[0].ID, "bmx-1")
	}

	update, err := store.LatestUpdate(ctx)
	if err != nil {
		t.Fatalf("LatestUpdate() error = %v", err)
	}
	if update.IsZero() {
		t.Error("LatestUpdate() returned zero time")
	}
}

func TestTicketFilterFields(t *testing.T) {
	filter := TicketFilter{
		Status:    "open",
		IssueType: "bug",
		Limit:     50,
		Search:    "login",
	}

	if filter.Status != "open" {
		t.Errorf("Status = %q, want %q", filter.Status, "open")
	}
	if filter.IssueType != "bug" {
		t.Errorf("IssueType = %q, want %q", filter.IssueType, "bug")
	}
	if filter.Limit != 50 {
		t.Errorf("Limit = %d, want %d", filter.Limit, 50)
	}
	if filter.Search != "login" {
		t.Errorf("Search = %q, want %q", filter.Search, "login")
	}
}

func TestTicketFilterZeroValue(t *testing.T) {
	var filter TicketFilter

	if filter.Status != "" {
		t.Errorf("Status = %q, want empty", filter.Status)
	}
	if filter.IssueType != "" {
		t.Errorf("IssueType = %q, want empty", filter.IssueType)
	}
	if filter.Limit != 0 {
		t.Errorf("Limit = %d, want 0", filter.Limit)
	}
	if filter.Search != "" {
		t.Errorf("Search = %q, want empty", filter.Search)
	}
}
