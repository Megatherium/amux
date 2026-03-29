// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package tickets

import (
	"strings"
	"testing"
	"time"
)

func TestRenderer_RenderCommand(t *testing.T) {
	tests := []struct {
		name    string
		ctx     TemplateContext
		want    string
		wantErr bool
		errHas  []string
	}{
		{
			name: "simple",
			ctx: TemplateContext{
				CommandTemplate: "echo {{.Model}}",
				Model:           NewModelContext("claude-sonnet"),
			},
			want: "echo claude-sonnet",
		},
		{
			name: "complex",
			ctx: TemplateContext{
				CommandTemplate: "opencode --model {{.Model}} --agent {{.Agent}} --ticket {{.TicketID}}",
				Selection:       Selection{Agent: "coder"},
				TicketID:        "bb-abc",
				Model:           NewModelContext("claude-sonnet-4-20250514"),
			},
			want: "opencode --model claude-sonnet-4-20250514 --agent coder --ticket bb-abc",
		},
		{
			name:    "invalid template syntax",
			ctx:     TemplateContext{CommandTemplate: "echo {{.BadField", Selection: Selection{Assistant: "bad"}},
			wantErr: true,
			errHas:  []string{"bad", "command_template"},
		},
		{
			name:    "missing field execution",
			ctx:     TemplateContext{CommandTemplate: "echo {{.NonExistent}}"},
			wantErr: true,
			errHas:  []string{"command_template"},
		},
		{
			name: "empty template",
			ctx:  TemplateContext{CommandTemplate: ""},
			want: "",
		},
		{
			name: "escaping",
			ctx: TemplateContext{
				CommandTemplate: "echo '{{.TicketTitle}}'",
				TicketTitle:     `It is a "test"`,
			},
			want: `echo 'It is a "test"'`,
		},
		{
			name: "model structured fields",
			ctx: TemplateContext{
				CommandTemplate: "echo {{.Model.Provider}}/{{.Model.Org}}/{{.Model.Name}} {{.Model.ModelID}}",
				Model:           NewModelContext("openrouter/google/gemini-3-pro"),
			},
			want: "echo openrouter/google/gemini-3-pro openrouter/google/gemini-3-pro",
		},
		{
			name: "model backward compatible",
			ctx: TemplateContext{
				CommandTemplate: "echo {{.Model}}",
				Model:           NewModelContext("openrouter/google/gemini-3-pro"),
			},
			want: "echo openrouter/google/gemini-3-pro",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewRenderer().RenderCommand(tt.ctx)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error")
				}
				for _, substr := range tt.errHas {
					if !strings.Contains(err.Error(), substr) {
						t.Errorf("Error should contain %q, got: %v", substr, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestRenderer_RenderPrompt(t *testing.T) {
	tests := []struct {
		name    string
		ctx     TemplateContext
		want    string
		wantErr bool
		errHas  string
	}{
		{
			name: "with template",
			ctx: TemplateContext{
				PromptTemplate: "Work on {{.TicketID}}: {{.TicketTitle}}",
				TicketID:       "bb-123",
				TicketTitle:    "Fix Bug",
			},
			want: "Work on bb-123: Fix Bug",
		},
		{
			name: "empty template",
			ctx: TemplateContext{
				PromptTemplate: "",
				TicketID:       "bb-123",
			},
			want: "",
		},
		{
			name:    "invalid template",
			ctx:     TemplateContext{PromptTemplate: "{{.Bad"},
			wantErr: true,
			errHas:  "prompt_template",
		},
		{
			name: "multiline",
			ctx: TemplateContext{
				PromptTemplate: "Ticket: {{.TicketID}}\nTitle: {{.TicketTitle}}\nPriority: {{.TicketPriority}}",
				TicketID:       "bb-456",
				TicketTitle:    "Multiline Test",
				TicketPriority: 1,
			},
			want: "Ticket: bb-456\nTitle: Multiline Test\nPriority: 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewRenderer().RenderPrompt(tt.ctx)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error")
				}
				if !strings.Contains(err.Error(), tt.errHas) {
					t.Errorf("Error should contain %q, got: %v", tt.errHas, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestRenderer_RenderSelection(t *testing.T) {
	now := time.Now()
	baseSel := Selection{
		Ticket: Ticket{
			ID: "bb-base", Title: "Base", Description: "Desc",
			Status: "open", Priority: 2, IssueType: "task", Assignee: "user",
			CreatedAt: now, UpdatedAt: now,
		},
		Assistant: "test", Model: "gpt-4", Agent: "coder",
	}

	tests := []struct {
		name       string
		ctx        TemplateContext
		wantCmd    string
		wantPrompt string
		wantErr    bool
		errHas     string
	}{
		{
			name: "complete with prompt",
			ctx: func() TemplateContext {
				sel := baseSel
				sel.Ticket.ID = "bb-test"
				sel.Ticket.Title = "Test Ticket"
				sel.Ticket.Description = "A test description"
				sel.Assistant = "opencode"
				sel.Model = "claude-sonnet"
				sel.Agent = "coder"
				c := BuildTemplateContext(sel, "")
				c.CommandTemplate = "opencode --model {{.Model}} --agent {{.Agent}}"
				c.PromptTemplate = "{{.TicketID}}: {{.TicketTitle}}\n{{.TicketDescription}}"
				return c
			}(),
			wantCmd:    "opencode --model claude-sonnet --agent coder",
			wantPrompt: "bb-test: Test Ticket\nA test description",
		},
		{
			name: "no prompt",
			ctx: func() TemplateContext {
				sel := baseSel
				sel.Ticket.ID = "bb-123"
				sel.Ticket.Title = "No Prompt Test"
				sel.Assistant = "minimal"
				sel.Model = "test-model"
				sel.Agent = "test-agent"
				c := BuildTemplateContext(sel, "")
				c.CommandTemplate = "minimal"
				return c
			}(),
			wantCmd:    "minimal",
			wantPrompt: "",
		},
		{
			name: "command error",
			ctx: func() TemplateContext {
				sel := baseSel
				sel.Ticket.ID = "bb-123"
				sel.Assistant = "bad-cmd"
				c := BuildTemplateContext(sel, "")
				c.CommandTemplate = "{{.UndefinedVar}}"
				return c
			}(),
			wantErr: true,
			errHas:  "failed to render command",
		},
		{
			name: "prompt error",
			ctx: func() TemplateContext {
				sel := baseSel
				sel.Ticket.ID = "bb-123"
				sel.Assistant = "bad-prompt"
				c := BuildTemplateContext(sel, "")
				c.CommandTemplate = "echo ok"
				c.PromptTemplate = "{{.UndefinedVar}}"
				return c
			}(),
			wantErr: true,
			errHas:  "failed to render prompt",
		},
		{
			name: "prompt variable in command",
			ctx: func() TemplateContext {
				sel := baseSel
				sel.Ticket.ID = "bb-xyz"
				sel.Ticket.Title = "Test Ticket"
				sel.Assistant = "test"
				sel.Model = "gpt-4"
				c := BuildTemplateContext(sel, "")
				c.CommandTemplate = `ai-agent --prompt "{{.Prompt}}"`
				c.PromptTemplate = "Fix issue {{.TicketID}}: {{.TicketTitle}}"
				return c
			}(),
			wantCmd:    `ai-agent --prompt "Fix issue bb-xyz: Test Ticket"`,
			wantPrompt: "Fix issue bb-xyz: Test Ticket",
		},
		{
			name: "empty prompt variable in command",
			ctx: func() TemplateContext {
				sel := baseSel
				sel.Ticket.ID = "bb-empty"
				sel.Ticket.Title = "Test"
				sel.Assistant = "test"
				c := BuildTemplateContext(sel, "")
				c.CommandTemplate = "echo '{{.Prompt}}'"
				return c
			}(),
			wantCmd:    "echo ''",
			wantPrompt: "",
		},
		{
			name: "multiline prompt variable in command",
			ctx: func() TemplateContext {
				sel := baseSel
				sel.Ticket.ID = "bb-multi"
				sel.Ticket.Title = "Multiline Test"
				sel.Assistant = "test"
				c := BuildTemplateContext(sel, "")
				c.CommandTemplate = "script.sh << 'EOF'\n{{.Prompt}}\nEOF"
				c.PromptTemplate = "Line 1: {{.TicketID}}\nLine 2: {{.TicketTitle}}\nLine 3: Done"
				return c
			}(),
			wantCmd:    "script.sh << 'EOF'\nLine 1: bb-multi\nLine 2: Multiline Test\nLine 3: Done\nEOF",
			wantPrompt: "Line 1: bb-multi\nLine 2: Multiline Test\nLine 3: Done",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := NewRenderer().RenderSelection(tt.ctx)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error")
				}
				if !strings.Contains(err.Error(), tt.errHas) {
					t.Errorf("Error should contain %q, got: %v", tt.errHas, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if spec.RenderedCommand != tt.wantCmd {
				t.Errorf("command: expected %q, got %q", tt.wantCmd, spec.RenderedCommand)
			}
			if spec.RenderedPrompt != tt.wantPrompt {
				t.Errorf("prompt: expected %q, got %q", tt.wantPrompt, spec.RenderedPrompt)
			}
		})
	}
}

func TestRenderer_RenderSelection_PreservesMetadata(t *testing.T) {
	now := time.Now()
	sel := Selection{
		Ticket: Ticket{
			ID: "bb-test", Title: "Test Ticket", Description: "A test",
			Status: "open", Priority: 1, IssueType: "task", Assignee: "testuser",
			CreatedAt: now, UpdatedAt: now,
		},
		Assistant: "opencode", Model: "claude-sonnet", Agent: "coder",
	}
	ctx := BuildTemplateContext(sel, "/work")
	ctx.CommandTemplate = "echo ok"

	spec, err := NewRenderer().RenderSelection(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if spec.LauncherID != "bb-test" {
		t.Errorf("Expected LauncherID 'bb-test', got %q", spec.LauncherID)
	}
	if spec.Selection.Ticket.ID != "bb-test" {
		t.Errorf("Expected ticket ID preserved")
	}
	if spec.WorkDir != "/work" {
		t.Errorf("Expected WorkDir '/work', got %q", spec.WorkDir)
	}
}

func TestRenderer_RenderSelection_EmptyCommandGuard(t *testing.T) {
	now := time.Now()
	sel := Selection{
		Ticket: Ticket{
			ID: "bb-test", Title: "Test", Description: "Desc",
			Status: "open", Priority: 1, IssueType: "task", Assignee: "user",
			CreatedAt: now, UpdatedAt: now,
		},
		Assistant: "test", Model: "gpt-4", Agent: "coder",
	}
	ctx := BuildTemplateContext(sel, "")

	_, err := NewRenderer().RenderSelection(ctx)
	if err == nil {
		t.Fatal("Expected error for empty command_template result")
	}
	if !strings.Contains(err.Error(), "command_template produced empty result") {
		t.Errorf("Error should mention empty result, got: %v", err)
	}
	if !strings.Contains(err.Error(), "test") {
		t.Errorf("Error should contain assistant name, got: %v", err)
	}
}

func TestBuildTemplateContext_Complete(t *testing.T) {
	now := time.Now()
	sel := Selection{
		Ticket: Ticket{
			ID: "bb-abc", Title: "Test Title", Description: "Test Desc",
			Status: "in_progress", Priority: 2, IssueType: "feature", Assignee: "user1",
			CreatedAt: now, UpdatedAt: now,
		},
		Assistant: "opencode", Model: "claude-sonnet", Agent: "coder",
	}

	ctx := BuildTemplateContext(sel, "")

	if ctx.TicketID != "bb-abc" {
		t.Errorf("Expected TicketID 'bb-abc', got %q", ctx.TicketID)
	}
	if ctx.TicketTitle != "Test Title" {
		t.Errorf("Expected TicketTitle 'Test Title', got %q", ctx.TicketTitle)
	}
	if ctx.TicketDescription != "Test Desc" {
		t.Errorf("Expected TicketDescription 'Test Desc', got %q", ctx.TicketDescription)
	}
	if ctx.TicketStatus != "in_progress" {
		t.Errorf("Expected TicketStatus 'in_progress', got %q", ctx.TicketStatus)
	}
	if ctx.TicketPriority != 2 {
		t.Errorf("Expected TicketPriority 2, got %d", ctx.TicketPriority)
	}
	if ctx.TicketIssueType != "feature" {
		t.Errorf("Expected TicketIssueType 'feature', got %q", ctx.TicketIssueType)
	}
	if ctx.TicketAssignee != "user1" {
		t.Errorf("Expected TicketAssignee 'user1', got %q", ctx.TicketAssignee)
	}
	if ctx.Assistant != "opencode" {
		t.Errorf("Expected Assistant 'opencode', got %q", ctx.Assistant)
	}
	if ctx.Model.ModelID() != "claude-sonnet" {
		t.Errorf("Expected ModelID 'claude-sonnet', got %q", ctx.Model.ModelID())
	}
	if ctx.Agent != "coder" {
		t.Errorf("Expected Agent 'coder', got %q", ctx.Agent)
	}
	if ctx.Timestamp != now.Unix() {
		t.Errorf("Expected Timestamp %d, got %d", now.Unix(), ctx.Timestamp)
	}
}

func TestBuildTemplateContext_Environment(t *testing.T) {
	sel := Selection{
		Ticket:    Ticket{ID: "bb-test", Title: "Test"},
		Assistant: "claude", Model: "claude-sonnet", Agent: "coder",
	}

	t.Run("custom workdir", func(t *testing.T) {
		ctx := BuildTemplateContext(sel, "/custom/workdir")
		if ctx.WorkDir != "/custom/workdir" {
			t.Errorf("Expected WorkDir '/custom/workdir', got %q", ctx.WorkDir)
		}
		if ctx.RepoPath != "" {
			t.Errorf("Expected empty RepoPath, got %q", ctx.RepoPath)
		}
	})

	t.Run("timestamp from updated_at", func(t *testing.T) {
		now := time.Now()
		sel := sel
		sel.Ticket.UpdatedAt = now
		ctx := BuildTemplateContext(sel, "")
		if ctx.Timestamp != now.Unix() {
			t.Errorf("Expected Timestamp %d, got %d", now.Unix(), ctx.Timestamp)
		}
	})

	t.Run("templates not hydrated", func(t *testing.T) {
		ctx := BuildTemplateContext(sel, "")
		if ctx.CommandTemplate != "" {
			t.Errorf("Expected empty CommandTemplate, got %q", ctx.CommandTemplate)
		}
		if ctx.PromptTemplate != "" {
			t.Errorf("Expected empty PromptTemplate, got %q", ctx.PromptTemplate)
		}
	})
}
