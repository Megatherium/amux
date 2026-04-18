package e2e

import (
	"fmt"
	"path/filepath"
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
