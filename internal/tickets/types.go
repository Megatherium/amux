// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// Package tickets provides domain types for bead issue tracking and ticket-aware launching.
package tickets

import (
	"errors"
	"fmt"
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
// while also exposing ticket fields directly for template access via methods.
//
// Templates receive this struct and can access any field directly:
//   - Ticket fields via .Ticket.ID, .Ticket.Title, etc. (from embedded Selection)
//   - Flattened ticket accessors via .TicketID, .TicketTitle, etc. (method-based)
//   - Selection fields via .Assistant, .Model, .Agent (from embedded Selection)
//   - Model helpers: {{.Model.Provider}}, {{.Model.Org}}, {{.Model.Name}}, {{.Model.ModelID}}
//
// The method-based accessors eliminate manual field copying (Shotgun Surgery):
// adding a new Ticket field only requires adding one accessor method here.
type TemplateContext struct {
	Selection `json:"selection"`

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

// TicketID returns the ticket's ID for template access as {{.TicketID}}.
func (t TemplateContext) TicketID() string { return t.Ticket.ID }

// TicketTitle returns the ticket's title for template access as {{.TicketTitle}}.
func (t TemplateContext) TicketTitle() string { return t.Ticket.Title }

// TicketDescription returns the ticket's description for template access as {{.TicketDescription}}.
func (t TemplateContext) TicketDescription() string { return t.Ticket.Description }

// TicketStatus returns the ticket's status for template access as {{.TicketStatus}}.
func (t TemplateContext) TicketStatus() string { return t.Ticket.Status }

// TicketPriority returns the ticket's priority for template access as {{.TicketPriority}}.
func (t TemplateContext) TicketPriority() int { return t.Ticket.Priority }

// TicketIssueType returns the ticket's issue type for template access as {{.TicketIssueType}}.
func (t TemplateContext) TicketIssueType() string { return t.Ticket.IssueType }

// TicketAssignee returns the ticket's assignee for template access as {{.TicketAssignee}}.
func (t TemplateContext) TicketAssignee() string { return t.Ticket.Assignee }

// TicketCreatedAt returns the ticket's creation timestamp as ISO 8601 for template access as {{.TicketCreatedAt}}.
func (t TemplateContext) TicketCreatedAt() string {
	return t.Ticket.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
}

// TicketUpdatedAt returns the ticket's update timestamp as ISO 8601 for template access as {{.TicketUpdatedAt}}.
func (t TemplateContext) TicketUpdatedAt() string {
	return t.Ticket.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
}

// PriorityLabel returns a verbose human-readable label for a priority number
// (e.g., "P0 critical", "P1 high"). Used in the center pane where space is
// available.
func PriorityLabel(p int) string {
	switch {
	case p <= 0:
		return "P?"
	case p == 1:
		return "P0 critical"
	case p == 2:
		return "P1 high"
	case p == 3:
		return "P2 medium"
	case p == 4:
		return "P3 low"
	default:
		return fmt.Sprintf("P%d", p-1)
	}
}

// PriorityLabelShort returns a compact priority label (e.g., "P0", "P1").
// Used in the sidebar where horizontal space is limited.
func PriorityLabelShort(p int) string {
	switch {
	case p <= 0:
		return "?"
	case p == 1:
		return "P0"
	case p == 2:
		return "P1"
	case p == 3:
		return "P2"
	case p == 4:
		return "P3"
	default:
		return fmt.Sprintf("P%d", p-1)
	}
}
