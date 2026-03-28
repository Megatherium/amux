// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package dolt

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestBuildServerDSN(t *testing.T) {
	tests := []struct {
		name     string
		metadata *Metadata
		port     int
		wantUser string
		wantHost string
		wantPort int
		wantDB   string
	}{
		{
			name: "basic connection",
			metadata: &Metadata{
				ServerHost:   "127.0.0.1",
				ServerPort:   3306,
				ServerUser:   "root",
				DoltDatabase: "beads_bb",
			},
			port:     3306,
			wantUser: "root",
			wantHost: "127.0.0.1",
			wantPort: 3306,
			wantDB:   "beads_bb",
		},
		{
			name: "custom host and port",
			metadata: &Metadata{
				ServerHost:   "10.0.0.1",
				ServerPort:   13307,
				ServerUser:   "mysql-user",
				DoltDatabase: "test_db",
			},
			port:     13307,
			wantUser: "mysql-user",
			wantHost: "10.0.0.1",
			wantPort: 13307,
			wantDB:   "test_db",
		},
		{
			name: "default user when empty",
			metadata: &Metadata{
				ServerHost:   "127.0.0.1",
				ServerUser:   "",
				DoltDatabase: "beads_bb",
			},
			port:     3306,
			wantUser: "root",
			wantHost: "127.0.0.1",
			wantPort: 3306,
			wantDB:   "beads_bb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := buildServerDSN(tt.metadata, tt.port)

			if dsn == "" {
				t.Fatal("buildServerDSN returned empty string")
			}
		})
	}
}

func TestBuildServerDSN_PortZeroPanics(t *testing.T) {
	metadata := &Metadata{
		ServerHost:   "127.0.0.1",
		ServerUser:   "root",
		DoltDatabase: "beads_bb",
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("buildServerDSN with port=0 did not panic")
		}
	}()

	buildServerDSN(metadata, 0)
}

func TestReadServerPort(t *testing.T) {
	tests := []struct {
		name      string
		portFile  string
		wantPort  int
		wantError bool
	}{
		{
			name:     "valid port",
			portFile: "3306",
			wantPort: 3306,
		},
		{
			name:     "port with newline",
			portFile: "13307\n",
			wantPort: 13307,
		},
		{
			name:     "port with spaces",
			portFile: "  3306  ",
			wantPort: 3306,
		},
		{
			name:      "invalid non-numeric",
			portFile:  "not-a-port",
			wantError: true,
		},
		{
			name:      "zero port",
			portFile:  "0",
			wantError: true,
		},
		{
			name:      "negative port",
			portFile:  "-1",
			wantError: true,
		},
		{
			name:      "port too high",
			portFile:  "70000",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			beadsDir := filepath.Join(tmpDir, ".beads")
			if err := os.MkdirAll(beadsDir, 0o750); err != nil {
				t.Fatalf("Failed to create beads dir: %v", err)
			}

			portFilePath := filepath.Join(beadsDir, doltServerPortFile)
			if err := os.WriteFile(portFilePath, []byte(tt.portFile), 0o644); err != nil {
				t.Fatalf("Failed to write port file: %v", err)
			}

			port, err := readServerPort(beadsDir)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none; port=%d", port)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if port != tt.wantPort {
				t.Errorf("Expected port %d, got %d", tt.wantPort, port)
			}
		})
	}
}

func TestReadServerPort_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads", "nonexistent")

	_, err := readServerPort(beadsDir)
	if err == nil {
		t.Error("Expected error for missing port file")
	}
}

func TestServerStore_Close(t *testing.T) {
	store := &ServerStore{
		closed: false,
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Unexpected error on close: %v", err)
	}

	if !store.closed {
		t.Error("Expected closed to be true after Close()")
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Unexpected error on second close: %v", err)
	}
}

func TestServerStore_CanRetryConnection(t *testing.T) {
	store := &ServerStore{mode: ServerMode}
	if !store.CanRetryConnection() {
		t.Error("Expected CanRetryConnection to return true for ServerMode")
	}

	store = &ServerStore{}
	if store.CanRetryConnection() {
		t.Error("Expected CanRetryConnection to return false for zero value mode")
	}
}

