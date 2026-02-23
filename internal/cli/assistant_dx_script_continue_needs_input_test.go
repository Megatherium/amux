package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssistantDXContinue_DoesNotAutoFollowWhenPromptHasExplicitChoices(t *testing.T) {
	requireBinary(t, "jq")
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dx.sh")
	fakeBinDir := t.TempDir()
	fakeAmuxPath := filepath.Join(fakeBinDir, "amux")
	fakeTurnPath := filepath.Join(fakeBinDir, "fake-turn.sh")
	countFile := filepath.Join(fakeBinDir, "turn-count.txt")

	writeExecutable(t, fakeAmuxPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s' '{"ok":true,"data":{},"error":null}'
`)

	writeExecutable(t, fakeTurnPath, `#!/usr/bin/env bash
set -euo pipefail
count=0
if [[ -f "${TURN_COUNT_FILE:?missing TURN_COUNT_FILE}" ]]; then
  count="$(cat "${TURN_COUNT_FILE}")"
fi
count=$((count + 1))
printf '%s' "$count" > "${TURN_COUNT_FILE}"
if [[ "$count" -eq 1 ]]; then
  printf '%s' '{"ok":true,"mode":"send","status":"needs_input","overall_status":"needs_input","summary":"Pick one:\n1. Continue with codex\n2. Continue with claude\nPress enter to continue","input_hint":"Pick one:\n1. Continue with codex\n2. Continue with claude\nPress enter to continue","agent_id":"agent-1","workspace_id":"ws-1","next_action":"","suggested_command":"","quick_actions":[],"channel":{"message":"Pick one:\n1. Continue with codex\n2. Continue with claude\nPress enter to continue","chunks":["Pick one:\n1. Continue with codex\n2. Continue with claude\nPress enter to continue"],"chunks_meta":[{"index":1,"total":1,"text":"Pick one:\n1. Continue with codex\n2. Continue with claude\nPress enter to continue"}]}}'
else
  printf '%s' '{"ok":true,"mode":"send","status":"idle","overall_status":"completed","summary":"should not auto-continue","agent_id":"agent-1","workspace_id":"ws-1","quick_actions":[],"channel":{"message":"done","chunks":["done"],"chunks_meta":[{"index":1,"total":1,"text":"done"}]}}'
fi
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "TURN_COUNT_FILE", countFile)

	payload := runScriptJSON(t, scriptPath, env,
		"continue",
		"--agent", "agent-1",
		"--text", "continue",
		"--enter",
	)

	if got, _ := payload["status"].(string); got != "needs_input" {
		t.Fatalf("status = %q, want %q when explicit choices require user decision", got, "needs_input")
	}
	countRaw, err := os.ReadFile(countFile)
	if err != nil {
		t.Fatalf("read turn count: %v", err)
	}
	if strings.TrimSpace(string(countRaw)) != "1" {
		t.Fatalf("turn invocation count = %q, want 1 (no auto-follow)", strings.TrimSpace(string(countRaw)))
	}

	quickActions, ok := payload["quick_actions"].([]any)
	if !ok || len(quickActions) == 0 {
		t.Fatalf("quick_actions missing or empty: %#v", payload["quick_actions"])
	}
	var sawReply1 bool
	var sawReply2 bool
	var sawReplyEnter bool
	for _, raw := range quickActions {
		action, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, _ := action["id"].(string)
		cmd, _ := action["command"].(string)
		if id == "reply_1" && strings.Contains(cmd, "--text \"1\"") {
			sawReply1 = true
		}
		if id == "reply_2" && strings.Contains(cmd, "--text \"2\"") {
			sawReply2 = true
		}
		if id == "reply_enter" && strings.Contains(cmd, "continue --agent agent-1 --enter") {
			sawReplyEnter = true
		}
	}
	if !sawReply1 || !sawReply2 || !sawReplyEnter {
		t.Fatalf("expected reply_1/reply_2/reply_enter quick actions in %#v", quickActions)
	}
}

