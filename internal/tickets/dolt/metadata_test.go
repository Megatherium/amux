// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package dolt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMetadata_ServerMode(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	metadataJSON := `{
		"backend": "dolt",
		"dolt_database": "beads_bb",
		"dolt_mode": "server",
		"dolt_server_host": "10.11.0.1",
		"dolt_server_port": 13307,
		"dolt_server_user": "mysql-root"
	}`
	metadataPath := filepath.Join(beadsDir, "metadata.json")
	if err := os.WriteFile(metadataPath, []byte(metadataJSON), 0o644); err != nil {
		t.Fatalf("Failed to write metadata.json: %v", err)
	}

	metadata, err := LoadMetadata(beadsDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if metadata.DoltDatabase != "beads_bb" {
		t.Errorf("Expected DoltDatabase='beads_bb', got %q", metadata.DoltDatabase)
	}

	if metadata.ServerHost != "10.11.0.1" {
		t.Errorf("Expected ServerHost='10.11.0.1', got %q", metadata.ServerHost)
	}

	if metadata.ServerPort != 13307 {
		t.Errorf("Expected ServerPort=13307, got %d", metadata.ServerPort)
	}

	if metadata.ServerUser != "mysql-root" {
		t.Errorf("Expected ServerUser='mysql-root', got %q", metadata.ServerUser)
	}
}

func TestLoadMetadata_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, "nonexistent", ".beads")

	_, err := LoadMetadata(beadsDir)
	if err == nil {
		t.Fatal("Expected error for missing metadata.json")
	}

	if !strings.Contains(err.Error(), "no beads database found") {
		t.Errorf("Error should mention beads database not found, got: %v", err)
	}

	if !strings.Contains(err.Error(), "bd init") {
		t.Errorf("Error should suggest running 'bd init', got: %v", err)
	}
}

func TestLoadMetadata_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	metadataPath := filepath.Join(beadsDir, "metadata.json")
	if err := os.WriteFile(metadataPath, []byte("not valid json"), 0o644); err != nil {
		t.Fatalf("Failed to write metadata.json: %v", err)
	}

	_, err := LoadMetadata(beadsDir)
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "corrupted or has invalid JSON") {
		t.Errorf("Error should mention invalid JSON, got: %v", err)
	}
}

func TestLoadMetadata_MissingDoltDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	metadataJSON := `{
		"backend": "dolt"
	}`
	metadataPath := filepath.Join(beadsDir, "metadata.json")
	if err := os.WriteFile(metadataPath, []byte(metadataJSON), 0o644); err != nil {
		t.Fatalf("Failed to write metadata.json: %v", err)
	}

	_, err := LoadMetadata(beadsDir)
	if err == nil {
		t.Fatal("Expected error for missing dolt_database")
	}

	if !strings.Contains(err.Error(), "missing required field 'dolt_database'") {
		t.Errorf("Error should mention missing dolt_database, got: %v", err)
	}
}

func TestDoltDir(t *testing.T) {
	beadsDir := "/home/user/project/.beads"
	expected := "/home/user/project/.beads/dolt"
	got := DoltDir(beadsDir)
	if got != expected {
		t.Errorf("DoltDir(%q) = %q, want %q", beadsDir, got, expected)
	}
}

func TestResolveServerPort_AlreadySet(t *testing.T) {
	m := &Metadata{
		DoltDatabase: "test_db",
		ServerPort:   12345,
	}

	port, err := m.ResolveServerPort("/tmp/test")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if port != 12345 {
		t.Errorf("Expected port 12345, got %d", port)
	}
}

func TestResolveServerPort_FromEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	t.Setenv("BEADS_DOLT_SERVER_PORT", "12345")

	m := &Metadata{
		DoltDatabase: "test_db",
		ServerPort:   0,
	}

	port, err := m.ResolveServerPort(beadsDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if port != 12345 {
		t.Errorf("Expected port 12345, got %d", port)
	}
}

func TestResolveServerPort_EnvOverridesPortFile(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(beadsDir, "dolt-server.port"), []byte("11111"), 0o644); err != nil {
		t.Fatalf("Failed to write port file: %v", err)
	}

	t.Setenv("BEADS_DOLT_SERVER_PORT", "22222")

	m := &Metadata{
		DoltDatabase: "test_db",
		ServerPort:   0,
	}

	port, err := m.ResolveServerPort(beadsDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if port != 22222 {
		t.Errorf("Expected env var port 22222 (overrides port file), got %d", port)
	}
}

func TestResolveServerPort_PortFileOverridesConfigYAML(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(beadsDir, "dolt-server.port"), []byte("33333"), 0o644); err != nil {
		t.Fatalf("Failed to write port file: %v", err)
	}

	configYAML := "dolt:\n  port: 44444\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte(configYAML), 0o644); err != nil {
		t.Fatalf("Failed to write config.yaml: %v", err)
	}

	m := &Metadata{
		DoltDatabase: "test_db",
		ServerPort:   0,
	}

	port, err := m.ResolveServerPort(beadsDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if port != 33333 {
		t.Errorf("Expected port file port 33333 (overrides config.yaml), got %d", port)
	}
}

func TestResolveServerPort_ConfigYAMLOverridesMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	configYAML := "dolt:\n  port: 55555\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte(configYAML), 0o644); err != nil {
		t.Fatalf("Failed to write config.yaml: %v", err)
	}

	m := &Metadata{
		DoltDatabase: "test_db",
		ServerPort:   60000,
	}

	port, err := m.ResolveServerPort(beadsDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if port != 55555 {
		t.Errorf("Expected config.yaml port 55555 (overrides metadata.json), got %d", port)
	}
}

func TestResolveServerPort_MetadataFallback(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	m := &Metadata{
		DoltDatabase: "test_db",
		ServerPort:   77000,
	}

	port, err := m.ResolveServerPort(beadsDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if port != 77000 {
		t.Errorf("Expected metadata.json fallback port 77000, got %d", port)
	}
}

func TestResolveServerPort_AllSourcesUnavailable(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	t.Setenv("BEADS_DOLT_SERVER_PORT", "")

	m := &Metadata{
		DoltDatabase: "test_db",
		ServerPort:   0,
	}

	_, err := m.ResolveServerPort(beadsDir)
	if err == nil {
		t.Fatal("Expected error when all sources unavailable")
	}

	if !strings.Contains(err.Error(), "failed to resolve Dolt server port") {
		t.Errorf("Error should mention failed to resolve, got: %v", err)
	}
}

func TestResolveServerPort_InvalidEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	t.Setenv("BEADS_DOLT_SERVER_PORT", "not-a-number")

	m := &Metadata{
		DoltDatabase: "test_db",
		ServerPort:   0,
	}

	_, err := m.ResolveServerPort(beadsDir)
	if err == nil {
		t.Fatal("Expected error for invalid env var")
	}
}

func TestResolveServerPort_EnvVarPortOutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	t.Setenv("BEADS_DOLT_SERVER_PORT", "99999")

	m := &Metadata{
		DoltDatabase: "test_db",
		ServerPort:   0,
	}

	_, err := m.ResolveServerPort(beadsDir)
	if err == nil {
		t.Fatal("Expected error for port out of range (>65535)")
	}
}

func TestResolveServerPort_InvalidPortFile(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(beadsDir, "dolt-server.port"), []byte("not-numeric"), 0o644); err != nil {
		t.Fatalf("Failed to write port file: %v", err)
	}

	m := &Metadata{
		DoltDatabase: "test_db",
		ServerPort:   0,
	}

	_, err := m.ResolveServerPort(beadsDir)
	if err == nil {
		t.Fatal("Expected error for non-numeric port file")
	}
}

func TestResolveServerPort_PortFileOutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(beadsDir, "dolt-server.port"), []byte("70000"), 0o644); err != nil {
		t.Fatalf("Failed to write port file: %v", err)
	}

	m := &Metadata{
		DoltDatabase: "test_db",
		ServerPort:   0,
	}

	_, err := m.ResolveServerPort(beadsDir)
	if err == nil {
		t.Fatal("Expected error for port file out of range (>65535)")
	}
}

func TestResolveServerPort_PortFileZero(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(beadsDir, "dolt-server.port"), []byte("0"), 0o644); err != nil {
		t.Fatalf("Failed to write port file: %v", err)
	}

	m := &Metadata{
		DoltDatabase: "test_db",
		ServerPort:   0,
	}

	_, err := m.ResolveServerPort(beadsDir)
	if err == nil {
		t.Fatal("Expected error for zero port in port file")
	}
}

func TestResolveServerPort_PortFileWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(beadsDir, "dolt-server.port"), []byte("  \n"), 0o644); err != nil {
		t.Fatalf("Failed to write port file: %v", err)
	}

	m := &Metadata{
		DoltDatabase: "test_db",
		ServerPort:   0,
	}

	_, err := m.ResolveServerPort(beadsDir)
	if err == nil {
		t.Fatal("Expected error for whitespace-only port file")
	}
}

func TestResolveServerPort_InvalidConfigYAML(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	configYAML := "not: valid: yaml"
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte(configYAML), 0o644); err != nil {
		t.Fatalf("Failed to write config.yaml: %v", err)
	}

	m := &Metadata{
		DoltDatabase: "test_db",
		ServerPort:   0,
	}

	_, err := m.ResolveServerPort(beadsDir)
	if err == nil {
		t.Fatal("Expected error for malformed config.yaml")
	}
}

func TestResolveServerPort_ConfigYAMLInvalidPort(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	configYAML := "dolt:\n  port: -1\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte(configYAML), 0o644); err != nil {
		t.Fatalf("Failed to write config.yaml: %v", err)
	}

	m := &Metadata{
		DoltDatabase: "test_db",
		ServerPort:   0,
	}

	_, err := m.ResolveServerPort(beadsDir)
	if err == nil {
		t.Fatal("Expected error for negative port in config.yaml")
	}
}

func TestResolveServerPort_ConfigYAMLPortOutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	configYAML := "dolt:\n  port: 99999\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte(configYAML), 0o644); err != nil {
		t.Fatalf("Failed to write config.yaml: %v", err)
	}

	m := &Metadata{
		DoltDatabase: "test_db",
		ServerPort:   0,
	}

	_, err := m.ResolveServerPort(beadsDir)
	if err == nil {
		t.Fatal("Expected error for out-of-range port in config.yaml")
	}
}
