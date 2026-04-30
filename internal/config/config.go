package config

import (
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/andyrewlee/amux/internal/validation"
)

// Config holds the application configuration
type Config struct {
	Paths         *Paths
	PortStart     int
	PortRangeSize int
	Assistants    map[string]AssistantConfig
	UI            UISettings
	Defaults      *Defaults
}

// Defaults holds optional default selections.
type Defaults struct {
	Harness string
	Model   string
	Agent   string
}

// AssistantConfig defines how to launch an AI assistant
type AssistantConfig struct {
	Command          string            // Shell command to launch the assistant
	CommandTemplate  string            // Go text/template for command rendering
	PromptTemplate   string            // Go text/template for prompt rendering
	SupportedModels  []string          // List of supported model identifiers
	SupportedAgents  []string          // List of supported agent types
	Env              map[string]string // Environment variables to set
	InterruptCount   int               // Number of Ctrl-C signals to send (default 1, claude needs 2)
	InterruptDelayMs int               // Delay between interrupts in milliseconds
}

type assistantConfigRaw struct {
	Command          string            `json:"command"`
	CommandTemplate  string            `json:"command_template,omitempty"`
	PromptTemplate   string            `json:"prompt_template,omitempty"`
	SupportedModels  []string          `json:"supported_models,omitempty"`
	SupportedAgents  []string          `json:"supported_agents,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
	InterruptCount   *int              `json:"interrupt_count,omitempty"`
	InterruptDelayMs *int              `json:"interrupt_delay_ms,omitempty"`
}

const fallbackDefaultAssistant = "claude"

var preferredAssistantOrder = []string{
	"claude",
	"codex",
	"gemini",
	"amp",
	"opencode",
	"droid",
	"cline",
	"cursor",
	"pi",
}

// DefaultConfig returns the default configuration
func DefaultConfig() (*Config, error) {
	paths, err := DefaultPaths()
	if err != nil {
		return nil, err
	}

	assistants := defaultAssistants()
	loadYAMLConfig(paths.ConfigYAMLPath, assistants)
	loadAssistantOverrides(paths.ConfigPath, assistants)

	cfg := &Config{
		Paths:         paths,
		PortStart:     6200,
		PortRangeSize: 10,
		UI:            loadUISettings(paths.ConfigPath),
		Assistants:    assistants,
	}
	return cfg, nil
}

// AssistantNames returns assistant IDs in deterministic display order.
func (c *Config) AssistantNames() []string {
	if c == nil {
		return nil
	}
	return orderedAssistantNames(c.Assistants)
}

// IsAssistantKnown reports whether assistant exists in loaded config.
func (c *Config) IsAssistantKnown(assistant string) bool {
	if c == nil || len(c.Assistants) == 0 {
		return false
	}
	_, ok := c.Assistants[normalizeAssistantName(assistant)]
	return ok
}

// ResolvedDefaultAssistant returns a valid default assistant name.
func (c *Config) ResolvedDefaultAssistant() string {
	if c == nil {
		return fallbackDefaultAssistant
	}
	return canonicalDefaultAssistant(fallbackDefaultAssistant, c.Assistants)
}

func defaultAssistants() map[string]AssistantConfig {
	return map[string]AssistantConfig{
		"claude": {
			Command:          "claude",
			InterruptCount:   2,
			InterruptDelayMs: 200,
		},
		"codex": {
			Command:          "codex",
			InterruptCount:   1,
			InterruptDelayMs: 0,
		},
		"gemini": {
			Command:          "gemini",
			InterruptCount:   1,
			InterruptDelayMs: 0,
		},
		"amp": {
			Command:          "amp",
			InterruptCount:   1,
			InterruptDelayMs: 0,
		},
		"opencode": {
			Command:          "opencode",
			InterruptCount:   1,
			InterruptDelayMs: 0,
		},
		"droid": {
			Command:          "droid",
			InterruptCount:   1,
			InterruptDelayMs: 0,
		},
		"cline": {
			Command:          "cline",
			InterruptCount:   1,
			InterruptDelayMs: 0,
		},
		"cursor": {
			Command:          "agent",
			InterruptCount:   1,
			InterruptDelayMs: 0,
		},
		"pi": {
			Command:          "pi",
			InterruptCount:   1,
			InterruptDelayMs: 0,
		},
	}
}

//nolint:cyclop // legacy suppression
func loadAssistantOverrides(path string, assistants map[string]AssistantConfig) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var raw struct {
		Assistants map[string]assistantConfigRaw `json:"assistants"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}

	for name, override := range raw.Assistants {
		normalized := normalizeAssistantName(name)
		if normalized == "" {
			continue
		}
		if err := validation.ValidateAssistant(normalized); err != nil {
			continue
		}

		cfg := assistants[normalized]
		if cmd := strings.TrimSpace(override.Command); cmd != "" {
			cfg.Command = cmd
		}
		if cmdTemplate := strings.TrimSpace(override.CommandTemplate); cmdTemplate != "" {
			cfg.CommandTemplate = cmdTemplate
		}
		if promptTemplate := strings.TrimSpace(override.PromptTemplate); promptTemplate != "" {
			cfg.PromptTemplate = promptTemplate
		}
		if override.SupportedModels != nil {
			cfg.SupportedModels = override.SupportedModels
		}
		if override.SupportedAgents != nil {
			cfg.SupportedAgents = override.SupportedAgents
		}
		if override.Env != nil {
			cfg.Env = override.Env
		}
		if override.InterruptCount != nil {
			cfg.InterruptCount = *override.InterruptCount
		}
		if override.InterruptDelayMs != nil {
			cfg.InterruptDelayMs = *override.InterruptDelayMs
		}

		if cfg.Command == "" {
			continue
		}
		if cfg.InterruptCount <= 0 {
			cfg.InterruptCount = 1
		}
		if cfg.InterruptDelayMs < 0 {
			cfg.InterruptDelayMs = 0
		}

		assistants[normalized] = cfg
	}
}

