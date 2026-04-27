// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package dolt

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestIsProcessAlive(t *testing.T) {
	tests := []struct {
		name      string
		pid       int
		wantAlive bool
		wantError bool
	}{
		{
			name:      "current process is alive",
			pid:       os.Getpid(),
			wantAlive: true,
			wantError: false,
		},
		{
			name:      "init process is alive",
			pid:       1,
			wantAlive: true,
			wantError: false,
		},
		{
			name:      "non-existent PID",
			pid:       999999,
			wantAlive: false,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isProcessAlive(tt.pid)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}

func TestIsDoltProcess(t *testing.T) {
	tests := []struct {
		name      string
		pid       int
		wantDolt  bool
		wantError bool
	}{
		{
			name:      "current process is not dolt",
			pid:       os.Getpid(),
			wantDolt:  false,
			wantError: false,
		},
		{
			name:      "init process is not dolt",
			pid:       1,
			wantDolt:  false,
			wantError: false,
		},
		{
			name:      "non-existent PID",
			pid:       999999,
			wantDolt:  false,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isDolt, err := isDoltProcess(tt.pid)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none; isDolt=%v", isDolt)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if isDolt != tt.wantDolt {
				t.Errorf("Expected isDolt=%v, got %v", tt.wantDolt, isDolt)
			}
		})
	}
}

func TestIsRunning(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(string) error
		wantRunning bool
		wantError   bool
	}{
		{
			name: "missing PID file",
			setup: func(dir string) error {
				return nil
			},
			wantRunning: false,
			wantError:   false,
		},
		{
			name: "invalid PID file",
			setup: func(dir string) error {
				pidPath := filepath.Join(dir, doltServerPIDFile)
				return os.WriteFile(pidPath, []byte("invalid"), 0o644)
			},
			wantRunning: false,
			wantError:   true,
		},
		{
			name: "non-existent process",
			setup: func(dir string) error {
				pidPath := filepath.Join(dir, doltServerPIDFile)
				return os.WriteFile(pidPath, []byte("999999"), 0o644)
			},
			wantRunning: false,
			wantError:   true,
		},
		{
			name: "alive process but not dolt",
			setup: func(dir string) error {
				pidPath := filepath.Join(dir, doltServerPIDFile)
				return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o644)
			},
			wantRunning: false,
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			beadsDir := filepath.Join(tmpDir, ".beads")
			if err := os.MkdirAll(beadsDir, 0o750); err != nil {
				t.Fatalf("Failed to create beads dir: %v", err)
			}

			if err := tt.setup(beadsDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			running, _, err := IsRunning(beadsDir)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none; running=%v", running)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if running != tt.wantRunning {
				t.Errorf("Expected running=%v, got %v", tt.wantRunning, running)
			}
		})
	}
}

func TestIsRunning_OrphanedServer(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	pidPath := filepath.Join(beadsDir, doltServerPIDFile)
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatalf("Failed to write PID file: %v", err)
	}

	_, _, err := IsRunning(beadsDir)
	if err == nil {
		t.Error("Expected error when process is alive but not dolt (orphaned)")
	}
}

func TestIsRunning_AliveDoltWithoutPort(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	pidPath := filepath.Join(beadsDir, doltServerPIDFile)
	if err := os.WriteFile(pidPath, []byte("1"), 0o644); err != nil {
		t.Fatalf("Failed to write PID file: %v", err)
	}

	_, _, err := IsRunning(beadsDir)
	if err == nil {
		t.Error("Expected error when PID is alive (init) but not dolt and no port file")
	}
}
