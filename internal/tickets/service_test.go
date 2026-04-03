// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package tickets

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/andyrewlee/amux/internal/discovery"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// stubStore implements TicketStore for testing.
type stubStore struct {
	tickets []Ticket
	update  time.Time
	err     error
}

func (s *stubStore) ListTickets(_ context.Context, _ TicketFilter) ([]Ticket, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.tickets, nil
}

func (s *stubStore) LatestUpdate(_ context.Context) (time.Time, error) {
	if s.err != nil {
		return time.Time{}, s.err
	}
	return s.update, nil
}

// stubRegistry wraps a real discovery.Registry with pre-loaded providers.
func newStubRegistry(providers map[string]discovery.Provider) *discovery.Registry {
	reg, _ := discovery.NewRegistry("")
	reg.SetProviders(providers)
	return reg
}

func sampleProviders() map[string]discovery.Provider {
	return map[string]discovery.Provider{
		"openai": {
			ID:   "openai",
			Name: "OpenAI",
			Env:  []string{"OPENAI_API_KEY"},
			Models: map[string]discovery.Model{
				"gpt-4o": {ID: "gpt-4o", Name: "GPT-4o"},
			},
		},
		"anthropic": {
			ID:   "anthropic",
			Name: "Anthropic",
			Env:  []string{"ANTHROPIC_API_KEY"},
			Models: map[string]discovery.Model{
				"claude-sonnet-4": {ID: "claude-sonnet-4", Name: "Claude Sonnet 4"},
			},
		},
	}
}

func makeTicket(id, title, status string) Ticket {
	now := time.Now()
	return Ticket{
		ID: id, Title: title, Status: status,
		Priority: 2, IssueType: "task",
		CreatedAt: now, UpdatedAt: now,
	}
}

func makeService(store TicketStore, reg *discovery.Registry) *TicketService {
	return NewTicketService(store, reg, NewRenderer())
}

// ---------------------------------------------------------------------------
// NewTicketService
// ---------------------------------------------------------------------------

func TestNewTicketService_NilDependencies(t *testing.T) {
	svc := NewTicketService(nil, nil, nil)
	if svc == nil {
		t.Fatal("NewTicketService should return non-nil even with nil deps")
	}
}

// ---------------------------------------------------------------------------
// ListTickets
// ---------------------------------------------------------------------------

func TestTicketService_ListTickets_DelegatesToStore(t *testing.T) {
	tickets := []Ticket{
		makeTicket("bmx-1", "First", "open"),
		makeTicket("bmx-2", "Second", "closed"),
	}
	svc := makeService(&stubStore{tickets: tickets}, nil)

	got, err := svc.ListTickets(context.Background(), TicketFilter{Status: "open"})
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
}

func TestTicketService_ListTickets_StoreError(t *testing.T) {
	svc := makeService(&stubStore{err: errors.New("db down")}, nil)

	_, err := svc.ListTickets(context.Background(), TicketFilter{})
	if err == nil {
		t.Fatal("expected error from store failure")
	}
	if !strings.Contains(err.Error(), "db down") {
		t.Errorf("error = %v, want containing 'db down'", err)
	}
}

func TestTicketService_ListTickets_NilStore(t *testing.T) {
	svc := NewTicketService(nil, nil, NewRenderer())

	_, err := svc.ListTickets(context.Background(), TicketFilter{})
	if err == nil {
		t.Fatal("expected error for nil store")
	}
	if !strings.Contains(err.Error(), "store not configured") {
		t.Errorf("error = %v, want 'store not configured'", err)
	}
}

// ---------------------------------------------------------------------------
// LatestUpdate
// ---------------------------------------------------------------------------

func TestTicketService_LatestUpdate_DelegatesToStore(t *testing.T) {
	ts := time.Now()
	svc := makeService(&stubStore{update: ts}, nil)

	got, err := svc.LatestUpdate(context.Background())
	if err != nil {
		t.Fatalf("LatestUpdate() error = %v", err)
	}
	if !got.Equal(ts) {
		t.Errorf("LatestUpdate() = %v, want %v", got, ts)
	}
}

func TestTicketService_LatestUpdate_NilStore(t *testing.T) {
	svc := NewTicketService(nil, nil, NewRenderer())

	_, err := svc.LatestUpdate(context.Background())
	if err == nil {
		t.Fatal("expected error for nil store")
	}
}

