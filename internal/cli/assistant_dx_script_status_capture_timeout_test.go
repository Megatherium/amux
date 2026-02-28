package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestAssistantDXStatus_BoundsSlowCaptureCalls(t *testing.T) {
	requireBinary(t, "jq")
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dx.sh")
	fakeBinDir := t.TempDir()
	fakeAmuxPath := filepath.Join(fakeBinDir, "amux")

	writeExecutable(t, fakeAmuxPath, `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "--json" ]]; then
  shift
fi
case "${1:-} ${2:-}" in
  "project list")
    printf '%s' '{"ok":true,"data":[{"name":"demo","path":"/tmp/demo"}],"error":null}'
    ;;
  "workspace list")
    printf '%s' '{"ok":true,"data":[{"id":"ws-main","name":"mainline","repo":"/tmp/demo","scope":"project","created":"2026-01-01T00:00:00Z"}],"error":null}'
    ;;
  "agent list")
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-main","agent_id":"agent-main","workspace_id":"ws-main","tab_id":"tab-1","type":"agent"}],"error":null}'
    ;;
  "terminal list")
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
  "session list")
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-main"}],"error":null}'
    ;;
  "session prune")
    printf '%s' '{"ok":true,"data":{"dry_run":true,"pruned":[],"total":0,"errors":[]},"error":null}'
    ;;
  "agent capture")
    sleep 5
    printf '%s' '{"ok":true,"data":{"session_name":"sess-main","status":"captured","summary":"slow capture","needs_input":false,"input_hint":""},"error":null}'
    ;;
  *)
    printf '{"ok":false,"error":{"code":"unexpected","message":"unexpected args: %s"}}' "$*"
    ;;
esac
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_CAPTURE_TIMEOUT_SECONDS", "1")

	start := time.Now()
	payload := runScriptJSON(t, scriptPath, env, "status")
	elapsed := time.Since(start)
	if elapsed > 4*time.Second {
		t.Fatalf("status took %s with bounded capture timeout; want <= 4s", elapsed)
	}
	if got, _ := payload["status"].(string); got == "" {
		t.Fatalf("status missing in payload: %#v", payload)
	}
}

func TestAssistantDXStatus_CaptureTimeoutDoesNotLeaveCaptureProcessRunning(t *testing.T) {
	requireBinary(t, "jq")
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dx.sh")
	fakeBinDir := t.TempDir()
	fakeAmuxPath := filepath.Join(fakeBinDir, "amux")
	capturePIDPath := filepath.Join(fakeBinDir, "capture.pid")

	writeExecutable(t, fakeAmuxPath, `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "--json" ]]; then
  shift
fi
case "${1:-} ${2:-}" in
  "project list")
    printf '%s' '{"ok":true,"data":[{"name":"demo","path":"/tmp/demo"}],"error":null}'
    ;;
  "workspace list")
    printf '%s' '{"ok":true,"data":[{"id":"ws-main","name":"mainline","repo":"/tmp/demo","scope":"project","created":"2026-01-01T00:00:00Z"}],"error":null}'
    ;;
  "agent list")
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-main","agent_id":"agent-main","workspace_id":"ws-main","tab_id":"tab-1","type":"agent"}],"error":null}'
    ;;
  "terminal list")
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
  "session list")
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-main"}],"error":null}'
    ;;
  "session prune")
    printf '%s' '{"ok":true,"data":{"dry_run":true,"pruned":[],"total":0,"errors":[]},"error":null}'
    ;;
  "agent capture")
    printf '%s' "$$" > "${CAPTURE_PID_FILE:?missing CAPTURE_PID_FILE}"
    sleep 30
    printf '%s' '{"ok":true,"data":{"session_name":"sess-main","status":"captured","summary":"late capture","needs_input":false,"input_hint":""},"error":null}'
    ;;
  *)
    printf '{"ok":false,"error":{"code":"unexpected","message":"unexpected args: %s"}}' "$*"
    ;;
esac
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_CAPTURE_TIMEOUT_SECONDS", "1")
	env = withEnv(env, "CAPTURE_PID_FILE", capturePIDPath)

	_ = runScriptJSON(t, scriptPath, env, "status")

	pidRaw, err := os.ReadFile(capturePIDPath)
	if err != nil {
		t.Fatalf("read capture pid: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidRaw)))
	if err != nil || pid <= 0 {
		t.Fatalf("invalid capture pid %q: %v", strings.TrimSpace(string(pidRaw)), err)
	}

	// Give the watchdog a brief window to finish TERM/KILL cleanup.
	time.Sleep(150 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err == nil || err == syscall.EPERM {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("capture process still alive after timeout cleanup (pid=%d)", pid)
	}
}
