// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package tickets

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/andyrewlee/amux/internal/discovery"
)

// TicketService composes store, discovery, and renderer into a single
// business-logic layer. It is the primary entry point for ticket-aware
// operations and follows the amux service pattern (cf. workspaceService).
//
// The service is intentionally config-agnostic: callers supply template
// strings when calling RenderLaunch, keeping this layer free of config
// imports and easily testable.
type TicketService struct {
	store    TicketStore
	registry *discovery.Registry
	renderer *Renderer
}

// NewTicketService creates a TicketService from its three dependencies.
// Any nil dependency will cause the corresponding methods to return errors.
func NewTicketService(store TicketStore, registry *discovery.Registry, renderer *Renderer) *TicketService {
	return &TicketService{
		store:    store,
		registry: registry,
		renderer: renderer,
	}
}

// ListTickets returns tickets matching the given filter.
// Delegates directly to the underlying TicketStore.
func (s *TicketService) ListTickets(ctx context.Context, filter TicketFilter) ([]Ticket, error) {
	if s.store == nil {
		return nil, fmt.Errorf("ticket service: store not configured")
	}
	return s.store.ListTickets(ctx, filter)
}

// LatestUpdate returns the most recent update timestamp across all tickets.
// Delegates directly to the underlying TicketStore.
func (s *TicketService) LatestUpdate(ctx context.Context) (time.Time, error) {
	if s.store == nil {
		return time.Time{}, fmt.Errorf("ticket service: store not configured")
	}
	return s.store.LatestUpdate(ctx)
}

// ResolveModels expands a list of model keywords into concrete model IDs.
//
// Three keyword forms are supported:
//   - "discover:active" → all models from providers whose env vars are set
//   - "provider:X"      → all models from provider X
//   - literal string    → passed through unchanged
//
// The result is deduplicated and sorted.
func (s *TicketService) ResolveModels(keywords []string) []string {
	if s.registry == nil || len(keywords) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	var result []string

	for _, kw := range keywords {
		resolved := s.resolveKeyword(kw)
		for _, m := range resolved {
			if _, ok := seen[m]; !ok {
				seen[m] = struct{}{}
				result = append(result, m)
			}
		}
	}

	sort.Strings(result)
	return result
}

func (s *TicketService) resolveKeyword(keyword string) []string {
	switch {
	case keyword == discovery.KeywordDiscoverActive:
		return s.registry.GetActiveModels()
	case strings.HasPrefix(keyword, discovery.PrefixProvider):
		providerID := strings.TrimPrefix(keyword, discovery.PrefixProvider)
		if models := s.registry.GetModelsForProvider(providerID); models != nil {
			return models
		}
		return nil
	default:
		return []string{keyword}
	}
}

// RenderLaunch builds a fully resolved LaunchSpec from a Selection.
//
// The caller provides command and prompt template strings (typically from
// AssistantConfig). The method constructs a TemplateContext, hydrates the
// templates through the Renderer, and returns a LaunchSpec ready for
// execution by the app's PTY/tmux layer.
func (s *TicketService) RenderLaunch(sel Selection, workDir, cmdTemplate, promptTemplate string) (*LaunchSpec, error) {
	if s.renderer == nil {
		return nil, fmt.Errorf("ticket service: renderer not configured")
	}

	ctx := BuildTemplateContext(sel, workDir)
	ctx.CommandTemplate = cmdTemplate
	ctx.PromptTemplate = promptTemplate

	spec, err := s.renderer.RenderSelection(ctx)
	if err != nil {
		return nil, fmt.Errorf("ticket service: %w", err)
	}

	return spec, nil
}
