package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() error = %v", err)
	}
	if cfg.Paths == nil {
		t.Fatal("DefaultConfig() returned nil Paths")
	}
	if cfg.PortStart == 0 || cfg.PortRangeSize == 0 {
		t.Fatalf("DefaultConfig() returned invalid ports: start=%d range=%d", cfg.PortStart, cfg.PortRangeSize)
	}

	// Verify assistant configs referenced in README exist.
	for _, name := range []string{"claude", "codex", "gemini", "amp", "opencode", "cline"} {
		if _, ok := cfg.Assistants[name]; !ok {
			t.Fatalf("DefaultConfig() missing assistant config for %s", name)
		}
	}
	if cfg.ResolvedDefaultAssistant() == "" {
		t.Fatal("resolved default assistant should not be empty")
	}
}

func TestDefaultConfigLoadsAssistantOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".amux", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := `{
  "assistants": {
    "my-fast-agent": {
      "command": "my-fast-agent --fast"
    },
    "myagent": {
      "command": "myagent",
      "interrupt_count": 3,
      "interrupt_delay_ms": 150
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() error = %v", err)
	}

	if got := cfg.ResolvedDefaultAssistant(); got != "claude" {
		t.Fatalf("ResolvedDefaultAssistant() = %q, want %q", got, "claude")
	}
	customFast, ok := cfg.Assistants["my-fast-agent"]
	if !ok {
		t.Fatalf("expected my-fast-agent assistant to exist")
	}
	if customFast.Command != "my-fast-agent --fast" {
		t.Fatalf("my-fast-agent command = %q, want %q", customFast.Command, "my-fast-agent --fast")
	}

	custom, ok := cfg.Assistants["myagent"]
	if !ok {
		t.Fatalf("expected custom assistant to be loaded")
	}
	if custom.Command != "myagent" {
		t.Fatalf("custom command = %q, want %q", custom.Command, "myagent")
	}
	if custom.InterruptCount != 3 {
		t.Fatalf("custom interrupt_count = %d, want %d", custom.InterruptCount, 3)
	}
	if custom.InterruptDelayMs != 150 {
		t.Fatalf("custom interrupt_delay_ms = %d, want %d", custom.InterruptDelayMs, 150)
	}
}

func TestDefaultConfigIgnoresDefaultAssistantSetting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".amux", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := `{"default_assistant":"does-not-exist"}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() error = %v", err)
	}

	if got := cfg.ResolvedDefaultAssistant(); got != "claude" {
		t.Fatalf("ResolvedDefaultAssistant() = %q, want %q", got, "claude")
	}
}

