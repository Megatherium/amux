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

	"github.com/andyrewlee/amux/internal/config"
)

func TestRenderer_RenderCommand_Simple(t *testing.T) {
	renderer := NewRenderer()
	cfg := config.AssistantConfig{
		CommandTemplate: "echo {{.Model}}",
	}
	ctx := TemplateContext{
		Model: NewModelContext("claude-sonnet"),
	}

	result, err := renderer.RenderCommand(cfg, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "echo claude-sonnet"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestRenderer_RenderCommand_Complex(t *testing.T) {
	renderer := NewRenderer()
	cfg := config.AssistantConfig{
		CommandTemplate: "opencode --model {{.Model}} --agent {{.Agent}} --ticket {{.TicketID}}",
	}
	ctx := TemplateContext{
		Selection: Selection{
			Agent: "coder",
		},
		TicketID: "bb-abc",
		Model:    NewModelContext("claude-sonnet-4-20250514"),
	}

	result, err := renderer.RenderCommand(cfg, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "opencode --model claude-sonnet-4-20250514 --agent coder --ticket bb-abc"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestRenderer_RenderCommand_InvalidTemplate(t *testing.T) {
	renderer := NewRenderer()
	cfg := config.AssistantConfig{
		CommandTemplate: "echo {{.BadField", // Invalid template syntax
	}
	ctx := TemplateContext{
		Selection: Selection{
			Assistant: "bad",
		},
	}

	_, err := renderer.RenderCommand(cfg, ctx)
	if err == nil {
		t.Fatal("Expected error for invalid template")
	}

	if !strings.Contains(err.Error(), "bad") {
		t.Errorf("Error should mention assistant name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "command_template") {
		t.Errorf("Error should mention template type, got: %v", err)
	}
}

func TestRenderer_RenderCommand_MissingField(t *testing.T) {
	renderer := NewRenderer()
	cfg := config.AssistantConfig{
		CommandTemplate: "echo {{.NonExistent}}",
	}
	ctx := TemplateContext{}

	_, err := renderer.RenderCommand(cfg, ctx)
	if err == nil {
		t.Fatal("Expected error for missing field in template execution")
	}

	if !strings.Contains(err.Error(), "command_template") {
		t.Errorf("Error should mention template type, got: %v", err)
	}
}

func TestRenderer_RenderPrompt_WithTemplate(t *testing.T) {
	renderer := NewRenderer()
	cfg := config.AssistantConfig{
		PromptTemplate: "Work on {{.TicketID}}: {{.TicketTitle}}",
	}
	ctx := TemplateContext{
		TicketID:    "bb-123",
		TicketTitle: "Fix Bug",
	}

	result, err := renderer.RenderPrompt(cfg, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "Work on bb-123: Fix Bug"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestRenderer_RenderPrompt_EmptyTemplate(t *testing.T) {
	renderer := NewRenderer()
	cfg := config.AssistantConfig{
		PromptTemplate: "", // Empty template
	}
	ctx := TemplateContext{
		TicketID: "bb-123",
	}

	result, err := renderer.RenderPrompt(cfg, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result != "" {
		t.Errorf("Expected empty string for empty template, got %q", result)
	}
}

func TestRenderer_RenderPrompt_NoTemplateField(t *testing.T) {
	renderer := NewRenderer()
	cfg := config.AssistantConfig{
		PromptTemplate: "", // Empty string (zero value)
	}
	ctx := TemplateContext{
		TicketID: "bb-123",
	}

	result, err := renderer.RenderPrompt(cfg, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result != "" {
		t.Errorf("Expected empty string when no prompt_template, got %q", result)
	}
}

func TestRenderer_RenderPrompt_InvalidTemplate(t *testing.T) {
	renderer := NewRenderer()
	cfg := config.AssistantConfig{
		PromptTemplate: "{{.Bad", // Invalid syntax
	}
	ctx := TemplateContext{}

	_, err := renderer.RenderPrompt(cfg, ctx)
	if err == nil {
		t.Fatal("Expected error for invalid prompt template")
	}

	if !strings.Contains(err.Error(), "prompt_template") {
		t.Errorf("Error should mention template type, got: %v", err)
	}
}

func TestRenderer_RenderSelection_Complete(t *testing.T) {
	renderer := NewRenderer()
	now := time.Now()

	sel := Selection{
		Ticket: Ticket{
			ID:          "bb-test",
			Title:       "Test Ticket",
			Description: "A test description",
			Status:      "open",
			Priority:    1,
			IssueType:   "task",
			Assignee:    "testuser",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Assistant: "opencode",
		Model:     "claude-sonnet",
		Agent:     "coder",
	}
	cfg := config.AssistantConfig{
		CommandTemplate: "opencode --model {{.Model}} --agent {{.Agent}}",
		PromptTemplate:  "{{.TicketID}}: {{.TicketTitle}}\n{{.TicketDescription}}",
	}

	spec, err := renderer.RenderSelection(sel, cfg, "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedCmd := "opencode --model claude-sonnet --agent coder"
	if spec.RenderedCommand != expectedCmd {
		t.Errorf("Expected command %q, got %q", expectedCmd, spec.RenderedCommand)
	}

	expectedPrompt := "bb-test: Test Ticket\nA test description"
	if spec.RenderedPrompt != expectedPrompt {
		t.Errorf("Expected prompt %q, got %q", expectedPrompt, spec.RenderedPrompt)
	}

	if spec.LauncherID != "bb-test" {
		t.Errorf("Expected launcher ID 'bb-test', got %q", spec.LauncherID)
	}

	if spec.Selection.Ticket.ID != "bb-test" {
		t.Errorf("Expected ticket ID preserved")
	}
}

func TestRenderer_RenderSelection_NoPrompt(t *testing.T) {
	renderer := NewRenderer()

	sel := Selection{
		Ticket: Ticket{
			ID:    "bb-123",
			Title: "No Prompt Test",
		},
		Assistant: "minimal",
		Model:     "test-model",
		Agent:     "test-agent",
	}
	cfg := config.AssistantConfig{
		CommandTemplate: "minimal",
		PromptTemplate:  "", // No prompt
	}

	spec, err := renderer.RenderSelection(sel, cfg, "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if spec.RenderedPrompt != "" {
		t.Errorf("Expected empty prompt, got %q", spec.RenderedPrompt)
	}

	if spec.RenderedCommand != "minimal" {
		t.Errorf("Expected command 'minimal', got %q", spec.RenderedCommand)
	}
}

func TestRenderer_RenderSelection_CommandError(t *testing.T) {
	renderer := NewRenderer()

	sel := Selection{
		Ticket: Ticket{
			ID: "bb-123",
		},
		Assistant: "bad-cmd",
	}
	cfg := config.AssistantConfig{
		CommandTemplate: "{{.UndefinedVar}}", // Will fail on execution
	}

	_, err := renderer.RenderSelection(sel, cfg, "")
	if err == nil {
		t.Fatal("Expected error for invalid command template")
	}

	if !strings.Contains(err.Error(), "failed to render command") {
		t.Errorf("Error should mention command rendering, got: %v", err)
	}
}

func TestRenderer_RenderSelection_PromptError(t *testing.T) {
	renderer := NewRenderer()

	sel := Selection{
		Ticket: Ticket{
			ID: "bb-123",
		},
		Assistant: "bad-prompt",
	}
	cfg := config.AssistantConfig{
		CommandTemplate: "echo ok",
		PromptTemplate:  "{{.UndefinedVar}}", // Will fail on execution
	}

	_, err := renderer.RenderSelection(sel, cfg, "")
	if err == nil {
		t.Fatal("Expected error for invalid prompt template")
	}

	if !strings.Contains(err.Error(), "failed to render prompt") {
		t.Errorf("Error should mention prompt rendering, got: %v", err)
	}
}

func TestBuildTemplateContext_Complete(t *testing.T) {
	now := time.Now()
	sel := Selection{
		Ticket: Ticket{
			ID:          "bb-abc",
			Title:       "Test Title",
			Description: "Test Desc",
			Status:      "in_progress",
			Priority:    2,
			IssueType:   "feature",
			Assignee:    "user1",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Assistant: "opencode",
		Model:     "claude-sonnet",
		Agent:     "coder",
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

	if ctx.HarnessName != "opencode" {
		t.Errorf("Expected HarnessName 'opencode', got %q", ctx.HarnessName)
	}

	if ctx.Model.ModelID() != "claude-sonnet" {
		t.Errorf("Expected ModelID 'claude-sonnet', got %q", ctx.Model.ModelID())
	}
	if ctx.Model.Name() != "claude-sonnet" {
		t.Errorf("Expected Model.Name 'claude-sonnet', got %q", ctx.Model.Name())
	}
	if ctx.Agent != "coder" {
		t.Errorf("Expected Agent 'coder', got %q", ctx.Agent)
	}

	if ctx.Timestamp != now.Unix() {
		t.Errorf("Expected Timestamp %d, got %d", now.Unix(), ctx.Timestamp)
	}
}

func TestRenderer_MultilineTemplate(t *testing.T) {
	renderer := NewRenderer()
	cfg := config.AssistantConfig{
		PromptTemplate: `Ticket: {{.TicketID}}
Title: {{.TicketTitle}}
Priority: {{.TicketPriority}}`,
	}
	ctx := TemplateContext{
		TicketID:       "bb-456",
		TicketTitle:    "Multiline Test",
		TicketPriority: 1,
	}

	result, err := renderer.RenderPrompt(cfg, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "Ticket: bb-456\nTitle: Multiline Test\nPriority: 1"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestRenderer_Escaping(t *testing.T) {
	renderer := NewRenderer()
	cfg := config.AssistantConfig{
		CommandTemplate: "echo '{{.TicketTitle}}'",
	}
	ctx := TemplateContext{
		TicketTitle: `It is a "test"`, // Contains quotes, no backslash needed
	}

	result, err := renderer.RenderCommand(cfg, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := `echo 'It is a "test"'`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestRenderer_RenderCommand_WithPromptVariable(t *testing.T) {
	renderer := NewRenderer()
	now := time.Now()

	sel := Selection{
		Ticket: Ticket{
			ID:          "bb-xyz",
			Title:       "Test Ticket",
			Description: "Test description",
			Status:      "open",
			Priority:    2,
			IssueType:   "task",
			Assignee:    "user",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Assistant: "test",
		Model:     "gpt-4",
		Agent:     "coder",
	}
	cfg := config.AssistantConfig{
		CommandTemplate: "ai-agent --prompt \"{{.Prompt}}\"",
		PromptTemplate:  "Fix issue {{.TicketID}}: {{.TicketTitle}}",
	}

	spec, err := renderer.RenderSelection(sel, cfg, "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedPrompt := "Fix issue bb-xyz: Test Ticket"
	if spec.RenderedPrompt != expectedPrompt {
		t.Errorf("Expected prompt %q, got %q", expectedPrompt, spec.RenderedPrompt)
	}

	expectedCmd := `ai-agent --prompt "Fix issue bb-xyz: Test Ticket"`
	if spec.RenderedCommand != expectedCmd {
		t.Errorf("Expected command %q, got %q", expectedCmd, spec.RenderedCommand)
	}
}

func TestRenderer_RenderCommand_PromptVariable_EmptyPrompt(t *testing.T) {
	renderer := NewRenderer()
	now := time.Now()

	sel := Selection{
		Ticket: Ticket{
			ID:          "bb-empty",
			Title:       "Test",
			Description: "Desc",
			Status:      "open",
			Priority:    2,
			IssueType:   "task",
			Assignee:    "user",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Assistant: "test",
		Model:     "gpt-4",
		Agent:     "coder",
	}
	cfg := config.AssistantConfig{
		CommandTemplate: "echo '{{.Prompt}}'",
		// No PromptTemplate - will be empty
	}

	spec, err := renderer.RenderSelection(sel, cfg, "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if spec.RenderedPrompt != "" {
		t.Errorf("Expected empty prompt, got %q", spec.RenderedPrompt)
	}

	expectedCmd := "echo ''"
	if spec.RenderedCommand != expectedCmd {
		t.Errorf("Expected command %q, got %q", expectedCmd, spec.RenderedCommand)
	}
}

func TestRenderer_RenderCommand_PromptVariable_Multiline(t *testing.T) {
	renderer := NewRenderer()
	now := time.Now()

	sel := Selection{
		Ticket: Ticket{
			ID:          "bb-multi",
			Title:       "Multiline Test",
			Description: "Test",
			Status:      "open",
			Priority:    2,
			IssueType:   "task",
			Assignee:    "user",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Assistant: "test",
		Model:     "gpt-4",
		Agent:     "coder",
	}
	cfg := config.AssistantConfig{
		CommandTemplate: "script.sh << 'EOF'\n{{.Prompt}}\nEOF",
		PromptTemplate:  "Line 1: {{.TicketID}}\nLine 2: {{.TicketTitle}}\nLine 3: Done",
	}

	spec, err := renderer.RenderSelection(sel, cfg, "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedPrompt := "Line 1: bb-multi\nLine 2: Multiline Test\nLine 3: Done"
	if spec.RenderedPrompt != expectedPrompt {
		t.Errorf("Expected prompt %q, got %q", expectedPrompt, spec.RenderedPrompt)
	}

	expectedCmd := "script.sh << 'EOF'\nLine 1: bb-multi\nLine 2: Multiline Test\nLine 3: Done\nEOF"
	if spec.RenderedCommand != expectedCmd {
		t.Errorf("Expected command %q, got %q", expectedCmd, spec.RenderedCommand)
	}
}

func TestRenderer_RenderCommand_ModelStructuredFields(t *testing.T) {
	renderer := NewRenderer()
	cfg := config.AssistantConfig{
		CommandTemplate: "echo {{.Model.Provider}}/{{.Model.Org}}/{{.Model.Name}} {{.Model.ModelID}}",
	}
	ctx := TemplateContext{
		Model: NewModelContext("openrouter/google/gemini-3-pro"),
	}

	result, err := renderer.RenderCommand(cfg, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "echo openrouter/google/gemini-3-pro openrouter/google/gemini-3-pro"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestRenderer_RenderCommand_ModelBackwardCompatible(t *testing.T) {
	renderer := NewRenderer()
	cfg := config.AssistantConfig{
		CommandTemplate: "echo {{.Model}}",
	}
	ctx := TemplateContext{
		Model: NewModelContext("openrouter/google/gemini-3-pro"),
	}

	result, err := renderer.RenderCommand(cfg, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "echo openrouter/google/gemini-3-pro"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestBuildTemplateContext_EmptyWorkDir(t *testing.T) {
	sel := Selection{
		Ticket: Ticket{
			ID:    "bb-test",
			Title: "Test",
		},
		Assistant: "claude",
		Model:     "claude-sonnet",
		Agent:     "coder",
	}

	ctx := BuildTemplateContext(sel, "/custom/workdir")

	if ctx.WorkDir != "/custom/workdir" {
		t.Errorf("Expected WorkDir '/custom/workdir', got %q", ctx.WorkDir)
	}
	if ctx.RepoPath != "" {
		t.Errorf("Expected empty RepoPath, got %q", ctx.RepoPath)
	}
	if ctx.Branch != "" {
		t.Errorf("Expected empty Branch, got %q", ctx.Branch)
	}
}

func TestBuildTemplateContext_TimestampFromUpdatedAt(t *testing.T) {
	now := time.Now()
	sel := Selection{
		Ticket: Ticket{
			ID:        "bb-ts",
			Title:     "Timestamp Test",
			UpdatedAt: now,
		},
		Assistant: "claude",
		Model:     "claude-sonnet",
		Agent:     "coder",
	}

	ctx := BuildTemplateContext(sel, "")

	if ctx.Timestamp != now.Unix() {
		t.Errorf("Expected Timestamp %d, got %d", now.Unix(), ctx.Timestamp)
	}
}
