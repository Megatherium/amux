package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigLoadsYAMLFirst(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	yamlConfigPath := filepath.Join(home, ".amux", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(yamlConfigPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	yamlContent := `
assistants:
  opencode:
    command_template: "opencode-yaml"
    prompt_template: "YAML prompt {{.TicketID}}"
    models:
      - yaml-model
`
	if err := os.WriteFile(yamlConfigPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() error = %v", err)
	}

	opencode, ok := cfg.Assistants["opencode"]
	if !ok {
		t.Fatal("Expected opencode assistant to exist")
	}
	if opencode.CommandTemplate != "opencode-yaml" {
		t.Errorf("opencode command_template = %q, want %q", opencode.CommandTemplate, "opencode-yaml")
	}
	if opencode.PromptTemplate != "YAML prompt {{.TicketID}}" {
		t.Errorf("opencode prompt_template = %q, want %q", opencode.PromptTemplate, "YAML prompt {{.TicketID}}")
	}
	if len(opencode.SupportedModels) != 1 || opencode.SupportedModels[0] != "yaml-model" {
		t.Errorf("opencode supported_models = %v, want [yaml-model]", opencode.SupportedModels)
	}
}

func TestDefaultConfigJSONOverridesYAML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	yamlConfigPath := filepath.Join(home, ".amux", "config.yaml")
	jsonConfigPath := filepath.Join(home, ".amux", "config.json")
	if err := os.MkdirAll(filepath.Dir(yamlConfigPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	yamlContent := `
assistants:
  opencode:
    command_template: "opencode-yaml"
    prompt_template: "YAML prompt"
    models:
      - yaml-model
`
	if err := os.WriteFile(yamlConfigPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	jsonContent := `{
  "assistants": {
    "opencode": {
      "command_template": "opencode-json"
    }
  }
}`
	if err := os.WriteFile(jsonConfigPath, []byte(jsonContent), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() error = %v", err)
	}

	opencode, ok := cfg.Assistants["opencode"]
	if !ok {
		t.Fatal("Expected opencode assistant to exist")
	}
	if opencode.CommandTemplate != "opencode-json" {
		t.Errorf("opencode command_template = %q, want %q (JSON should override YAML)", opencode.CommandTemplate, "opencode-json")
	}
	if opencode.PromptTemplate != "YAML prompt" {
		t.Errorf("opencode prompt_template = %q, want %q (YAML should be preserved)", opencode.PromptTemplate, "YAML prompt")
	}
}

func TestDefaultConfigFallsBackToJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	jsonConfigPath := filepath.Join(home, ".amux", "config.json")
	if err := os.MkdirAll(filepath.Dir(jsonConfigPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	jsonContent := `{
  "assistants": {
    "opencode": {
      "command": "opencode-json"
    }
  }
}`
	if err := os.WriteFile(jsonConfigPath, []byte(jsonContent), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() error = %v", err)
	}

	opencode, ok := cfg.Assistants["opencode"]
	if !ok {
		t.Fatal("Expected opencode assistant to exist")
	}
	if opencode.Command != "opencode-json" {
		t.Errorf("opencode command = %q, want %q", opencode.Command, "opencode-json")
	}
}
