package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssistantDXStart_TimedOutActiveSessionReportsInProgress(t *testing.T) {
	requireBinary(t, "jq")
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dx.sh")
	fakeBinDir := t.TempDir()
	fakeAmuxPath := filepath.Join(fakeBinDir, "amux")
	fakeTurnPath := filepath.Join(fakeBinDir, "fake-turn.sh")

	writeExecutable(t, fakeAmuxPath, `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "--json" ]]; then
  shift
fi
case "${1:-} ${2:-}" in
  "workspace list")
    printf '%s' '{"ok":true,"data":[{"id":"ws-1","name":"mobile","repo":"/tmp/demo","root":"/tmp/ws-1","assistant":"codex"}],"error":null}'
    ;;
  "agent list")
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-1","agent_id":"ws-1:t_inflight","workspace_id":"ws-1","tab_id":"t_inflight","assistant":"codex"}],"error":null}'
    ;;
  "agent capture")
    printf '%s' '{"ok":true,"data":{"session_name":"sess-1","status":"captured","latest_line":"","summary":"","content":"","needs_input":false,"input_hint":""},"error":null}'
    ;;
  *)
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
esac
`)

	writeExecutable(t, fakeTurnPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s' '{"ok":true,"mode":"run","status":"timed_out","overall_status":"timed_out","summary":"Timed out waiting for first output.","agent_id":"ws-1:t_inflight","workspace_id":"ws-1","assistant":"codex","next_action":"Continue review.","suggested_command":"","quick_actions":[],"channel":{"message":"timed out","chunks":["timed out"],"chunks_meta":[{"index":1,"total":1,"text":"timed out"}]}}'
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY", "false")

	payload := runScriptJSON(t, scriptPath, env,
		"start",
		"--workspace", "ws-1",
		"--assistant", "codex",
		"--prompt", "Review uncommitted changes.",
	)

	if got, _ := payload["status"].(string); got != "attention" {
		t.Fatalf("status = %q, want %q", got, "attention")
	}
	if got, _ := payload["overall_status"].(string); got != "in_progress" {
		t.Fatalf("overall_status = %q, want %q", got, "in_progress")
	}
	summary, _ := payload["summary"].(string)
	if !strings.Contains(strings.ToLower(summary), "still running") {
		t.Fatalf("summary = %q, want running guidance", summary)
	}
}

func TestAssistantDXStart_ReusesInflightAgentWithoutSecondTurnRun(t *testing.T) {
	requireBinary(t, "jq")
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dx.sh")
	fakeBinDir := t.TempDir()
	fakeAmuxPath := filepath.Join(fakeBinDir, "amux")
	fakeTurnPath := filepath.Join(fakeBinDir, "fake-turn.sh")
	turnArgsLog := filepath.Join(fakeBinDir, "turn-args.log")

	writeExecutable(t, fakeAmuxPath, `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "--json" ]]; then
  shift
fi
case "${1:-} ${2:-}" in
  "workspace list")
    printf '%s' '{"ok":true,"data":[{"id":"ws-1","name":"mobile","repo":"/tmp/demo","root":"/tmp/ws-1","assistant":"codex"}],"error":null}'
    ;;
  "agent list")
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-1","agent_id":"ws-1:t_inflight","workspace_id":"ws-1","tab_id":"t_inflight","assistant":"codex"}],"error":null}'
    ;;
  "agent capture")
    printf '%s' '{"ok":true,"data":{"session_name":"sess-1","status":"captured","latest_line":"Working...","summary":"Working...","content":"Working...","needs_input":false,"input_hint":""},"error":null}'
    ;;
  *)
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
esac
`)

	writeExecutable(t, fakeTurnPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${TURN_ARGS_LOG:?missing TURN_ARGS_LOG}"
printf '%s' '{"ok":true,"mode":"run","status":"timed_out","overall_status":"timed_out","summary":"Timed out waiting for first output.","agent_id":"ws-1:t_inflight","workspace_id":"ws-1","assistant":"codex","next_action":"Continue.","suggested_command":"","quick_actions":[],"channel":{"message":"timed out","chunks":["timed out"],"chunks_meta":[{"index":1,"total":1,"text":"timed out"}]}}'
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "TURN_ARGS_LOG", turnArgsLog)
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY", "false")

	_ = runScriptJSON(t, scriptPath, env,
		"start",
		"--workspace", "ws-1",
		"--assistant", "codex",
		"--prompt", "Review uncommitted changes.",
	)
	second := runScriptJSON(t, scriptPath, env,
		"start",
		"--workspace", "ws-1",
		"--assistant", "codex",
		"--prompt", "Review uncommitted changes.",
	)

	if got, _ := second["status"].(string); got != "attention" {
		t.Fatalf("second status = %q, want %q", got, "attention")
	}
	summary, _ := second["summary"].(string)
	if !strings.Contains(strings.ToLower(summary), "already running") {
		t.Fatalf("second summary = %q, want already-running guidance", summary)
	}

	turnArgsRaw, err := os.ReadFile(turnArgsLog)
	if err != nil {
		t.Fatalf("read turn args: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(turnArgsRaw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("turn invocations = %d, want 1. lines=%q", len(lines), string(turnArgsRaw))
	}
}