func TestDefaultConfigSkipsInvalidAssistantOverrideIDs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".amux", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := `{
  "assistants": {
    "my agent": {
      "command": "bad-assistant"
    },
    "ok_agent": {
      "command": "ok-agent"
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() error = %v", err)
	}

	if _, ok := cfg.Assistants["my agent"]; ok {
		t.Fatalf("expected invalid assistant id to be ignored")
	}
	if _, ok := cfg.Assistants["ok_agent"]; !ok {
		t.Fatalf("expected valid assistant id to be loaded")
	}
	if got := cfg.ResolvedDefaultAssistant(); got != "claude" {
		t.Fatalf("ResolvedDefaultAssistant() = %q, want %q", got, "claude")
	}
}

func TestAssistantNamesOrder(t *testing.T) {
	cfg := &Config{
		Assistants: map[string]AssistantConfig{
			"zeta":     {Command: "zeta"},
			"codex":    {Command: "codex"},
			"claude":   {Command: "claude"},
			"my-agent": {Command: "my-agent"},
			"gemini":   {Command: "gemini"},
			"amp":      {Command: "amp"},
			"opencode": {Command: "opencode"},
			"droid":    {Command: "droid"},
			"cline":    {Command: "cline"},
			"cursor":   {Command: "cursor"},
			"pi":       {Command: "pi"},
		},
	}

	got := cfg.AssistantNames()
	wantPrefix := []string{"claude", "codex", "gemini", "amp", "opencode", "droid", "cline", "cursor", "pi"}
	for i, want := range wantPrefix {
		if got[i] != want {
			t.Fatalf("AssistantNames()[%d] = %q, want %q", i, got[i], want)
		}
	}
	if got[len(got)-2] != "my-agent" || got[len(got)-1] != "zeta" {
		t.Fatalf("expected custom assistants to be sorted at end, got %v", got)
	}
}

func TestAssistantConfigTemplateFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".amux", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := `{
  "assistants": {
    "claude": {
      "command": "claude",
      "command_template": "claude {{.Model}}",
      "prompt_template": "You are an expert in {{.TicketTitle}}"
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() error = %v", err)
	}

	claude, ok := cfg.Assistants["claude"]
	if !ok {
		t.Fatalf("expected claude assistant to exist")
	}
	if claude.Command != "claude" {
		t.Fatalf("claude command = %q, want %q", claude.Command, "claude")
	}
	if claude.CommandTemplate != "claude {{.Model}}" {
		t.Fatalf("claude command_template = %q, want %q", claude.CommandTemplate, "claude {{.Model}}")
	}
	if claude.PromptTemplate != "You are an expert in {{.TicketTitle}}" {
		t.Fatalf("claude prompt_template = %q, want %q", claude.PromptTemplate, "You are an expert in {{.TicketTitle}}")
	}
}

func TestAssistantConfigSupportedFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".amux", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := `{
  "assistants": {
    "myagent": {
      "command": "myagent",
      "supported_models": ["gpt-4", "gpt-3.5-turbo"],
      "supported_agents": ["coder", "reviewer"]
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() error = %v", err)
	}

	agent, ok := cfg.Assistants["myagent"]
	if !ok {
		t.Fatalf("expected myagent assistant to exist")
	}
	if len(agent.SupportedModels) != 2 {
		t.Fatalf("expected 2 supported models, got %d", len(agent.SupportedModels))
	}
	if agent.SupportedModels[0] != "gpt-4" || agent.SupportedModels[1] != "gpt-3.5-turbo" {
		t.Fatalf("supported models = %v, want [gpt-4 gpt-3.5-turbo]", agent.SupportedModels)
	}
	if len(agent.SupportedAgents) != 2 {
		t.Fatalf("expected 2 supported agents, got %d", len(agent.SupportedAgents))
	}
	if agent.SupportedAgents[0] != "coder" || agent.SupportedAgents[1] != "reviewer" {
		t.Fatalf("supported agents = %v, want [coder reviewer]", agent.SupportedAgents)
	}
}

func TestAssistantConfigEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".amux", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := `{
  "assistants": {
    "myagent": {
      "command": "myagent",
      "env": {
        "API_KEY": "secret",
        "DEBUG": "true"
      }
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() error = %v", err)
	}

	agent, ok := cfg.Assistants["myagent"]
	if !ok {
		t.Fatalf("expected myagent assistant to exist")
	}
	if len(agent.Env) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(agent.Env))
	}
	if agent.Env["API_KEY"] != "secret" {
		t.Fatalf("env API_KEY = %q, want %q", agent.Env["API_KEY"], "secret")
	}
	if agent.Env["DEBUG"] != "true" {
		t.Fatalf("env DEBUG = %q, want %q", agent.Env["DEBUG"], "true")
	}
}

func TestDefaultConfigHasNilDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() error = %v", err)
	}
	if cfg.Defaults != nil {
		t.Fatalf("expected Defaults to be nil when not configured")
	}
}

func TestAssistantConfigPreservesDefaultCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".amux", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := `{
  "assistants": {
    "claude": {
      "supported_models": ["claude-3-opus"]
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() error = %v", err)
	}

	claude, ok := cfg.Assistants["claude"]
	if !ok {
		t.Fatalf("expected claude assistant to exist")
	}
	if claude.Command != "claude" {
		t.Fatalf("claude command = %q, want %q (default should be preserved)", claude.Command, "claude")
	}
	if len(claude.SupportedModels) != 1 || claude.SupportedModels[0] != "claude-3-opus" {
		t.Fatalf("claude supported_models = %v, want [claude-3-opus]", claude.SupportedModels)
	}
}
