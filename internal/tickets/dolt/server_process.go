package dolt

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func StartServer(ctx context.Context, beadsDir string, metadata *Metadata) error {
	projectDir := filepath.Dir(beadsDir)
	timeout := metadata.ServerReadyTimeout()

	startCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(startCtx, "bd", "dolt", "start")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dolt server: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		if startCtx.Err() == context.DeadlineExceeded {
		} else {
			return fmt.Errorf("dolt server start command failed: %w", err)
		}
	}

	return waitForServerReady(ctx, beadsDir, timeout)
}

func waitForServerReady(ctx context.Context, beadsDir string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		port, err := readServerPort(beadsDir)
		if err == nil && port > 0 {
			if err := testServerConnection(ctx, beadsDir, port); err == nil {
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("dolt server did not become ready within %v", timeout)
}

func readServerPort(beadsDir string) (int, error) {
	portFilePath := filepath.Join(beadsDir, doltServerPortFile)

	data, err := os.ReadFile(portFilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read port file: %w", err)
	}

	portStr := strings.TrimSpace(string(data))
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid port in file: %s", portStr)
	}

	return port, nil
}

func testServerConnection(ctx context.Context, beadsDir string, port int) error {
	metadata, err := LoadMetadata(beadsDir)
	if err != nil {
		return err
	}

	dsn := buildServerDSN(metadata, port)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return db.PingContext(pingCtx)
}

func IsRunning(beadsDir string) (bool, int, error) {
	pid, err := readServerPID(beadsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	if err := isProcessAlive(pid); err != nil {
		return false, 0, fmt.Errorf("failed to check process liveness: %w", err)
	}

	isDolt, err := isDoltProcess(pid)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("failed to verify dolt process: %w", err)
	}
	if !isDolt {
		return false, 0, fmt.Errorf("PID %d is not a dolt process", pid)
	}

	port, err := readServerPort(beadsDir)
	if err != nil {
		return false, 0, fmt.Errorf("dolt server PID %d is alive but port file is missing: server may be orphaned", pid)
	}

	return true, port, nil
}

func readServerPID(beadsDir string) (int, error) {
	pidFilePath := filepath.Join(beadsDir, doltServerPIDFile)

	data, err := os.ReadFile(pidFilePath)
	if err != nil {
		return 0, err
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid PID in file: %s", pidStr)
	}

	return pid, nil
}

func isProcessAlive(pid int) error {
	err := syscall.Kill(pid, 0)
	if err != nil {
		if err == syscall.ESRCH {
			return fmt.Errorf("process %d does not exist", pid)
		}
		if err == syscall.EPERM {
			return nil
		}
		return err
	}
	return nil
}

func isDoltProcess(pid int) (bool, error) {
	cmdlinePath := fmt.Sprintf("/proc/%d/cmdline", pid)

	data, err := os.ReadFile(cmdlinePath)
	if err != nil {
		return false, fmt.Errorf("failed to read cmdline for PID %d: %w", pid, err)
	}

	cmdline := string(data)
	parts := strings.Split(cmdline, "\x00")
	for _, part := range parts {
		if len(part) > 0 {
			basename := filepath.Base(part)
			if strings.EqualFold(basename, "dolt") {
				return true, nil
			}
		}
	}

	return false, nil
}
