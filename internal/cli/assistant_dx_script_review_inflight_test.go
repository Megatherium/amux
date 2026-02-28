package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func argValueFromLine(argsLine, flag string) string {
	fields := strings.Fields(argsLine)
	for i := 0; i < len(fields)-1; i++ {
		if fields[i] == flag {
			return fields[i+1]
		}
	}
	return ""
}

func TestAssistantDXReview_PassiveRecoveryInfersNeedsInputFromPromptText(t *testing.T) {
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
  "agent list")
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-1","agent_id":"ws-1:t_dgq1foo_1_abc12345","workspace_id":"ws-1","tab_id":"t_dgq1foo_1_abc12345","type":"agent"}],"error":null}'
    ;;
  "agent capture")
    printf '%s' '{"ok":true,"data":{"session_name":"sess-1","status":"captured","latest_line":"Would you like me to fix #1 now?","summary":"Would you like me to fix #1 now?","content":"Would you like me to fix #1 now?","needs_input":false,"input_hint":""},"error":null}'
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
printf '%s\n' "$*" > "${STEP_ARGS_LOG:?missing STEP_ARGS_LOG}"
printf '%s' '{"ok":true,"mode":"send","status":"idle","summary":"step fallback should not run","agent_id":"ws-1:t_dgq1foo_1_abc12345","workspace_id":"ws-1","assistant":"codex","response":{"substantive_output":true,"changed":true,"needs_input":false},"next_action":"","suggested_command":"","quick_actions":[],"channel":{"message":"step","chunks":["step"],"chunks_meta":[{"index":1,"total":1,"text":"step"}]}}'
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "AMUX_ASSISTANT_DX_STEP_SCRIPT", fakeStepPath)
	env = withEnv(env, "STEP_ARGS_LOG", stepArgsLog)
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY", "true")
	env = withEnv(env, "AMUX_ASSISTANT_DX_REVIEW_TIMEOUT_RECOVERY", "true")
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY_PASSIVE_WAIT_SECONDS", "1")

	payload := runScriptJSON(t, scriptPath, env,
		"review",
		"--workspace", "ws-1",
		"--assistant", "codex",
		"--allow-new-run",
	)

	if got, _ := payload["status"].(string); got != "needs_input" {
		t.Fatalf("status = %q, want %q", got, "needs_input")
	}
	if stepArgsRaw, err := os.ReadFile(stepArgsLog); err == nil {
		if strings.TrimSpace(string(stepArgsRaw)) != "" {
			t.Fatalf("step fallback should not run when passive capture infers needs_input: %q", string(stepArgsRaw))
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("read step args: %v", err)
	}
}

func TestAssistantDXReview_TimedOutActiveSessionReportsInProgressGuidance(t *testing.T) {
	requireBinary(t, "jq")
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dx.sh")
	fakeBinDir := t.TempDir()
	fakeAmuxPath := filepath.Join(fakeBinDir, "amux")
	fakeTurnPath := filepath.Join(fakeBinDir, "fake-turn.sh")
	fakeStepPath := filepath.Join(fakeBinDir, "fake-step.sh")
	fakeWaitPath := filepath.Join(fakeBinDir, "fake-wait-for-idle.sh")
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
    printf '%s' '{"ok":true,"data":{"session_name":"sess-1","status":"captured","latest_line":"","summary":"","content":"","needs_input":false,"input_hint":""},"error":null}'
    ;;
  *)
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
esac
`)

	writeExecutable(t, fakeTurnPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s' '{"ok":true,"mode":"run","status":"timed_out","overall_status":"timed_out","summary":"Timed out waiting for first output.","agent_id":"ws-1:tab-1","workspace_id":"ws-1","assistant":"codex","next_action":"","suggested_command":"","quick_actions":[],"channel":{"message":"timed out","chunks":["timed out"],"chunks_meta":[{"index":1,"total":1,"text":"timed out"}]}}'
`)

	writeExecutable(t, fakeStepPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" > "${STEP_ARGS_LOG:?missing STEP_ARGS_LOG}"
printf '%s' '{"ok":true,"mode":"send","status":"idle","summary":"step fallback should not run","agent_id":"ws-1:tab-1","workspace_id":"ws-1","assistant":"codex","response":{"substantive_output":true,"changed":true,"needs_input":false},"next_action":"","suggested_command":"","quick_actions":[],"channel":{"message":"step","chunks":["step"],"chunks_meta":[{"index":1,"total":1,"text":"step"}]}}'
`)

	writeExecutable(t, fakeWaitPath, `#!/usr/bin/env bash
set -euo pipefail
exit 1
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "AMUX_ASSISTANT_DX_STEP_SCRIPT", fakeStepPath)
	env = withEnv(env, "AMUX_ASSISTANT_DX_WAIT_FOR_IDLE_SCRIPT", fakeWaitPath)
	env = withEnv(env, "STEP_ARGS_LOG", stepArgsLog)
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY", "true")
	env = withEnv(env, "AMUX_ASSISTANT_DX_REVIEW_TIMEOUT_RECOVERY", "true")
	env = withEnv(env, "AMUX_ASSISTANT_DX_REVIEW_TIMEOUT_RECOVERY_SEND_FALLBACK", "false")

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
	summary, _ := payload["summary"].(string)
	if !strings.Contains(strings.ToLower(summary), "still running") {
		t.Fatalf("summary = %q, want in-progress wording", summary)
	}
	nextAction, _ := payload["next_action"].(string)
	if !strings.Contains(nextAction, "re-check status") {
		t.Fatalf("next_action = %q, want status re-check guidance", nextAction)
	}
	suggested, _ := payload["suggested_command"].(string)
	if !strings.Contains(suggested, "assistant-dx.sh status --workspace ws-1") {
		t.Fatalf("suggested_command = %q, want workspace status command", suggested)
	}

	quickActions, ok := payload["quick_actions"].([]any)
	if !ok || len(quickActions) == 0 {
		t.Fatalf("quick_actions missing/empty: %#v", payload["quick_actions"])
	}
	var sawStatus bool
	var sawProgress bool
	for _, raw := range quickActions {
		action, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, _ := action["id"].(string)
		cmd, _ := action["command"].(string)
		if id == "status_ws" && strings.Contains(cmd, "status --workspace ws-1") {
			sawStatus = true
		}
		if id == "continue_status" && strings.Contains(cmd, "continue --agent ws-1:tab-1") {
			sawProgress = true
		}
	}
	if !sawStatus {
		t.Fatalf("expected status_ws quick action in %#v", quickActions)
	}
	if !sawProgress {
		t.Fatalf("expected continue_status quick action in %#v", quickActions)
	}

	if stepArgsRaw, err := os.ReadFile(stepArgsLog); err == nil {
		if strings.TrimSpace(string(stepArgsRaw)) != "" {
			t.Fatalf("step fallback should remain disabled for this path: %q", string(stepArgsRaw))
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("read step args: %v", err)
	}
}

