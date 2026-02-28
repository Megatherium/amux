package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssistantDXReview_UsesReviewDefaultsAndRecoversTimedOutTurn(t *testing.T) {
	requireBinary(t, "jq")
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dx.sh")
	fakeBinDir := t.TempDir()
	fakeAmuxPath := filepath.Join(fakeBinDir, "amux")
	fakeTurnPath := filepath.Join(fakeBinDir, "fake-turn.sh")
	fakeStepPath := filepath.Join(fakeBinDir, "fake-step.sh")
	turnArgsLog := filepath.Join(fakeBinDir, "turn-args.log")
	stepArgsLog := filepath.Join(fakeBinDir, "step-args.log")

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
printf '%s\n' "$*" > "${TURN_ARGS_LOG:?missing TURN_ARGS_LOG}"
printf '%s' '{"ok":true,"mode":"run","status":"timed_out","overall_status":"timed_out","summary":"Review is still running.","agent_id":"ws-1:tab-1","workspace_id":"ws-1","assistant":"codex","next_action":"Continue review.","suggested_command":"","quick_actions":[],"channel":{"message":"timed out","chunks":["timed out"],"chunks_meta":[{"index":1,"total":1,"text":"timed out"}]}}'
`)

	writeExecutable(t, fakeStepPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" > "${STEP_ARGS_LOG:?missing STEP_ARGS_LOG}"
printf '%s' '{"ok":true,"mode":"send","status":"idle","summary":"Recovered review findings.","agent_id":"ws-1:tab-1","workspace_id":"ws-1","assistant":"codex","response":{"substantive_output":true,"changed":true,"needs_input":false},"next_action":"Ship if clean.","suggested_command":"","quick_actions":[],"channel":{"message":"done","chunks":["done"],"chunks_meta":[{"index":1,"total":1,"text":"done"}]}}'
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "AMUX_ASSISTANT_DX_STEP_SCRIPT", fakeStepPath)
	env = withEnv(env, "TURN_ARGS_LOG", turnArgsLog)
	env = withEnv(env, "STEP_ARGS_LOG", stepArgsLog)
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY", "true")
	env = withEnv(env, "AMUX_ASSISTANT_DX_REVIEW_TIMEOUT_RECOVERY", "true")
	env = withEnv(env, "AMUX_ASSISTANT_DX_REVIEW_TIMEOUT_RECOVERY_SEND_FALLBACK", "true")

	payload := runScriptJSON(t, scriptPath, env,
		"review",
		"--workspace", "ws-1",
		"--assistant", "codex",
	)

	if got, _ := payload["status"].(string); got != "idle" {
		t.Fatalf("status = %q, want %q", got, "idle")
	}
	summary, _ := payload["summary"].(string)
	if !strings.Contains(summary, "Recovered review findings.") {
		t.Fatalf("summary = %q, want recovery summary", summary)
	}

	turnArgsRaw, err := os.ReadFile(turnArgsLog)
	if err != nil {
		t.Fatalf("read turn args: %v", err)
	}
	turnArgs := strings.TrimSpace(string(turnArgsRaw))
	if !strings.Contains(turnArgs, "--turn-budget 120") {
		t.Fatalf("turn args = %q, expected default review turn budget", turnArgs)
	}
	if !strings.Contains(turnArgs, "--wait-timeout 30s") {
		t.Fatalf("turn args = %q, expected default review wait timeout", turnArgs)
	}
	if !strings.Contains(turnArgs, "--idempotency-key dx-review-ws-1-codex-run-") {
		t.Fatalf("turn args = %q, expected deterministic review run idempotency key", turnArgs)
	}

	stepArgsRaw, err := os.ReadFile(stepArgsLog)
	if err != nil {
		t.Fatalf("read step args: %v", err)
	}
	stepArgs := strings.TrimSpace(string(stepArgsRaw))
	if !strings.Contains(stepArgs, "send --agent ws-1:tab-1") {
		t.Fatalf("step args = %q, expected timeout recovery send", stepArgs)
	}
	if !strings.Contains(stepArgs, "--wait-timeout 15s") {
		t.Fatalf("step args = %q, expected bounded review recovery wait timeout", stepArgs)
	}
}

func TestAssistantDXReview_BoundedRecoveryWaitIgnoresLongReviewWaitTimeout(t *testing.T) {
	requireBinary(t, "jq")
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dx.sh")
	fakeBinDir := t.TempDir()
	fakeAmuxPath := filepath.Join(fakeBinDir, "amux")
	fakeTurnPath := filepath.Join(fakeBinDir, "fake-turn.sh")
	fakeStepPath := filepath.Join(fakeBinDir, "fake-step.sh")
	stepArgsLog := filepath.Join(fakeBinDir, "step-args.log")

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
printf '%s' '{"ok":true,"mode":"run","status":"timed_out","overall_status":"timed_out","summary":"Review is still running.","agent_id":"ws-1:tab-1","workspace_id":"ws-1","assistant":"codex","next_action":"Continue review.","suggested_command":"","quick_actions":[],"channel":{"message":"timed out","chunks":["timed out"],"chunks_meta":[{"index":1,"total":1,"text":"timed out"}]}}'
`)

	writeExecutable(t, fakeStepPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" > "${STEP_ARGS_LOG:?missing STEP_ARGS_LOG}"
printf '%s' '{"ok":true,"mode":"send","status":"idle","summary":"Recovered review findings.","agent_id":"ws-1:tab-1","workspace_id":"ws-1","assistant":"codex","response":{"substantive_output":true,"changed":true,"needs_input":false},"next_action":"Ship if clean.","suggested_command":"","quick_actions":[],"channel":{"message":"done","chunks":["done"],"chunks_meta":[{"index":1,"total":1,"text":"done"}]}}'
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "AMUX_ASSISTANT_DX_STEP_SCRIPT", fakeStepPath)
	env = withEnv(env, "STEP_ARGS_LOG", stepArgsLog)
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY", "true")
	env = withEnv(env, "AMUX_ASSISTANT_DX_REVIEW_TIMEOUT_RECOVERY", "true")
	env = withEnv(env, "AMUX_ASSISTANT_DX_REVIEW_TIMEOUT_RECOVERY_SEND_FALLBACK", "true")

	_ = runScriptJSON(t, scriptPath, env,
		"review",
		"--workspace", "ws-1",
		"--assistant", "codex",
		"--wait-timeout", "75s",
	)

	stepArgsRaw, err := os.ReadFile(stepArgsLog)
	if err != nil {
		t.Fatalf("read step args: %v", err)
	}
	stepArgs := strings.TrimSpace(string(stepArgsRaw))
	if !strings.Contains(stepArgs, "--wait-timeout 15s") {
		t.Fatalf("step args = %q, expected bounded recovery wait timeout", stepArgs)
	}
}