// ---------------------------------------------------------------------------
// ResolveModels
// ---------------------------------------------------------------------------

func TestTicketService_ResolveModels_DiscoverActive(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	reg := newStubRegistry(sampleProviders())
	svc := makeService(nil, reg)

	models := svc.ResolveModels([]string{"discover:active"})

	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1 (only openai has env set)", len(models))
	}
	if models[0] != "openai/gpt-4o" {
		t.Errorf("models[0] = %q, want %q", models[0], "openai/gpt-4o")
	}
}

func TestTicketService_ResolveModels_ProviderPrefix(t *testing.T) {
	reg := newStubRegistry(sampleProviders())
	svc := makeService(nil, reg)

	models := svc.ResolveModels([]string{"provider:anthropic"})

	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}
	if models[0] != "anthropic/claude-sonnet-4" {
		t.Errorf("models[0] = %q, want %q", models[0], "anthropic/claude-sonnet-4")
	}
}

func TestTicketService_ResolveModels_LiteralPassthrough(t *testing.T) {
	svc := makeService(nil, newStubRegistry(nil))

	models := svc.ResolveModels([]string{"gpt-4o", "claude-sonnet"})

	if len(models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(models))
	}
	// sorted
	if models[0] != "claude-sonnet" {
		t.Errorf("models[0] = %q, want %q", models[0], "claude-sonnet")
	}
	if models[1] != "gpt-4o" {
		t.Errorf("models[1] = %q, want %q", models[1], "gpt-4o")
	}
}

func TestTicketService_ResolveModels_MixedKeywords(t *testing.T) {
	reg := newStubRegistry(sampleProviders())
	svc := makeService(nil, reg)

	models := svc.ResolveModels([]string{"provider:openai", "my-custom-model"})

	if len(models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(models))
	}
	// sorted: "my-custom-model" < "openai/gpt-4o"
	if models[0] != "my-custom-model" {
		t.Errorf("models[0] = %q, want %q", models[0], "my-custom-model")
	}
	if models[1] != "openai/gpt-4o" {
		t.Errorf("models[1] = %q, want %q", models[1], "openai/gpt-4o")
	}
}

func TestTicketService_ResolveModels_Deduplicates(t *testing.T) {
	reg := newStubRegistry(sampleProviders())
	svc := makeService(nil, reg)

	models := svc.ResolveModels([]string{"provider:openai", "provider:openai"})

	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1 (deduped)", len(models))
	}
}

func TestTicketService_ResolveModels_UnknownProviderReturnsNothing(t *testing.T) {
	reg := newStubRegistry(sampleProviders())
	svc := makeService(nil, reg)

	models := svc.ResolveModels([]string{"provider:nonexistent"})

	if len(models) != 0 {
		t.Fatalf("len(models) = %d, want 0 for unknown provider", len(models))
	}
}

func TestTicketService_ResolveModels_EmptyInput(t *testing.T) {
	reg := newStubRegistry(sampleProviders())
	svc := makeService(nil, reg)

	models := svc.ResolveModels(nil)
	if models != nil {
		t.Errorf("expected nil for empty input, got %v", models)
	}

	models = svc.ResolveModels([]string{})
	if models != nil {
		t.Errorf("expected nil for empty slice, got %v", models)
	}
}

func TestTicketService_ResolveModels_NilRegistry(t *testing.T) {
	svc := NewTicketService(nil, nil, nil)

	models := svc.ResolveModels([]string{"gpt-4o"})
	if models != nil {
		t.Errorf("expected nil with nil registry, got %v", models)
	}
}

func TestTicketService_ResolveModels_Sorted(t *testing.T) {
	svc := makeService(nil, newStubRegistry(nil))

	models := svc.ResolveModels([]string{"z-model", "a-model", "m-model"})

	if models[0] != "a-model" || models[1] != "m-model" || models[2] != "z-model" {
		t.Errorf("expected sorted output, got %v", models)
	}
}

// ---------------------------------------------------------------------------
// RenderLaunch
// ---------------------------------------------------------------------------

