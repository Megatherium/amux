package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type yamlConfig struct {
	Assistants map[string]yamlAssistant `yaml:"assistants"`
	Defaults   *yamlDefaults            `yaml:"defaults,omitempty"`
}

type yamlAssistant struct {
	CommandTemplate string            `yaml:"command_template"`
	PromptTemplate  string            `yaml:"prompt_template,omitempty"`
	Models          []string          `yaml:"models,omitempty"`
	Agents          []string          `yaml:"agents,omitempty"`
	Env             map[string]string `yaml:"env,omitempty"`
}

type yamlDefaults struct {
	Harness string `yaml:"harness,omitempty"`
	Model   string `yaml:"model,omitempty"`
	Agent   string `yaml:"agent,omitempty"`
}

type YAMLLoader struct{}

func NewYAMLLoader() *YAMLLoader {
	return &YAMLLoader{}
}

func (l *YAMLLoader) Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var raw yamlConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse YAML in %s: %w", path, err)
	}

	configDir := filepath.Dir(path)
	return l.convertAndValidate(&raw, configDir)
}

func (l *YAMLLoader) convertAndValidate(raw *yamlConfig, configDir string) (*Config, error) {
	assistants := make(map[string]AssistantConfig)

	if len(raw.Assistants) == 0 {
		return &Config{Assistants: assistants}, nil
	}

	if err := l.validateAssistantNames(raw.Assistants); err != nil {
		return nil, err
	}

	for name, rawAssistant := range raw.Assistants {
		normalized := normalizeAssistantName(name)
		if normalized == "" {
			continue
		}

		cfg, err := l.convertAssistant(rawAssistant, configDir)
		if err != nil {
			return nil, fmt.Errorf("assistant %q: %w", name, err)
		}

		assistants[normalized] = cfg
	}

	cfg := &Config{Assistants: assistants}

	if raw.Defaults != nil {
		cfg.Defaults = &Defaults{
			Harness: raw.Defaults.Harness,
			Model:   raw.Defaults.Model,
			Agent:   raw.Defaults.Agent,
		}
	}

	return cfg, nil
}

func (l *YAMLLoader) validateAssistantNames(assistants map[string]yamlAssistant) error {
	seenNames := make(map[string]int)
	i := 0
	for name := range assistants {
		if name == "" {
			continue
		}
		if firstIdx, exists := seenNames[name]; exists {
			return fmt.Errorf("duplicate assistant name %q at index %d (first defined at index %d)", name, i, firstIdx)
		}
		seenNames[name] = i
		i++
	}
	return nil
}

func (l *YAMLLoader) convertAssistant(raw yamlAssistant, configDir string) (AssistantConfig, error) {
	commandTemplate, err := loadTemplateValue(raw.CommandTemplate, configDir)
	if err != nil {
		return AssistantConfig{}, fmt.Errorf("command_template: %w", err)
	}
	if commandTemplate == "" {
		return AssistantConfig{}, errors.New("command_template is required")
	}

	promptTemplate, err := loadTemplateValue(raw.PromptTemplate, configDir)
	if err != nil {
		return AssistantConfig{}, fmt.Errorf("prompt_template: %w", err)
	}

	models := raw.Models
	if models == nil {
		models = []string{}
	}

	agents := raw.Agents
	if agents == nil {
		agents = []string{}
	}

	env := raw.Env
	if env == nil {
		env = map[string]string{}
	}

	return AssistantConfig{
		CommandTemplate:  commandTemplate,
		PromptTemplate:   promptTemplate,
		SupportedModels:  models,
		SupportedAgents:  agents,
		Env:              env,
		InterruptCount:   1,
		InterruptDelayMs: 0,
	}, nil
}

func loadTemplateValue(value, configDir string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, "@") {
		return value, nil
	}

	filePath := strings.TrimPrefix(value, "@")
	resolvedPath := filePath
	if !filepath.IsAbs(resolvedPath) {
		resolvedPath = filepath.Join(configDir, resolvedPath)
	}

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("failed to load template file: %s (file not found)", value)
		}
		return "", fmt.Errorf("failed to load template file: %s: %w", value, err)
	}

	return string(content), nil
}
