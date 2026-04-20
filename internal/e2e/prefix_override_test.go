package e2e

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrefixOverrideCtrlP(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoTmux(t)

	home := t.TempDir()
	repo := initRepo(t)
	writeRegistry(t, home, repo)

	server := fmt.Sprintf("amux-e2e-prefix-%d", time.Now().UnixNano())
	defer killTmuxServer(t, server)

	env := append(sessionEnv("", server),
		"AMUX_PREFIX_KEY=ctrl+p",
	)

	session, cleanup, err := StartPTYSession(PTYOptions{
		Home: home,
		Env:  env,
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	defer cleanup()

	// Wait for the dashboard to render
	waitForUIContains(t, session, filepath.Base(repo), persistenceTimeout)

	// Verify the help bar shows the override label (C-P) instead of C-Space
	waitForUIContains(t, session, "C-P", persistenceTimeout)

	// Send Ctrl+P (byte 0x10) to trigger prefix mode
	if err := session.SendBytes([]byte{0x10}); err != nil {
		t.Fatalf("send ctrl+p: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// Verify the prefix palette appeared (it shows "choices")
	waitForUIContains(t, session, "choices", persistenceTimeout)

	// Test a full prefix sequence: "q" to quit
	if err := session.SendString("q"); err != nil {
		t.Fatalf("send q: %v", err)
	}
	waitForUIContains(t, session, "Quit AMUX", persistenceTimeout)
	if err := session.SendString("\r"); err != nil {
		t.Fatalf("confirm quit: %v", err)
	}
	if err := session.WaitForExit(persistenceTimeout); err != nil {
		t.Fatalf("waiting for exit: %v", err)
	}
}

// TestPrefixTimeoutCustom verifies that AMUX_PREFIX_TIMEOUT is respected.
// With a 10s timeout the prefix palette should remain visible well past the
// default 3s — we check it's still present after 4s.
func TestPrefixTimeoutCustom(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoTmux(t)

	home := t.TempDir()
	repo := initRepo(t)
	writeRegistry(t, home, repo)

	server := fmt.Sprintf("amux-e2e-timeout-%d", time.Now().UnixNano())
	defer killTmuxServer(t, server)

	env := append(sessionEnv("", server),
		"AMUX_PREFIX_KEY=ctrl+p",
		"AMUX_PREFIX_TIMEOUT=10s",
	)

	session, cleanup, err := StartPTYSession(PTYOptions{
		Home: home,
		Env:  env,
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	defer cleanup()

	waitForUIContains(t, session, filepath.Base(repo), persistenceTimeout)

	// Enter prefix mode
	if err := session.SendBytes([]byte{0x10}); err != nil {
		t.Fatalf("send ctrl+p: %v", err)
	}
	waitForUIContains(t, session, "choices", persistenceTimeout)

	// Wait 4s — past the default 3s timeout but well within the 10s override.
	time.Sleep(4 * time.Second)

	// The prefix palette should still be visible (contains "choices").
	output := session.ScreenASCII()
	if !strings.Contains(output, "choices") {
		t.Fatal("prefix palette disappeared before custom timeout — AMUX_PREFIX_TIMEOUT not respected")
	}

	// Clean exit via prefix "q" sequence
	if err := session.SendString("q"); err != nil {
		t.Fatalf("send q: %v", err)
	}
	waitForUIContains(t, session, "Quit AMUX", persistenceTimeout)
	if err := session.SendString("\r"); err != nil {
		t.Fatalf("confirm quit: %v", err)
	}
	if err := session.WaitForExit(persistenceTimeout); err != nil {
		t.Fatalf("waiting for exit: %v", err)
	}
}