func TestAssistantDXReview_PassiveTimeoutRecoveryPrefersSessionWaitCapture(t *testing.T) {
	requireBinary(t, "jq")
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dx.sh")
	fakeBinDir := t.TempDir()
	fakeAmuxPath := filepath.Join(fakeBinDir, "amux")
	fakeTurnPath := filepath.Join(fakeBinDir, "fake-turn.sh")
	fakeStepPath := filepath.Join(fakeBinDir, "fake-step.sh")
	fakeWaitPath := filepath.Join(fakeBinDir, "fake-wait-for-idle.sh")
	waitArgsLog := filepath.Join(fakeBinDir, "wait-args.log")
	stepArgsLog := filepath.Join(fakeBinDir, "step-args.log")

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
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-1","agent_id":"ws-1:tab-1","workspace_id":"ws-1","tab_id":"tab-1","assistant":"codex"}],"error":null}'
    ;;
  "agent capture")
    printf '%s' '{"ok":true,"data":{"session_name":"sess-1","status":"captured","latest_line":"All checks passed.","summary":"All checks passed.","content":"Implemented updates\nAll checks passed.","needs_input":false,"input_hint":""},"error":null}'
    ;;
  *)
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
esac
`)

	writeExecutable(t, fakeTurnPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s' '{"ok":true,"mode":"run","status":"timed_out","overall_status":"timed_out","summary":"Review is still running.","agent_id":"ws-1:tab-1","workspace_id":"ws-1","assistant":"codex","next_action":"Continue review.","suggested_command":"","quick_actions":[],"channel":{"message":"timed out","chunks":["timed out"],"chunks_meta":[{"index":1,"total":1,"text":"timed out"}]}}'
`)

	writeExecutable(t, fakeStepPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${STEP_ARGS_LOG:?missing STEP_ARGS_LOG}"
printf '%s' '{"ok":true,"mode":"send","status":"idle","summary":"step fallback should not run","agent_id":"ws-1:tab-1","workspace_id":"ws-1","assistant":"codex","response":{"substantive_output":true,"changed":true,"needs_input":false},"next_action":"","suggested_command":"","quick_actions":[],"channel":{"message":"step","chunks":["step"],"chunks_meta":[{"index":1,"total":1,"text":"step"}]}}'
`)

	writeExecutable(t, fakeWaitPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${WAIT_ARGS_LOG:?missing WAIT_ARGS_LOG}"
exit 0
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "AMUX_ASSISTANT_DX_STEP_SCRIPT", fakeStepPath)
	env = withEnv(env, "AMUX_ASSISTANT_DX_WAIT_FOR_IDLE_SCRIPT", fakeWaitPath)
	env = withEnv(env, "WAIT_ARGS_LOG", waitArgsLog)
	env = withEnv(env, "STEP_ARGS_LOG", stepArgsLog)
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY_PASSIVE_WAIT_SECONDS", "30")
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY", "true")
	env = withEnv(env, "AMUX_ASSISTANT_DX_REVIEW_TIMEOUT_RECOVERY", "true")

	payload := runScriptJSON(t, scriptPath, env,
		"review",
		"--workspace", "ws-1",
		"--assistant", "codex",
	)

	if got, _ := payload["status"].(string); got != "idle" {
		t.Fatalf("status = %q, want %q", got, "idle")
	}
	summary, _ := payload["summary"].(string)
	if !strings.Contains(summary, "All checks passed.") {
		t.Fatalf("summary = %q, want passive-capture summary", summary)
	}

	waitArgsRaw, err := os.ReadFile(waitArgsLog)
	if err != nil {
		t.Fatalf("read wait args: %v", err)
	}
	waitArgs := strings.TrimSpace(string(waitArgsRaw))
	if !strings.Contains(waitArgs, "--session sess-1") {
		t.Fatalf("wait args = %q, expected session wait", waitArgs)
	}

	if stepArgsRaw, err := os.ReadFile(stepArgsLog); err == nil {
		if strings.TrimSpace(string(stepArgsRaw)) != "" {
			t.Fatalf("step fallback should not run when passive recovery succeeds: %q", string(stepArgsRaw))
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("read step args: %v", err)
	}
}

func TestAssistantDXReview_PassiveTimeoutRecoveryDefaultWaitIsShortAndBounded(t *testing.T) {
	requireBinary(t, "jq")
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dx.sh")
	fakeBinDir := t.TempDir()
	fakeAmuxPath := filepath.Join(fakeBinDir, "amux")
	fakeTurnPath := filepath.Join(fakeBinDir, "fake-turn.sh")
	fakeStepPath := filepath.Join(fakeBinDir, "fake-step.sh")
	fakeWaitPath := filepath.Join(fakeBinDir, "fake-wait-for-idle.sh")
	waitArgsLog := filepath.Join(fakeBinDir, "wait-args.log")

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
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-1","agent_id":"ws-1:t_dgq1foo_1_abc12345","workspace_id":"ws-1","tab_id":"t_dgq1foo_1_abc12345","type":"agent"}],"error":null}'
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
printf '%s' '{"ok":true,"mode":"run","status":"timed_out","overall_status":"timed_out","summary":"Review is still running.","agent_id":"ws-1:t_dgq1foo_1_abc12345","workspace_id":"ws-1","assistant":"codex","next_action":"Continue review.","suggested_command":"","quick_actions":[],"channel":{"message":"timed out","chunks":["timed out"],"chunks_meta":[{"index":1,"total":1,"text":"timed out"}]}}'
`)

	writeExecutable(t, fakeStepPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s' '{"ok":true,"mode":"send","status":"idle","summary":"Recovered after passive wait.","agent_id":"ws-1:t_dgq1foo_1_abc12345","workspace_id":"ws-1","assistant":"codex","response":{"substantive_output":true,"changed":true,"needs_input":false},"next_action":"","suggested_command":"","quick_actions":[],"channel":{"message":"done","chunks":["done"],"chunks_meta":[{"index":1,"total":1,"text":"done"}]}}'
`)

	writeExecutable(t, fakeWaitPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${WAIT_ARGS_LOG:?missing WAIT_ARGS_LOG}"
exit 1
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "AMUX_ASSISTANT_DX_STEP_SCRIPT", fakeStepPath)
	env = withEnv(env, "AMUX_ASSISTANT_DX_WAIT_FOR_IDLE_SCRIPT", fakeWaitPath)
	env = withEnv(env, "WAIT_ARGS_LOG", waitArgsLog)
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY", "true")
	env = withEnv(env, "AMUX_ASSISTANT_DX_REVIEW_TIMEOUT_RECOVERY", "true")

	_ = runScriptJSON(t, scriptPath, env,
		"review",
		"--workspace", "ws-1",
		"--assistant", "codex",
		"--wait-timeout", "60s",
		"--allow-new-run",
	)

	waitArgsRaw, err := os.ReadFile(waitArgsLog)
	if err != nil {
		t.Fatalf("read wait args: %v", err)
	}
	waitArgs := strings.TrimSpace(string(waitArgsRaw))
	if !strings.Contains(waitArgs, "--timeout 30") {
		t.Fatalf("wait args = %q, expected short default passive wait timeout", waitArgs)
	}
}

func TestAssistantDXReview_RepeatedRunsReuseDeterministicRunIdempotencyKey(t *testing.T) {
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
printf '%s' '{"ok":true,"mode":"run","status":"idle","overall_status":"completed","summary":"review done","agent_id":"ws-1:tab-1","workspace_id":"ws-1","assistant":"codex","next_action":"","suggested_command":"","quick_actions":[],"channel":{"message":"done","chunks":["done"],"chunks_meta":[{"index":1,"total":1,"text":"done"}]}}'
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "TURN_ARGS_LOG", turnArgsLog)
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY", "false")
	env = withEnv(env, "AMUX_ASSISTANT_DX_REVIEW_IDEMPOTENCY_WINDOW_SECONDS", "3600")

	_ = runScriptJSON(t, scriptPath, env,
		"review",
		"--workspace", "ws-1",
		"--assistant", "codex",
	)
	_ = runScriptJSON(t, scriptPath, env,
		"review",
		"--workspace", "ws-1",
		"--assistant", "codex",
	)

	turnArgsRaw, err := os.ReadFile(turnArgsLog)
	if err != nil {
		t.Fatalf("read turn args: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(turnArgsRaw)), "\n")
	if len(lines) < 2 {
		t.Fatalf("turn args lines = %q, expected at least two review runs", string(turnArgsRaw))
	}
	runKeyFirst := argValueFromLine(lines[0], "--idempotency-key")
	runKeySecond := argValueFromLine(lines[1], "--idempotency-key")
	if runKeyFirst == "" || runKeySecond == "" {
		t.Fatalf("turn args lines = %q, expected --idempotency-key in both runs", string(turnArgsRaw))
	}
	if runKeyFirst != runKeySecond {
		t.Fatalf("run idempotency key should be stable across repeats, got %q vs %q", runKeyFirst, runKeySecond)
	}
}