func TestAssistantDXReview_ReusesInflightAgentWithoutStartingNewRun(t *testing.T) {
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
    printf '%s' '{"ok":true,"data":{"session_name":"sess-1","status":"captured","latest_line":"","summary":"","content":"","needs_input":false,"input_hint":""},"error":null}'
    ;;
  *)
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
esac
`)

	writeExecutable(t, fakeTurnPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${TURN_ARGS_LOG:?missing TURN_ARGS_LOG}"
printf '%s' '{"ok":true,"mode":"run","status":"timed_out","overall_status":"timed_out","summary":"Review is still running.","agent_id":"ws-1:t_inflight","workspace_id":"ws-1","assistant":"codex","next_action":"","suggested_command":"","quick_actions":[],"channel":{"message":"timed out","chunks":["timed out"],"chunks_meta":[{"index":1,"total":1,"text":"timed out"}]}}'
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "TURN_ARGS_LOG", turnArgsLog)
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY", "false")

	first := runScriptJSON(t, scriptPath, env,
		"review",
		"--workspace", "ws-1",
		"--assistant", "codex",
		"--allow-new-run",
	)
	if got, _ := first["overall_status"].(string); got != "in_progress" {
		t.Fatalf("first overall_status = %q, want %q", got, "in_progress")
	}

	payload := runScriptJSON(t, scriptPath, env,
		"review",
		"--workspace", "ws-1",
		"--assistant", "codex",
		"--prompt", "review only uncommitted files with severity ranking",
		"--allow-new-run",
	)
	if got, _ := payload["overall_status"].(string); got != "in_progress" {
		t.Fatalf("overall_status = %q, want %q", got, "in_progress")
	}
	if got, _ := payload["status"].(string); got != "attention" {
		t.Fatalf("status = %q, want %q", got, "attention")
	}
	summary, _ := payload["summary"].(string)
	if !strings.Contains(strings.ToLower(summary), "still running") {
		t.Fatalf("summary = %q, want in-progress wording", summary)
	}

	if turnArgsRaw, err := os.ReadFile(turnArgsLog); err == nil {
		lines := strings.Split(strings.TrimSpace(string(turnArgsRaw)), "\n")
		if len(lines) != 1 {
			t.Fatalf("turn script should run once, then reuse cached inflight agent even when prompt wording changes. got lines: %q", string(turnArgsRaw))
		}
		if !strings.Contains(lines[0], "run --workspace ws-1 --assistant codex") {
			t.Fatalf("turn args line = %q, expected initial review run call", lines[0])
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("read turn args: %v", err)
	}
}

func TestAssistantDXReview_ReusesInflightAgentAfterLegacyTTLWindow(t *testing.T) {
	requireBinary(t, "jq")
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dx.sh")
	fakeBinDir := t.TempDir()
	fakeAmuxPath := filepath.Join(fakeBinDir, "amux")
	fakeTurnPath := filepath.Join(fakeBinDir, "fake-turn.sh")
	turnArgsLog := filepath.Join(fakeBinDir, "turn-args.log")
	contextPath := filepath.Join(fakeBinDir, "assistant-dx-context.json")

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
printf '%s' '{"ok":true,"mode":"run","status":"timed_out","overall_status":"timed_out","summary":"turn script should not run","agent_id":"ws-1:t_inflight","workspace_id":"ws-1","assistant":"codex","next_action":"","suggested_command":"","quick_actions":[],"channel":{"message":"timed out","chunks":["timed out"],"chunks_meta":[{"index":1,"total":1,"text":"timed out"}]}}'
`)

	legacyExpiredButStillRecent := time.Now().Unix() - 240
	contextJSON := fmt.Sprintf(`{"review":{"inflight":{"workspace_id":"ws-1","assistant":"codex","prompt_hash":"stale-hash","agent_id":"ws-1:t_inflight","updated_epoch":%d}}}`, legacyExpiredButStillRecent)
	if err := os.WriteFile(contextPath, []byte(contextJSON), 0o644); err != nil {
		t.Fatalf("write context file: %v", err)
	}

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "TURN_ARGS_LOG", turnArgsLog)
	env = withEnv(env, "AMUX_ASSISTANT_DX_CONTEXT_FILE", contextPath)
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY", "false")

	payload := runScriptJSON(t, scriptPath, env,
		"review",
		"--workspace", "ws-1",
		"--assistant", "codex",
	)

	if got, _ := payload["overall_status"].(string); got != "in_progress" {
		t.Fatalf("overall_status = %q, want %q", got, "in_progress")
	}
	if got, _ := payload["status"].(string); got != "attention" {
		t.Fatalf("status = %q, want %q", got, "attention")
	}
	summary, _ := payload["summary"].(string)
	summaryLower := strings.ToLower(summary)
	if !strings.Contains(summaryLower, "still running") && !strings.Contains(summaryLower, "already running") {
		t.Fatalf("summary = %q, want reuse/in-progress wording", summary)
	}

	if turnArgsRaw, err := os.ReadFile(turnArgsLog); err == nil {
		if strings.TrimSpace(string(turnArgsRaw)) != "" {
			t.Fatalf("turn script should not run when inflight review is reused: %q", string(turnArgsRaw))
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("read turn args: %v", err)
	}
}

