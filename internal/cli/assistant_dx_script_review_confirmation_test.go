package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssistantDXReview_ActiveAgentRequiresConfirmationByDefault(t *testing.T) {
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
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-1","agent_id":"ws-1:t_existing","workspace_id":"ws-1","tab_id":"t_existing","assistant":"codex"}],"error":null}'
    ;;
  "agent capture")
    printf '%s' '{"ok":true,"data":{"session_name":"sess-1","status":"captured","latest_line":"Running /review now","summary":"Running /review now","content":"Running /review now","needs_input":false,"input_hint":""},"error":null}'
    ;;
  *)
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
esac
`)

	writeExecutable(t, fakeTurnPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${TURN_ARGS_LOG:?missing TURN_ARGS_LOG}"
printf '%s' '{"ok":true,"mode":"run","status":"idle","overall_status":"completed","summary":"turn should not run","agent_id":"ws-1:t_new","workspace_id":"ws-1","assistant":"codex","next_action":"","suggested_command":"","quick_actions":[],"channel":{"message":"done","chunks":["done"],"chunks_meta":[{"index":1,"total":1,"text":"done"}]}}'
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
	)

	if got, _ := payload["status"].(string); got != "needs_input" {
		t.Fatalf("status = %q, want %q", got, "needs_input")
	}
	summary, _ := payload["summary"].(string)
	if !strings.Contains(strings.ToLower(summary), "not started automatically") {
		t.Fatalf("summary = %q, want confirmation guidance", summary)
	}

	channel, _ := payload["channel"].(map[string]any)
	message, _ := channel["message"].(string)
	if !strings.Contains(message, "Last agent line: Running /review now") {
		t.Fatalf("channel.message = %q, want last agent line", message)
	}

	if turnArgsRaw, err := os.ReadFile(turnArgsLog); err == nil {
		if strings.TrimSpace(string(turnArgsRaw)) != "" {
			t.Fatalf("turn script should not run without explicit confirmation: %q", string(turnArgsRaw))
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("read turn args: %v", err)
	}
}

func TestAssistantDXReview_AllowNewRunBypassesActiveAgentConfirmation(t *testing.T) {
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
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-1","agent_id":"ws-1:t_existing","workspace_id":"ws-1","tab_id":"t_existing","assistant":"codex"}],"error":null}'
    ;;
  "agent capture")
    printf '%s' '{"ok":true,"data":{"session_name":"sess-1","status":"captured","latest_line":"Running /review now","summary":"Running /review now","content":"Running /review now","needs_input":false,"input_hint":""},"error":null}'
    ;;
  *)
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
esac
`)

	writeExecutable(t, fakeTurnPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" > "${TURN_ARGS_LOG:?missing TURN_ARGS_LOG}"
printf '%s' '{"ok":true,"mode":"run","status":"idle","overall_status":"completed","summary":"review done","agent_id":"ws-1:t_new","workspace_id":"ws-1","assistant":"codex","next_action":"","suggested_command":"","quick_actions":[],"channel":{"message":"done","chunks":["done"],"chunks_meta":[{"index":1,"total":1,"text":"done"}]}}'
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

	if got, _ := payload["status"].(string); got != "idle" {
		t.Fatalf("status = %q, want %q", got, "idle")
	}

	turnArgsRaw, err := os.ReadFile(turnArgsLog)
	if err != nil {
		t.Fatalf("read turn args: %v", err)
	}
	turnArgs := strings.TrimSpace(string(turnArgsRaw))
	if !strings.Contains(turnArgs, "run --workspace ws-1 --assistant codex") {
		t.Fatalf("turn args = %q, expected review run call", turnArgs)
	}
}
