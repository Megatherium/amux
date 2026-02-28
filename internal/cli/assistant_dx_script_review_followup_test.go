package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssistantDXReview_AutoContinuesFixOfferPromptToFinalFindingsWhenEnabled(t *testing.T) {
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
  *)
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
esac
`)

	writeExecutable(t, fakeTurnPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${TURN_ARGS_LOG:?missing TURN_ARGS_LOG}"
if [[ "${1:-}" == "run" ]]; then
  printf '%s' '{"ok":true,"mode":"run","status":"needs_input","overall_status":"needs_input","summary":"Would you like me to fix any of these issues?","agent_id":"ws-1:t_dgq1foo_1_abc12345","workspace_id":"ws-1","assistant":"codex","next_action":"Needs answer.","suggested_command":"","quick_actions":[],"channel":{"message":"needs input","chunks":["needs input"],"chunks_meta":[{"index":1,"total":1,"text":"needs input"}]}}'
  exit 0
fi
printf '%s' '{"ok":true,"mode":"send","status":"idle","overall_status":"completed","summary":"Final review findings delivered.","agent_id":"ws-1:t_dgq1foo_1_abc12345","workspace_id":"ws-1","assistant":"codex","next_action":"Done.","suggested_command":"","quick_actions":[],"channel":{"message":"done","chunks":["done"],"chunks_meta":[{"index":1,"total":1,"text":"done"}]}}'
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "TURN_ARGS_LOG", turnArgsLog)
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY", "false")
	env = withEnv(env, "AMUX_ASSISTANT_DX_REVIEW_NEEDS_INPUT_AUTO_CONTINUE", "true")

	payload := runScriptJSON(t, scriptPath, env,
		"review",
		"--workspace", "ws-1",
		"--assistant", "codex",
	)

	if got, _ := payload["status"].(string); got != "idle" {
		t.Fatalf("status = %q, want %q", got, "idle")
	}
	summary, _ := payload["summary"].(string)
	if !strings.Contains(summary, "Final review findings delivered.") {
		t.Fatalf("summary = %q, want auto-continued final findings summary", summary)
	}

	turnArgsRaw, err := os.ReadFile(turnArgsLog)
	if err != nil {
		t.Fatalf("read turn args: %v", err)
	}
	turnArgs := string(turnArgsRaw)
	if !strings.Contains(turnArgs, "run --workspace ws-1 --assistant codex") {
		t.Fatalf("turn args = %q, expected initial run call", turnArgs)
	}
	if !strings.Contains(turnArgs, "send --agent ws-1:t_dgq1foo_1_abc12345") {
		t.Fatalf("turn args = %q, expected auto follow-up send call", turnArgs)
	}
	if !strings.Contains(turnArgs, "No code changes. Return final review findings only") {
		t.Fatalf("turn args = %q, expected default review auto-followup text", turnArgs)
	}
	lines := strings.Split(strings.TrimSpace(turnArgs), "\n")
	var runLine string
	var sendLine string
	for _, line := range lines {
		if strings.Contains(line, "run --workspace ws-1 --assistant codex") {
			runLine = line
		}
		if strings.Contains(line, "send --agent ws-1:t_dgq1foo_1_abc12345") {
			sendLine = line
		}
	}
	runKey := argValueFromLine(runLine, "--idempotency-key")
	if runKey == "" {
		t.Fatalf("run args line = %q, expected --idempotency-key", runLine)
	}
	if !strings.Contains(runKey, "dx-review-ws-1-codex-run-") {
		t.Fatalf("run idempotency key = %q, expected review run prefix", runKey)
	}
	sendKey := argValueFromLine(sendLine, "--idempotency-key")
	if sendKey == "" {
		t.Fatalf("send args line = %q, expected --idempotency-key", sendLine)
	}
	if !strings.Contains(sendKey, "dx-review-ws-1-codex-followup-") {
		t.Fatalf("send idempotency key = %q, expected review followup prefix", sendKey)
	}
	if runKey == sendKey {
		t.Fatalf("expected distinct idempotency keys for run/send, got %q", runKey)
	}
}

func TestAssistantDXReview_DoesNotAutoContinueNeedsInputByDefault(t *testing.T) {
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
  *)
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
esac
`)

	writeExecutable(t, fakeTurnPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${TURN_ARGS_LOG:?missing TURN_ARGS_LOG}"
if [[ "${1:-}" == "run" ]]; then
  printf '%s' '{"ok":true,"mode":"run","status":"needs_input","overall_status":"needs_input","summary":"Would you like me to fix any of these issues?","agent_id":"ws-1:t_dgq1foo_1_abc12345","workspace_id":"ws-1","assistant":"codex","next_action":"Needs answer.","suggested_command":"","quick_actions":[],"channel":{"message":"needs input","chunks":["needs input"],"chunks_meta":[{"index":1,"total":1,"text":"needs input"}]}}'
  exit 0
fi
printf '%s' '{"ok":true,"mode":"send","status":"idle","overall_status":"completed","summary":"should not send","agent_id":"ws-1:t_dgq1foo_1_abc12345","workspace_id":"ws-1","assistant":"codex","next_action":"Done.","suggested_command":"","quick_actions":[],"channel":{"message":"done","chunks":["done"],"chunks_meta":[{"index":1,"total":1,"text":"done"}]}}'
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
	if !strings.Contains(summary, "Would you like me to fix any of these issues?") {
		t.Fatalf("summary = %q, want original needs_input summary", summary)
	}

	turnArgsRaw, err := os.ReadFile(turnArgsLog)
	if err != nil {
		t.Fatalf("read turn args: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(turnArgsRaw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("turn args lines = %q, expected single run without auto-followup send", string(turnArgsRaw))
	}
	if !strings.Contains(lines[0], "run --workspace ws-1 --assistant codex") {
		t.Fatalf("turn args line = %q, expected initial run call", lines[0])
	}
}