func TestAssistantDXReview_StartLockBusySkipsLaunchingNewRun(t *testing.T) {
	requireBinary(t, "jq")
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dx.sh")
	fakeBinDir := t.TempDir()
	fakeAmuxPath := filepath.Join(fakeBinDir, "amux")
	fakeTurnPath := filepath.Join(fakeBinDir, "fake-turn.sh")
	turnArgsLog := filepath.Join(fakeBinDir, "turn-args.log")
	lockPath := filepath.Join(fakeBinDir, "review-start.lock")

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
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
  *)
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
esac
`)

	writeExecutable(t, fakeTurnPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${TURN_ARGS_LOG:?missing TURN_ARGS_LOG}"
printf '%s' '{"ok":true,"mode":"run","status":"timed_out","overall_status":"timed_out","summary":"Review timed out.","agent_id":"ws-1:t_locktest","workspace_id":"ws-1","assistant":"codex","next_action":"","suggested_command":"","quick_actions":[],"channel":{"message":"timed out","chunks":["timed out"],"chunks_meta":[{"index":1,"total":1,"text":"timed out"}]}}'
`)

	lockBody := fmt.Sprintf("%d %d\n", time.Now().Unix(), os.Getpid())
	if err := os.WriteFile(lockPath, []byte(lockBody), 0o644); err != nil {
		t.Fatalf("write lock file: %v", err)
	}

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "TURN_ARGS_LOG", turnArgsLog)
	env = withEnv(env, "AMUX_ASSISTANT_DX_TIMEOUT_RECOVERY", "false")
	env = withEnv(env, "AMUX_ASSISTANT_DX_REVIEW_START_LOCK_PATH", lockPath)
	env = withEnv(env, "AMUX_ASSISTANT_DX_REVIEW_START_LOCK_TTL_SECONDS", "600")

	payload := runScriptJSON(t, scriptPath, env,
		"review",
		"--workspace", "ws-1",
		"--assistant", "codex",
	)

	if got, _ := payload["status"].(string); got != "attention" {
		t.Fatalf("status = %q, want %q", got, "attention")
	}
	if got, _ := payload["overall_status"].(string); got != "in_progress" {
		t.Fatalf("overall_status = %q, want %q", got, "in_progress")
	}
	summary, _ := payload["summary"].(string)
	if !strings.Contains(strings.ToLower(summary), "startup is already in progress") {
		t.Fatalf("summary = %q, want startup lock in-progress wording", summary)
	}
	suggested, _ := payload["suggested_command"].(string)
	if !strings.Contains(suggested, "assistant-dx.sh status --workspace ws-1") {
		t.Fatalf("suggested_command = %q, want workspace status command", suggested)
	}

	if turnArgsRaw, err := os.ReadFile(turnArgsLog); err == nil {
		if strings.TrimSpace(string(turnArgsRaw)) != "" {
			t.Fatalf("turn script should not run while startup lock is held: %q", string(turnArgsRaw))
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("read turn args: %v", err)
	}
}