func loadYAMLConfig(path string, assistants map[string]AssistantConfig) {
	loader := NewYAMLLoader()
	yamlCfg, err := loader.Load(path)
	if err != nil {
		return
	}

	for name, override := range yamlCfg.Assistants {
		normalized := normalizeAssistantName(name)
		if normalized == "" {
			continue
		}
		if err := validation.ValidateAssistant(normalized); err != nil {
			continue
		}

		cfg := assistants[normalized]
		if cmdTemplate := strings.TrimSpace(override.CommandTemplate); cmdTemplate != "" {
			cfg.CommandTemplate = cmdTemplate
		}
		if promptTemplate := strings.TrimSpace(override.PromptTemplate); promptTemplate != "" {
			cfg.PromptTemplate = promptTemplate
		}
		if override.SupportedModels != nil {
			cfg.SupportedModels = override.SupportedModels
		}
		if override.SupportedAgents != nil {
			cfg.SupportedAgents = override.SupportedAgents
		}
		if override.Env != nil {
			cfg.Env = override.Env
		}

		assistants[normalized] = cfg
	}
}

func orderedAssistantNames(assistants map[string]AssistantConfig) []string {
	if len(assistants) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(assistants))
	names := make([]string, 0, len(assistants))

	for _, name := range preferredAssistantOrder {
		if _, ok := assistants[name]; ok {
			names = append(names, name)
			seen[name] = struct{}{}
		}
	}

	var extras []string
	for name := range assistants {
		if _, ok := seen[name]; ok {
			continue
		}
		extras = append(extras, name)
	}
	sort.Strings(extras)
	names = append(names, extras...)

	return names
}

func canonicalDefaultAssistant(candidate string, assistants map[string]AssistantConfig) string {
	name := normalizeAssistantName(candidate)
	if name != "" {
		if _, ok := assistants[name]; ok {
			return name
		}
	}
	if _, ok := assistants[fallbackDefaultAssistant]; ok {
		return fallbackDefaultAssistant
	}
	names := orderedAssistantNames(assistants)
	if len(names) > 0 {
		return names[0]
	}
	return fallbackDefaultAssistant
}

func normalizeAssistantName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