func TestAssistantDXContinue_DoesNotAutoFollowWhenPromptHasHigherNumericChoices(t *testing.T) {
	requireBinary(t, "jq")
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dx.sh")
	fakeBinDir := t.TempDir()
	fakeAmuxPath := filepath.Join(fakeBinDir, "amux")
	fakeTurnPath := filepath.Join(fakeBinDir, "fake-turn.sh")
	countFile := filepath.Join(fakeBinDir, "turn-count.txt")

	writeExecutable(t, fakeAmuxPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s' '{"ok":true,"data":{},"error":null}'
`)

	writeExecutable(t, fakeTurnPath, `#!/usr/bin/env bash
set -euo pipefail
count=0
if [[ -f "${TURN_COUNT_FILE:?missing TURN_COUNT_FILE}" ]]; then
  count="$(cat "${TURN_COUNT_FILE}")"
fi
count=$((count + 1))
printf '%s' "$count" > "${TURN_COUNT_FILE}"
if [[ "$count" -eq 1 ]]; then
  printf '%s' '{"ok":true,"mode":"send","status":"needs_input","overall_status":"needs_input","summary":"Pick one:\n4. Continue with codex\n5. Continue with claude","input_hint":"Pick one:\n4. Continue with codex\n5. Continue with claude","agent_id":"agent-1","workspace_id":"ws-1","next_action":"","suggested_command":"","quick_actions":[],"channel":{"message":"Pick one:\n4. Continue with codex\n5. Continue with claude","chunks":["Pick one:\n4. Continue with codex\n5. Continue with claude"],"chunks_meta":[{"index":1,"total":1,"text":"Pick one:\n4. Continue with codex\n5. Continue with claude"}]}}'
else
  printf '%s' '{"ok":true,"mode":"send","status":"idle","overall_status":"completed","summary":"should not auto-continue","agent_id":"agent-1","workspace_id":"ws-1","quick_actions":[],"channel":{"message":"done","chunks":["done"],"chunks_meta":[{"index":1,"total":1,"text":"done"}]}}'
fi
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "TURN_COUNT_FILE", countFile)

	payload := runScriptJSON(t, scriptPath, env,
		"continue",
		"--agent", "agent-1",
		"--text", "continue",
		"--enter",
	)

	if got, _ := payload["status"].(string); got != "needs_input" {
		t.Fatalf("status = %q, want %q when higher numeric choices require user decision", got, "needs_input")
	}
	countRaw, err := os.ReadFile(countFile)
	if err != nil {
		t.Fatalf("read turn count: %v", err)
	}
	if strings.TrimSpace(string(countRaw)) != "1" {
		t.Fatalf("turn invocation count = %q, want 1 (no auto-follow)", strings.TrimSpace(string(countRaw)))
	}
	quickActions, ok := payload["quick_actions"].([]any)
	if !ok || len(quickActions) == 0 {
		t.Fatalf("quick_actions missing or empty: %#v", payload["quick_actions"])
	}
	var sawReply4 bool
	var sawReply5 bool
	for _, raw := range quickActions {
		action, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, _ := action["id"].(string)
		cmd, _ := action["command"].(string)
		if id == "reply_4" && strings.Contains(cmd, "--text \"4\"") {
			sawReply4 = true
		}
		if id == "reply_5" && strings.Contains(cmd, "--text \"5\"") {
			sawReply5 = true
		}
	}
	if !sawReply4 || !sawReply5 {
		t.Fatalf("expected reply_4/reply_5 quick actions in %#v", quickActions)
	}
}

func TestAssistantDXContinue_EnterOnlyDoesNotInjectDefaultText(t *testing.T) {
	requireBinary(t, "jq")
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dx.sh")
	fakeBinDir := t.TempDir()
	fakeAmuxPath := filepath.Join(fakeBinDir, "amux")
	fakeTurnPath := filepath.Join(fakeBinDir, "fake-turn.sh")
	argsLog := filepath.Join(fakeBinDir, "turn-args.log")

	writeExecutable(t, fakeAmuxPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s' '{"ok":true,"data":{},"error":null}'
`)

	writeExecutable(t, fakeTurnPath, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" > "${TURN_ARGS_LOG:?missing TURN_ARGS_LOG}"
printf '%s' '{"ok":true,"mode":"send","status":"idle","overall_status":"completed","summary":"continued","agent_id":"agent-1","workspace_id":"ws-1","channel":{"message":"done","chunks":["done"],"chunks_meta":[{"index":1,"total":1,"text":"done"}]},"quick_actions":[]}'
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_TURN_SCRIPT", fakeTurnPath)
	env = withEnv(env, "TURN_ARGS_LOG", argsLog)

	_ = runScriptJSON(t, scriptPath, env,
		"continue",
		"--agent", "agent-1",
		"--enter",
	)

	argsRaw, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatalf("read turn args: %v", err)
	}
	args := strings.TrimSpace(string(argsRaw))
	if !strings.Contains(args, "send --agent agent-1") {
		t.Fatalf("turn args = %q, expected send with target agent", args)
	}
	if !strings.Contains(args, "--enter") {
		t.Fatalf("turn args = %q, expected --enter", args)
	}
	if strings.Contains(args, "--text Continue from current state and provide concise status and next action.") {
		t.Fatalf("turn args should not inject default continue text for enter-only replies: %q", args)
	}
}
