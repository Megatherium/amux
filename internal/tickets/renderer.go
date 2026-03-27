// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// Package renderer provides template rendering for ticket-aware assistant launches.
// It uses Go's text/template to render command and prompt templates with rich
// context from a Selection (ticket, assistant, model, agent) and configuration
// (AssistantConfig with CommandTemplate/PromptTemplate fields).
//
// The rendering pipeline:
//   - BuildTemplateContext converts a Selection into a TemplateContext with
//     flattened ticket fields and structured model access
//   - RenderPrompt renders the optional prompt template first
//   - RenderCommand renders the command template with Prompt available as {{.Prompt}}
//
// Templates have access to all TemplateContext fields including:
//   - Ticket fields: TicketID, TicketTitle, TicketDescription, TicketStatus, etc.
//   - Selection fields: Assistant, Model (structured), Agent, WorkDir
//   - Model helpers: {{.Model.Provider}}, {{.Model.Org}}, {{.Model.Name}}, {{.Model.ModelID}}
//
// Example command_template:
//
//	opencode --model {{.Model}} --agent {{.Agent}} --ticket {{.TicketID}}
//
// Example prompt_template:
//
//	Work on {{.TicketID}}: {{.TicketTitle}}
package tickets

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/andyrewlee/amux/internal/config"
)

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

func (r *Renderer) RenderCommand(cfg config.AssistantConfig, ctx TemplateContext) (string, error) {
	return r.renderTemplate(
		ctx.Assistant,
		"command_template",
		cfg.CommandTemplate,
		ctx,
	)
}

func (r *Renderer) RenderPrompt(cfg config.AssistantConfig, ctx TemplateContext) (string, error) {
	if cfg.PromptTemplate == "" {
		return "", nil
	}
	return r.renderTemplate(
		ctx.Assistant,
		"prompt_template",
		cfg.PromptTemplate,
		ctx,
	)
}

func (r *Renderer) renderTemplate(assistantName, templateName, templateStr string, ctx TemplateContext) (string, error) {
	if templateStr == "" {
		return "", nil
	}

	tmpl, err := template.New(templateName).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf(
			"failed to parse %s for assistant %q: %w",
			templateName,
			assistantName,
			err,
		)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf(
			"failed to execute %s for assistant %q: %w",
			templateName,
			assistantName,
			err,
		)
	}

	return buf.String(), nil
}

func (r *Renderer) RenderSelection(sel Selection, cfg config.AssistantConfig, workDir string) (*LaunchSpec, error) {
	ctx := BuildTemplateContext(sel, workDir)

	renderedPrompt, err := r.RenderPrompt(cfg, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to render prompt: %w", err)
	}

	ctx.Prompt = renderedPrompt

	renderedCmd, err := r.RenderCommand(cfg, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to render command: %w", err)
	}

	return &LaunchSpec{
		Selection:       sel,
		RenderedCommand: renderedCmd,
		RenderedPrompt:  renderedPrompt,
		LauncherID:      sel.Ticket.ID,
		WorkDir:         workDir,
	}, nil
}

func BuildTemplateContext(sel Selection, workDir string) TemplateContext {
	return TemplateContext{
		Selection: sel,

		TicketID:          sel.Ticket.ID,
		TicketTitle:       sel.Ticket.Title,
		TicketDescription: sel.Ticket.Description,
		TicketStatus:      sel.Ticket.Status,
		TicketPriority:    sel.Ticket.Priority,
		TicketIssueType:   sel.Ticket.IssueType,
		TicketAssignee:    sel.Ticket.Assignee,
		TicketCreatedAt:   sel.Ticket.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		TicketUpdatedAt:   sel.Ticket.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),

		HarnessName: sel.Assistant,

		Model: NewModelContext(sel.Model),

		WorkDir:  workDir,
		RepoPath: "",
		Branch:   "",
		User:     "",
		Hostname: "",

		DryRun:    false,
		Debug:     false,
		Timestamp: sel.Ticket.UpdatedAt.Unix(),

		Prompt: "",
	}
}