func TestTicketService_RenderLaunch_HappyPath(t *testing.T) {
	svc := makeService(nil, nil)
	sel := Selection{
		Ticket:    makeTicket("bmx-42", "Fix thing", "open"),
		Assistant: "opencode",
		Model:     "claude-sonnet",
		Agent:     "coder",
	}

	spec, err := svc.RenderLaunch(sel, "/work",
		"opencode --model {{.Model}} --agent {{.Agent}} --ticket {{.TicketID}}",
		"Work on {{.TicketID}}: {{.TicketTitle}}",
	)
	if err != nil {
		t.Fatalf("RenderLaunch() error = %v", err)
	}

	wantCmd := "opencode --model claude-sonnet --agent coder --ticket bmx-42"
	if spec.RenderedCommand != wantCmd {
		t.Errorf("command = %q, want %q", spec.RenderedCommand, wantCmd)
	}

	wantPrompt := "Work on bmx-42: Fix thing"
	if spec.RenderedPrompt != wantPrompt {
		t.Errorf("prompt = %q, want %q", spec.RenderedPrompt, wantPrompt)
	}

	if spec.LauncherID != "bmx-42" {
		t.Errorf("LauncherID = %q, want %q", spec.LauncherID, "bmx-42")
	}
	if spec.WorkDir != "/work" {
		t.Errorf("WorkDir = %q, want %q", spec.WorkDir, "/work")
	}
}

func TestTicketService_RenderLaunch_NoPrompt(t *testing.T) {
	svc := makeService(nil, nil)
	sel := Selection{
		Ticket:    makeTicket("bmx-1", "Test", "open"),
		Assistant: "test",
		Model:     "model",
		Agent:     "agent",
	}

	spec, err := svc.RenderLaunch(sel, "",
		"echo {{.TicketID}}",
		"",
	)
	if err != nil {
		t.Fatalf("RenderLaunch() error = %v", err)
	}
	if spec.RenderedCommand != "echo bmx-1" {
		t.Errorf("command = %q, want %q", spec.RenderedCommand, "echo bmx-1")
	}
	if spec.RenderedPrompt != "" {
		t.Errorf("prompt = %q, want empty", spec.RenderedPrompt)
	}
}

func TestTicketService_RenderLaunch_BadTemplate(t *testing.T) {
	svc := makeService(nil, nil)
	sel := Selection{
		Ticket:    makeTicket("bmx-1", "Test", "open"),
		Assistant: "bad",
	}

	_, err := svc.RenderLaunch(sel, "", "{{.Undefined", "")
	if err == nil {
		t.Fatal("expected error for bad template")
	}
	if !strings.Contains(err.Error(), "ticket service:") {
		t.Errorf("error should be wrapped by service, got: %v", err)
	}
}

func TestTicketService_RenderLaunch_NilRenderer(t *testing.T) {
	svc := NewTicketService(nil, nil, nil)
	sel := Selection{Ticket: makeTicket("bmx-1", "T", "open"), Assistant: "x"}

	_, err := svc.RenderLaunch(sel, "", "echo ok", "")
	if err == nil {
		t.Fatal("expected error for nil renderer")
	}
	if !strings.Contains(err.Error(), "renderer not configured") {
		t.Errorf("error = %v, want 'renderer not configured'", err)
	}
}

func TestTicketService_RenderLaunch_PreservesSelection(t *testing.T) {
	svc := makeService(nil, nil)
	sel := Selection{
		Ticket:    makeTicket("bmx-99", "Preserve me", "in_progress"),
		Assistant: "amp",
		Model:     "anthropic/claude-sonnet-4",
		Agent:     "researcher",
	}

	spec, err := svc.RenderLaunch(sel, "/repo",
		"amp --model {{.Model}}",
		"",
	)
	if err != nil {
		t.Fatalf("RenderLaunch() error = %v", err)
	}

	if spec.Selection.Ticket.ID != "bmx-99" {
		t.Errorf("Selection.Ticket.ID = %q, want %q", spec.Selection.Ticket.ID, "bmx-99")
	}
	if spec.Selection.Assistant != "amp" {
		t.Errorf("Selection.Assistant = %q, want %q", spec.Selection.Assistant, "amp")
	}
	if spec.Selection.Agent != "researcher" {
		t.Errorf("Selection.Agent = %q, want %q", spec.Selection.Agent, "researcher")
	}
}
