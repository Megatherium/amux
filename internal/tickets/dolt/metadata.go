// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package dolt

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	envDoltServerPort  = "BEADS_DOLT_SERVER_PORT"
	doltServerPortFile = "dolt-server.port"
	doltServerPIDFile  = "dolt-server.pid"
	configFileName     = "config.yaml"
	metadataFileName   = "metadata.json"
)

type Metadata struct {
	Backend                   string `json:"backend"`
	DoltDatabase              string `json:"dolt_database"`
	DoltMode                  string `json:"dolt_mode"`
	ServerHost                string `json:"dolt_server_host"`
	ServerPort                int    `json:"dolt_server_port"`
	ServerUser                string `json:"dolt_server_user"`
	ServerReadyTimeoutSeconds int    `json:"dolt_server_ready_timeout"`
}

func (m *Metadata) IsValid() bool {
	return m.DoltDatabase != ""
}

func (m *Metadata) ServerReadyTimeout() time.Duration {
	if m.ServerReadyTimeoutSeconds > 0 {
		return time.Duration(m.ServerReadyTimeoutSeconds) * time.Second
	}
	return 10 * time.Second
}

func LoadMetadata(beadsDir string) (*Metadata, error) {
	metadataPath := filepath.Join(beadsDir, metadataFileName)

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"no beads database found at %q: %s is missing\n"+
					"Is this a beads project? Run 'bd init' to initialize beads in this repository",
				beadsDir, metadataFileName,
			)
		}
		return nil, fmt.Errorf("failed to read %s: %w", metadataPath, err)
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf(
			"%s is corrupted or has invalid JSON: %w\n"+
				"Try removing %s and running 'bd init' to recreate it",
			metadataPath, err, metadataPath,
		)
	}

	if !metadata.IsValid() {
		return nil, fmt.Errorf(
			"%s is missing required field 'dolt_database'\n"+
				"File location: %s\n"+
				"Try running 'bd init' to regenerate the metadata file",
			metadataPath, metadataPath,
		)
	}

	return &metadata, nil
}

func DoltDir(beadsDir string) string {
	return filepath.Join(beadsDir, "dolt")
}

type yamlConfig struct {
	Dolt DoltConfig `yaml:"dolt"`
}

type DoltConfig struct {
	Port int `yaml:"port"`
}

func (m *Metadata) ResolveServerPort(beadsDir string) (int, error) {
	if port, err := m.resolveFromEnv(); err == nil && port > 0 {
		return port, nil
	}

	if port, err := m.resolveFromPortFile(beadsDir); err == nil && port > 0 {
		return port, nil
	}

	if port, err := m.resolveFromConfigYAML(beadsDir); err == nil && port > 0 {
		return port, nil
	}

	if m.ServerPort > 0 {
		return m.ServerPort, nil
	}

	// Deprecated: dolt_server_port in metadata.json is a legacy fallback.
	// New code should prefer env var, port file, or config.yaml.

	return 0, errors.New("failed to resolve Dolt server port")
}

func (m *Metadata) resolveFromEnv() (int, error) {
	portStr := os.Getenv(envDoltServerPort)
	if portStr == "" {
		return 0, errors.New("env var not set")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid port from env: %s", portStr)
	}

	return port, nil
}

func (m *Metadata) resolveFromPortFile(beadsDir string) (int, error) {
	portFilePath := filepath.Join(beadsDir, doltServerPortFile)

	data, err := os.ReadFile(portFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, errors.New("port file not found")
		}
		return 0, fmt.Errorf("failed to read port file: %w", err)
	}

	port, err := strconv.Atoi(string(data))
	if err != nil || port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid port in file: %s", string(data))
	}

	return port, nil
}

func (m *Metadata) resolveFromConfigYAML(beadsDir string) (int, error) {
	configPath := filepath.Join(beadsDir, configFileName)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, errors.New("config file not found")
		}
		return 0, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg yamlConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return 0, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	if cfg.Dolt.Port <= 0 || cfg.Dolt.Port > 65535 {
		return 0, fmt.Errorf("invalid dolt.port in config: %d", cfg.Dolt.Port)
	}

	return cfg.Dolt.Port, nil
}
