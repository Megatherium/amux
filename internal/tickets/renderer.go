// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// Package tickets provides template rendering for ticket-aware assistant launches.
// It uses Go's text/template to render command and prompt templates with rich
// context from a fully-hydrated TemplateContext.
//
// The rendering pipeline:
//   - Caller hydrates TemplateContext (including CommandTemplate/PromptTemplate from config)
//   - RenderPrompt renders the optional prompt template
//   - RenderCommand renders the command template with Prompt available as {{.Prompt}}
//
// Templates receive TemplateContext and can access any field directly:
//   - Ticket accessors: TicketID, TicketTitle, TicketDescription, etc. (method-based)
//   - Ticket fields: .Ticket.ID, .Ticket.Title, etc. (from embedded Selection)
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
)

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

func (r *Renderer) RenderCommand(ctx TemplateContext) (string, error) {
	return r.renderTemplate("command_template", ctx.CommandTemplate, ctx)
}

func (r *Renderer) RenderPrompt(ctx TemplateContext) (string, error) {
	if ctx.PromptTemplate == "" {
		return "", nil
	}
	return r.renderTemplate("prompt_template", ctx.PromptTemplate, ctx)
}

func (r *Renderer) renderTemplate(templateName, templateStr string, ctx TemplateContext) (string, error) {
	if templateStr == "" {
		return "", nil
	}

	tmpl, err := template.New(templateName).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf(
			"failed to parse %s for assistant %q: %w",
			templateName,
			ctx.Assistant,
			err,
		)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf(
			"failed to execute %s for assistant %q: %w",
			templateName,
			ctx.Assistant,
			err,
		)
	}

	return buf.String(), nil
}

func (r *Renderer) RenderSelection(ctx TemplateContext) (*LaunchSpec, error) {
	renderedPrompt, err := r.RenderPrompt(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to render prompt: %w", err)
	}

	ctx.Prompt = renderedPrompt

	renderedCmd, err := r.RenderCommand(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to render command: %w", err)
	}
	if renderedCmd == "" {
		return nil, fmt.Errorf("command_template produced empty result for assistant %q", ctx.Assistant)
	}

	return &LaunchSpec{
		Selection:       ctx.Selection,
		RenderedCommand: renderedCmd,
		RenderedPrompt:  renderedPrompt,
		LauncherID:      ctx.Ticket.ID,
		WorkDir:         ctx.WorkDir,
	}, nil
}

func BuildTemplateContext(sel Selection, workDir string) TemplateContext {
	return TemplateContext{
		Selection: sel,

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
