package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssistantDXReview_InfersNewInflightAgentAfterHardTimeoutWithoutAgentID(t *testing.T) {
	requireBinary(t, "jq")
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dx.sh")
	fakeBinDir := t.TempDir()
	fakeAmuxPath := filepath.Join(fakeBinDir, "amux")
	fakeTurnPath := filepath.Join(fakeBinDir, "fake-turn.sh")
	turnArgsLog := filepath.Join(fakeBinDir, "turn-args.log")
	workspaceListCount := filepath.Join(fakeBinDir, "agent-list-workspace.count")

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
    if [[ " $* " == *" --workspace ws-1 "* ]]; then
      count=0
      if [[ -f "${WORKSPACE_LIST_COUNT_FILE:?missing WORKSPACE_LIST_COUNT_FILE}" ]]; then
        count="$(cat "${WORKSPACE_LIST_COUNT_FILE}")"
      fi
      count=$((count + 1))
      printf '%s' "$count" > "${WORKSPACE_LIST_COUNT_FILE}"
      if [[ "$count" -eq 1 ]]; then
        printf '%s' '{"ok":true,"data":[{"session_name":"sess-old","agent_id":"ws-1:t_old","workspace_id":"ws-1","tab_id":"t_old","assistant":"codex"}],"error":null}'
      else
        printf '%s' '{"ok":true,"data":[{"session_name":"sess-old","agent_id":"ws-1:t_old","workspace_id":"ws-1","tab_id":"t_old","assistant":"codex"},{"session_name":"sess-new","agent_id":"ws-1:t_new","workspace_id":"ws-1","tab_id":"t_new","assistant":"codex"}],"error":null}'
      fi
    else
      printf '%s' '{"ok":true,"data":[{"session_name":"sess-old","agent_id":"ws-1:t_old","workspace_id":"ws-1","tab_id":"t_old","assistant":"codex"},{"session_name":"sess-new","agent_id":"ws-1:t_new","workspace_id":"ws-1","tab_id":"t_new","assistant":"codex"}],"error":null}'
    fi
    ;;
  "agent capture")
    if [[ "${3:-}" == "sess-new" ]]; then
      printf '%s' '{"ok":true,"data":{"session_name":"sess-new","status":"captured","latest_line":"Review running...","summary":"Review running...","content":"Review running...","needs_input":false,"input_hint":""},"error":null}'
    else
      printf '%s' '{"ok":true,"data":{"session_name":"sess-old","status":"session_exited","latest_line":"","summary":"","content":"","needs_input":false,"input_hint":""},"error":null}'
    fi
    ;;
  *)
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
esac
`)

	writeExecutable(t, fakeTurnPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${TURN_ARGS_LOG:?missing TURN_ARGS_LOG}"
printf '%s' '{"ok":true,"mode":"run","status":"command_error","overall_status":"partial","summary":"Partial after 1 step(s). amux command exceeded hard timeout","agent_id":"","workspace_id":"ws-1","assistant":"codex","next_action":"","suggested_command":"","quick_actions":[],"channel":{"message":"partial","chunks":["partial"],"chunks_meta":[{"index":1,"total":1,"text":"partial"}]}}'
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "TURN_ARGS_LOG", turnArgsLog)
	env = withEnv(env, "WORKSPACE_LIST_COUNT_FILE", workspaceListCount)
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY", "false")

	payload := runScriptJSON(t, scriptPath, env,
		"review",
		"--workspace", "ws-1",
		"--assistant", "codex",
		"--allow-new-run",
	)

	if got, _ := payload["status"].(string); got != "attention" {
		t.Fatalf("status = %q, want %q", got, "attention")
	}
	if got, _ := payload["overall_status"].(string); got != "in_progress" {
		t.Fatalf("overall_status = %q, want %q", got, "in_progress")
	}
	if got, _ := payload["agent_id"].(string); got != "ws-1:t_new" {
		t.Fatalf("agent_id = %q, want %q", got, "ws-1:t_new")
	}
	summary, _ := payload["summary"].(string)
	if !strings.Contains(summary, "Review is still running") {
		t.Fatalf("summary = %q, want running-review guidance", summary)
	}

	turnArgsRaw, err := os.ReadFile(turnArgsLog)
	if err != nil {
		t.Fatalf("read turn args: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(turnArgsRaw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("turn script should run exactly once; got %q", string(turnArgsRaw))
	}
}

func TestAssistantDXReview_InfersExistingInflightAgentAfterHardTimeoutWithoutAgentID(t *testing.T) {
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
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-existing","agent_id":"ws-1:t_existing","workspace_id":"ws-1","tab_id":"t_existing","assistant":"codex"}],"error":null}'
    ;;
  "agent capture")
    printf '%s' '{"ok":true,"data":{"session_name":"sess-existing","status":"captured","latest_line":"Review running...","summary":"Review running...","content":"Review running...","needs_input":false,"input_hint":""},"error":null}'
    ;;
  *)
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
esac
`)

	writeExecutable(t, fakeTurnPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${TURN_ARGS_LOG:?missing TURN_ARGS_LOG}"
printf '%s' '{"ok":true,"mode":"run","status":"command_error","overall_status":"partial","summary":"Partial after 1 step(s). amux command exceeded hard timeout","agent_id":"","workspace_id":"ws-1","assistant":"codex","next_action":"","suggested_command":"","quick_actions":[],"channel":{"message":"partial","chunks":["partial"],"chunks_meta":[{"index":1,"total":1,"text":"partial"}]}}'
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "TURN_ARGS_LOG", turnArgsLog)
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY", "false")

	payload := runScriptJSON(t, scriptPath, env,
		"review",
		"--workspace", "ws-1",
		"--assistant", "codex",
		"--allow-new-run",
	)

	if got, _ := payload["status"].(string); got != "attention" {
		t.Fatalf("status = %q, want %q", got, "attention")
	}
	if got, _ := payload["overall_status"].(string); got != "in_progress" {
		t.Fatalf("overall_status = %q, want %q", got, "in_progress")
	}
	if got, _ := payload["agent_id"].(string); got != "ws-1:t_existing" {
		t.Fatalf("agent_id = %q, want %q", got, "ws-1:t_existing")
	}
	summary, _ := payload["summary"].(string)
	if !strings.Contains(strings.ToLower(summary), "still running") {
		t.Fatalf("summary = %q, want running-review guidance", summary)
	}

	turnArgsRaw, err := os.ReadFile(turnArgsLog)
	if err != nil {
		t.Fatalf("read turn args: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(turnArgsRaw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("turn script should run exactly once; got %q", string(turnArgsRaw))
	}
}
