// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// Package tickets provides domain types for bead issue tracking and ticket-aware launching.
package tickets

import (
	"errors"
	"strings"
	"time"
)

// Ticket represents a beads issue for display and context in the TUI.
// Tickets are loaded from the bd CLI/Dolt database and used to provide
// issue context when launching agent tabs.
type Ticket struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    int       `json:"priority"`
	IssueType   string    `json:"issue_type"`
	Assignee    string    `json:"assignee"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewTicket creates a Ticket with basic validation.
// Returns an error if ID or Title are empty.
func NewTicket(id, title string) (Ticket, error) {
	if id == "" {
		return Ticket{}, errors.New("ticket id is required")
	}
	if title == "" {
		return Ticket{}, errors.New("ticket title is required")
	}
	return Ticket{ID: id, Title: title}, nil
}

// Selection captures the user's complete choice of ticket, assistant,
// model, and agent before rendering a command or prompt template.
type Selection struct {
	Ticket    Ticket `json:"ticket"`
	Assistant string `json:"assistant"`
	Model     string `json:"model"`
	Agent     string `json:"agent"`
}

// NewSelection creates a Selection with basic validation.
// Returns an error if Assistant is empty.
func NewSelection(ticket Ticket, assistant, model, agent string) (Selection, error) {
	if assistant == "" {
		return Selection{}, errors.New("assistant is required")
	}
	return Selection{
		Ticket:    ticket,
		Assistant: assistant,
		Model:     model,
		Agent:     agent,
	}, nil
}

// LaunchSpec is a fully resolved selection ready for execution.
// Contains the original selection plus rendered command/prompt and execution metadata.
type LaunchSpec struct {
	Selection       Selection `json:"selection"`
	RenderedCommand string    `json:"rendered_command"`
	RenderedPrompt  string    `json:"rendered_prompt"`
	LauncherID      string    `json:"launcher_id"`
	WorkDir         string    `json:"work_dir"`
}

// NewLaunchSpec creates a LaunchSpec with basic validation.
// Returns an error if RenderedCommand is empty.
func NewLaunchSpec(selection Selection, renderedCommand, renderedPrompt, launcherID, workDir string) (LaunchSpec, error) {
	if renderedCommand == "" {
		return LaunchSpec{}, errors.New("rendered command is required")
	}
	return LaunchSpec{
		Selection:       selection,
		RenderedCommand: renderedCommand,
		RenderedPrompt:  renderedPrompt,
		LauncherID:      launcherID,
		WorkDir:         workDir,
	}, nil
}

// LaunchResult captures the outcome of a launch attempt.
// Error is not serialized to JSON since it's not serializable.
type LaunchResult struct {
	LauncherID string `json:"launcher_id"`
	PID        int    `json:"pid"`
	Error      error  `json:"-"` // Excluded from JSON serialization
}

// ModelContext keeps the full model ID while exposing structured accessors in templates.
// Model IDs follow the format "provider/org/name" (e.g., "anthropic/claude/claude-sonnet-4").
// A string-backed type is used to preserve template truthiness semantics for {{if .Model}}.
// The zero value represents an unspecified model.
type ModelContext string

// NewModelContext wraps a model ID string for template access.
func NewModelContext(modelID string) ModelContext { return ModelContext(modelID) }

// String returns the full model ID string.
// Implements fmt.Stringer for convenience.
func (m ModelContext) String() string { return string(m) }

// ModelID returns the full model identifier.
func (m ModelContext) ModelID() string { return string(m) }

// Provider returns the provider segment of the model ID.
// For "anthropic/claude/claude-sonnet-4", returns "anthropic".
// Returns empty string if model ID has no provider segment.
func (m ModelContext) Provider() string {
	provider, _, _ := m.parts()
	return provider
}

// Org returns the organization segment of the model ID.
// For "anthropic/claude/claude-sonnet-4", returns "claude".
// Returns empty string if model ID has fewer than 2 segments.
func (m ModelContext) Org() string {
	_, org, _ := m.parts()
	return org
}

// Organization is an alias for Org for template readability.
func (m ModelContext) Organization() string {
	return m.Org()
}

// Name returns the model name segment (the final component).
// For "anthropic/claude/claude-sonnet-4", returns "claude-sonnet-4".
// For "claude-sonnet-4", returns "claude-sonnet-4".
func (m ModelContext) Name() string {
	_, _, name := m.parts()
	return name
}

// parts splits the model ID into provider, organization, and name components.
//
// Model IDs can have 1-3+ segments separated by slashes:
//   - 1 segment: name only (e.g., "claude-sonnet-4") → "", "", "claude-sonnet-4"
//   - 2 segments: provider/name (e.g., "anthropic/claude") → "anthropic", "", "claude"
//   - 3+ segments: provider/org/name (e.g., "anthropic/claude/claude-sonnet-4")
//     → "anthropic", "claude", "claude-sonnet-4"
//
// For IDs with more than 3 segments, the name combines all segments after org
// (e.g., "a/b/c/d" → "a", "b", "c/d").
//
// Returns empty strings for the zero value ModelContext.
func (m ModelContext) parts() (provider, org, name string) {
	if m == "" {
		return "", "", ""
	}

	parts := strings.Split(string(m), "/")
	switch len(parts) {
	case 1:
		return "", "", parts[0]
	case 2:
		return parts[0], "", parts[1]
	default:
		return parts[0], parts[1], strings.Join(parts[2:], "/")
	}
}

// TemplateContext is the fat context passed to both command and prompt templates.
// It embeds Selection to avoid duplicating ticket and selection fields,
// while also exposing all fields directly for template access.
//
// Templates receive this struct and can access any field directly:
//   - Ticket fields via .Ticket.ID, .Ticket.Title, etc.
//   - Selection fields via .Selection.Model, .Selection.Assistant, etc.
//   - Flattened ticket fields via .TicketID, .TicketTitle, etc. for convenience
type TemplateContext struct {
	Selection `json:"selection"`

	// Flattened ticket fields for template convenience.
	// These are populated from Selection.Ticket at construction time.
	TicketID          string `json:"ticket_id"`
	TicketTitle       string `json:"ticket_title"`
	TicketDescription string `json:"ticket_description"`
	TicketStatus      string `json:"ticket_status"`
	TicketPriority    int    `json:"ticket_priority"`
	TicketIssueType   string `json:"ticket_issue_type"`
	TicketAssignee    string `json:"ticket_assignee"`
	TicketCreatedAt   string `json:"ticket_created_at"`
	TicketUpdatedAt   string `json:"ticket_updated_at"`

	// Model is exposed as ModelContext for structured template accessors.
	Model ModelContext `json:"model"`

	// Environment fields
	RepoPath string `json:"repo_path"`
	Branch   string `json:"branch"`
	WorkDir  string `json:"work_dir"`
	User     string `json:"user"`
	Hostname string `json:"hostname"`

	// Runtime fields
	DryRun    bool  `json:"dry_run"`
	Debug     bool  `json:"debug"`
	Timestamp int64 `json:"timestamp"`

	// Templates resolved from config, used by the renderer.
	// The caller is responsible for hydrating these from AssistantConfig
	// before passing TemplateContext to any Render method.
	CommandTemplate string `json:"command_template"`
	PromptTemplate  string `json:"prompt_template"`

	// Prompt is the rendered prompt text, available in command_template
	// via {{.Prompt}} for composable template designs.
	Prompt string `json:"prompt"`
}