func TestServerStore_AutostartEnabled(t *testing.T) {
	store := &ServerStore{autostart: true}
	if !store.AutostartEnabled() {
		t.Error("Expected AutostartEnabled to return true")
	}

	store = &ServerStore{autostart: false}
	if store.AutostartEnabled() {
		t.Error("Expected AutostartEnabled to return false")
	}
}

func TestErrServerNotRunning(t *testing.T) {
	err := &ErrServerNotRunning{Message: "test message"}

	if err.Error() != "test message" {
		t.Errorf("Expected 'test message', got %q", err.Error())
	}
}

func TestIsErrServerNotRunning(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "ErrServerNotRunning",
			err:      &ErrServerNotRunning{Message: "test"},
			expected: true,
		},
		{
			name:     "wrapped ErrServerNotRunning",
			err:      fmtError("wrapped: %w", &ErrServerNotRunning{Message: "test"}),
			expected: true,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("some error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsErrServerNotRunning(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func fmtError(format string, err error) error {
	return &wrappedError{msg: format, err: err}
}

type wrappedError struct {
	msg string
	err error
}

func (e *wrappedError) Error() string {
	return e.msg
}

func (e *wrappedError) Unwrap() error {
	return e.err
}

func TestServerReadyTimeout(t *testing.T) {
	tests := []struct {
		name     string
		metadata *Metadata
		want     time.Duration
	}{
		{
			name:     "custom timeout",
			metadata: &Metadata{ServerReadyTimeoutSeconds: 30},
			want:     30 * time.Second,
		},
		{
			name:     "default timeout",
			metadata: &Metadata{ServerReadyTimeoutSeconds: 0},
			want:     10 * time.Second,
		},
		{
			name:     "negative timeout uses default",
			metadata: &Metadata{ServerReadyTimeoutSeconds: -1},
			want:     10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.metadata.ServerReadyTimeout()
			if got != tt.want {
				t.Errorf("Expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestWaitForServerReady_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads", "nonexistent")

	ctx := context.Background()
	timeout := 100 * time.Millisecond

	err := waitForServerReady(ctx, beadsDir, timeout)
	if err == nil {
		t.Error("Expected error when port file does not exist")
	}
}

func TestServerStore_EnsureRunningAgentsTable_NoOp(t *testing.T) {
	store := &ServerStore{}
	ctx := context.Background()

	if err := store.EnsureRunningAgentsTable(ctx); err != nil {
		t.Fatalf("EnsureRunningAgentsTable should be a no-op, got error: %v", err)
	}
}

func TestReadServerPID(t *testing.T) {
	tests := []struct {
		name      string
		pidFile   string
		wantPID   int
		wantError bool
	}{
		{
			name:    "valid PID",
			pidFile: "12345",
			wantPID: 12345,
		},
		{
			name:    "PID with newline",
			pidFile: "67890\n",
			wantPID: 67890,
		},
		{
			name:    "PID with spaces",
			pidFile: "  11111  ",
			wantPID: 11111,
		},
		{
			name:      "invalid non-numeric",
			pidFile:   "not-a-pid",
			wantError: true,
		},
		{
			name:      "zero PID",
			pidFile:   "0",
			wantError: true,
		},
		{
			name:      "negative PID",
			pidFile:   "-1",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			beadsDir := filepath.Join(tmpDir, ".beads")
			if err := os.MkdirAll(beadsDir, 0o750); err != nil {
				t.Fatalf("Failed to create beads dir: %v", err)
			}

			pidFilePath := filepath.Join(beadsDir, doltServerPIDFile)
			if err := os.WriteFile(pidFilePath, []byte(tt.pidFile), 0o644); err != nil {
				t.Fatalf("Failed to write PID file: %v", err)
			}

			pid, err := readServerPID(beadsDir)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none; pid=%d", pid)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if pid != tt.wantPID {
				t.Errorf("Expected PID %d, got %d", tt.wantPID, pid)
			}
		})
	}
}

func TestReadServerPID_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads", "nonexistent")

	_, err := readServerPID(beadsDir)
	if err == nil {
		t.Error("Expected error for missing PID file")
	}
}

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
