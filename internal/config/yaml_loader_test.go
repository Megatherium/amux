package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYAMLLoader_Load_ValidConfig(t *testing.T) {
	yamlContent := `
assistants:
  opencode:
    command_template: "opencode --model {{.Model}}"
    prompt_template: "Work on {{.TicketID}}"
    models:
      - claude-sonnet
      - o3
    agents:
      - coder
    env:
      LOG_LEVEL: debug
  amp:
    command_template: "amp"
    models: []
    agents: []
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	loader := NewYAMLLoader()
	config, err := loader.Load(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(config.Assistants) != 2 {
		t.Errorf("Expected 2 assistants, got %d", len(config.Assistants))
	}

	opencode, ok := config.Assistants["opencode"]
	if !ok {
		t.Fatal("Expected opencode assistant to exist")
	}
	if opencode.CommandTemplate != "opencode --model {{.Model}}" {
		t.Errorf("Unexpected command_template: %q", opencode.CommandTemplate)
	}
	if opencode.PromptTemplate != "Work on {{.TicketID}}" {
		t.Errorf("Unexpected prompt_template: %q", opencode.PromptTemplate)
	}
	if len(opencode.SupportedModels) != 2 {
		t.Errorf("Expected 2 models, got %d", len(opencode.SupportedModels))
	}
	if len(opencode.SupportedAgents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(opencode.SupportedAgents))
	}
	if opencode.Env["LOG_LEVEL"] != "debug" {
		t.Errorf("Expected env LOG_LEVEL='debug', got %q", opencode.Env["LOG_LEVEL"])
	}

	amp, ok := config.Assistants["amp"]
	if !ok {
		t.Fatal("Expected amp assistant to exist")
	}
	if len(amp.SupportedModels) != 0 {
		t.Errorf("Expected empty models list, got %d items", len(amp.SupportedModels))
	}
	if len(amp.SupportedAgents) != 0 {
		t.Errorf("Expected empty agents list, got %d items", len(amp.SupportedAgents))
	}
}

func TestYAMLLoader_Load_MissingFile(t *testing.T) {
	loader := NewYAMLLoader()
	_, err := loader.Load("/nonexistent/config.yaml")

	if err == nil {
		t.Fatal("Expected error for missing file")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("Error should mention 'config file not found', got: %v", err)
	}
}

func TestYAMLLoader_Load_InvalidYAML(t *testing.T) {
	yamlContent := `
assistants:
  opencode:
    command_template: [invalid yaml structure
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	loader := NewYAMLLoader()
	_, err := loader.Load(configPath)

	if err == nil {
		t.Fatal("Expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "failed to parse YAML") {
		t.Errorf("Error should mention 'failed to parse YAML', got: %v", err)
	}
}

func TestYAMLLoader_Load_MissingCommandTemplate(t *testing.T) {
	yamlContent := `
assistants:
  opencode:
    prompt_template: "Work on {{.TicketID}}"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	loader := NewYAMLLoader()
	_, err := loader.Load(configPath)

	if err == nil {
		t.Fatal("Expected error for missing command_template")
	}
	if !strings.Contains(err.Error(), "command_template is required") {
		t.Errorf("Error should mention 'command_template is required', got: %v", err)
	}
}

func TestYAMLLoader_Load_DuplicateAssistantNames(t *testing.T) {
	yamlContent := `
assistants:
  opencode:
    command_template: "opencode"
  opencode:
    command_template: "opencode-alt"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	loader := NewYAMLLoader()
	_, err := loader.Load(configPath)

	if err == nil {
		t.Fatal("Expected error for duplicate assistant names")
	}
	if !strings.Contains(err.Error(), "duplicate") && !strings.Contains(err.Error(), "already defined") {
		t.Errorf("Error should mention duplicate key, got: %v", err)
	}
}

func TestYAMLLoader_Load_FileBasedTemplates(t *testing.T) {
	tmpDir := t.TempDir()

	templatesDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	cmdTemplateContent := "opencode --model {{.Model}}"
	cmdTemplatePath := filepath.Join(templatesDir, "command.txt")
	if err := os.WriteFile(cmdTemplatePath, []byte(cmdTemplateContent), 0o644); err != nil {
		t.Fatalf("Failed to write command template: %v", err)
	}

	promptTemplateContent := "Work on {{.TicketID}}"
	promptTemplatePath := filepath.Join(templatesDir, "prompt.txt")
	if err := os.WriteFile(promptTemplatePath, []byte(promptTemplateContent), 0o644); err != nil {
		t.Fatalf("Failed to write prompt template: %v", err)
	}

	yamlContent := `
assistants:
  opencode:
    command_template: "@./templates/command.txt"
    prompt_template: "@./templates/prompt.txt"
    models:
      - claude-sonnet
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	loader := NewYAMLLoader()
	config, err := loader.Load(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	opencode, ok := config.Assistants["opencode"]
	if !ok {
		t.Fatal("Expected opencode assistant to exist")
	}
	if opencode.CommandTemplate != cmdTemplateContent {
		t.Errorf("Unexpected command_template: %q", opencode.CommandTemplate)
	}
	if opencode.PromptTemplate != promptTemplateContent {
		t.Errorf("Unexpected prompt_template: %q", opencode.PromptTemplate)
	}
}

func TestYAMLLoader_Load_FileBasedTemplates_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()

	yamlContent := `
assistants:
  opencode:
    command_template: "@./templates/missing.txt"
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	loader := NewYAMLLoader()
	_, err := loader.Load(configPath)

	if err == nil {
		t.Fatal("Expected error for missing template file")
	}
	if !strings.Contains(err.Error(), "failed to load template file") {
		t.Errorf("Error should mention 'failed to load template file', got: %v", err)
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("Error should mention 'file not found', got: %v", err)
	}
}

func TestYAMLLoader_Load_MixedInlineAndFileTemplates(t *testing.T) {
	tmpDir := t.TempDir()

	templatesDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	promptTemplateContent := "Complex prompt: {{.TicketID}}"
	promptTemplatePath := filepath.Join(templatesDir, "prompt.txt")
	if err := os.WriteFile(promptTemplatePath, []byte(promptTemplateContent), 0o644); err != nil {
		t.Fatalf("Failed to write prompt template: %v", err)
	}

	yamlContent := `
assistants:
  inline-cmd:
    command_template: "inline command {{.Model}}"
    prompt_template: "@./templates/prompt.txt"
  file-cmd:
    command_template: "@./templates/prompt.txt"
    prompt_template: "inline prompt {{.TicketID}}"
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	loader := NewYAMLLoader()
	config, err := loader.Load(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	inlineCmd, ok := config.Assistants["inline-cmd"]
	if !ok {
		t.Fatal("Expected inline-cmd assistant to exist")
	}
	if inlineCmd.CommandTemplate != "inline command {{.Model}}" {
		t.Errorf("Unexpected inline command_template: %q", inlineCmd.CommandTemplate)
	}
	if inlineCmd.PromptTemplate != promptTemplateContent {
		t.Errorf("Unexpected file-based prompt_template: %q", inlineCmd.PromptTemplate)
	}

	fileCmd, ok := config.Assistants["file-cmd"]
	if !ok {
		t.Fatal("Expected file-cmd assistant to exist")
	}
	if fileCmd.CommandTemplate != promptTemplateContent {
		t.Errorf("Unexpected file-based command_template: %q", fileCmd.CommandTemplate)
	}
	if fileCmd.PromptTemplate != "inline prompt {{.TicketID}}" {
		t.Errorf("Unexpected inline prompt_template: %q", fileCmd.PromptTemplate)
	}
}

func TestYAMLLoader_Load_DefaultsSection(t *testing.T) {
	yamlContent := `
assistants:
  opencode:
    command_template: "opencode"
defaults:
  harness: opencode
  model: claude-sonnet
  agent: coder
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	loader := NewYAMLLoader()
	config, err := loader.Load(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config.Defaults == nil {
		t.Fatal("Expected defaults to be set")
	}
	if config.Defaults.Harness != "opencode" {
		t.Errorf("Expected default harness 'opencode', got %q", config.Defaults.Harness)
	}
	if config.Defaults.Model != "claude-sonnet" {
		t.Errorf("Expected default model 'claude-sonnet', got %q", config.Defaults.Model)
	}
	if config.Defaults.Agent != "coder" {
		t.Errorf("Expected default agent 'coder', got %q", config.Defaults.Agent)
	}
}

func TestYAMLLoader_Load_FileBasedTemplates_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()

	cmdTemplateContent := "opencode --model {{.Model}}"
	cmdTemplatePath := filepath.Join(tmpDir, "command.txt")
	if err := os.WriteFile(cmdTemplatePath, []byte(cmdTemplateContent), 0o644); err != nil {
		t.Fatalf("Failed to write command template: %v", err)
	}

	yamlContent := `
assistants:
  opencode:
    command_template: "@` + cmdTemplatePath + `"
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	loader := NewYAMLLoader()
	config, err := loader.Load(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	opencode, ok := config.Assistants["opencode"]
	if !ok {
		t.Fatal("Expected opencode assistant to exist")
	}
	if opencode.CommandTemplate != cmdTemplateContent {
		t.Errorf("Unexpected command_template: %q", opencode.CommandTemplate)
	}
}

func TestYAMLLoader_Load_EmptyConfig(t *testing.T) {
	yamlContent := `
assistants: {}
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	loader := NewYAMLLoader()
	config, err := loader.Load(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(config.Assistants) != 0 {
		t.Errorf("Expected 0 assistants, got %d", len(config.Assistants))
	}
}
