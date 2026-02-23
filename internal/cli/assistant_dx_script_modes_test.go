package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssistantDXStatus_DetectsClaudeModeAndAddsPermissionsAction(t *testing.T) {
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
    printf '%s' '{"ok":true,"data":[{"id":"ws-main","name":"mainline","repo":"/tmp/demo","scope":"project","assistant":"claude","created":"2026-01-01T00:00:00Z"}],"error":null}'
    ;;
  "agent list")
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-main","agent_id":"agent-main","workspace_id":"ws-main","tab_id":"tab-2","type":"agent"}],"error":null}'
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
    printf '%s' '{"ok":true,"data":{"session_name":"sess-main","status":"captured","summary":"⏸ plan mode on (shift+tab to cycle) · PR #180","content":"⏸ plan mode on (shift+tab to cycle) · PR #180","needs_input":false,"input_hint":""},"error":null}'
    ;;
  *)
    printf '{"ok":false,"error":{"code":"unexpected","message":"unexpected args: %s"}}' "$*"
    ;;
esac
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))

	payload := runScriptJSON(t, scriptPath, env, "status", "--workspace", "ws-main")

	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data missing or wrong type: %T", payload["data"])
	}
	modeSignals, ok := data["mode_signals"].([]any)
	if !ok || len(modeSignals) == 0 {
		t.Fatalf("mode_signals missing or empty: %#v", data["mode_signals"])
	}
	firstMode, ok := modeSignals[0].(map[string]any)
	if !ok {
		t.Fatalf("mode_signals[0] wrong type: %T", modeSignals[0])
	}
	if got, _ := firstMode["assistant"].(string); got != "claude" {
		t.Fatalf("mode assistant = %q, want claude", got)
	}
	if got, _ := firstMode["mode_label"].(string); got != "plan" {
		t.Fatalf("mode label = %q, want plan", got)
	}

	quickActions, ok := payload["quick_actions"].([]any)
	if !ok || len(quickActions) == 0 {
		t.Fatalf("quick_actions missing or empty: %#v", payload["quick_actions"])
	}
	var sawPermissions bool
	for _, raw := range quickActions {
		action, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, _ := action["id"].(string)
		command, _ := action["command"].(string)
		if id == "claude_permissions" && strings.Contains(command, "--text \"/permissions\" --enter") {
			sawPermissions = true
			break
		}
	}
	if !sawPermissions {
		t.Fatalf("expected claude_permissions quick action in %#v", quickActions)
	}
}

func TestAssistantDXStatus_DetectsDroidModeAndAddsModeSwitchActions(t *testing.T) {
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
    printf '%s' '{"ok":true,"data":[{"id":"ws-main","name":"mainline","repo":"/tmp/demo","scope":"project","assistant":"droid","created":"2026-01-01T00:00:00Z"}],"error":null}'
    ;;
  "agent list")
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-main","agent_id":"agent-main","workspace_id":"ws-main","tab_id":"tab-2","type":"agent"}],"error":null}'
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
    printf '%s' '{"ok":true,"data":{"session_name":"sess-main","status":"captured","summary":"Auto (High) - allow all commands","content":"Auto (High) - allow all commands\\nshift+tab to cycle modes (auto/spec), ctrl+L for autonomy","needs_input":false,"input_hint":""},"error":null}'
    ;;
  *)
    printf '{"ok":false,"error":{"code":"unexpected","message":"unexpected args: %s"}}' "$*"
    ;;
esac
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))

	payload := runScriptJSON(t, scriptPath, env, "status", "--workspace", "ws-main")

	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data missing or wrong type: %T", payload["data"])
	}
	modeSignals, ok := data["mode_signals"].([]any)
	if !ok || len(modeSignals) == 0 {
		t.Fatalf("mode_signals missing or empty: %#v", data["mode_signals"])
	}
	firstMode, ok := modeSignals[0].(map[string]any)
	if !ok {
		t.Fatalf("mode_signals[0] wrong type: %T", modeSignals[0])
	}
	if got, _ := firstMode["assistant"].(string); got != "droid" {
		t.Fatalf("mode assistant = %q, want droid", got)
	}
	if got, _ := firstMode["mode_label"].(string); got != "auto-high" {
		t.Fatalf("mode label = %q, want auto-high", got)
	}

	quickActions, ok := payload["quick_actions"].([]any)
	if !ok || len(quickActions) == 0 {
		t.Fatalf("quick_actions missing or empty: %#v", payload["quick_actions"])
	}
	wantIDs := map[string]bool{
		"droid_mode":        false,
		"droid_normal":      false,
		"droid_spec":        false,
		"droid_auto_low":    false,
		"droid_auto_medium": false,
		"droid_auto_high":   false,
	}
	for _, raw := range quickActions {
		action, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, _ := action["id"].(string)
		if _, exists := wantIDs[id]; exists {
			wantIDs[id] = true
		}
	}
	for id, seen := range wantIDs {
		if !seen {
			t.Fatalf("missing %s quick action in %#v", id, quickActions)
		}
	}
}

func TestAssistantDXStatus_DoesNotInferClaudeOrDroidModeForCodexWorkspace(t *testing.T) {
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
    printf '%s' '{"ok":true,"data":[{"id":"ws-main","name":"mainline","repo":"/tmp/demo","scope":"project","assistant":"codex","created":"2026-01-01T00:00:00Z"}],"error":null}'
    ;;
  "agent list")
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-main","agent_id":"agent-main","workspace_id":"ws-main","tab_id":"tab-2","type":"agent"}],"error":null}'
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
    printf '%s' '{"ok":true,"data":{"session_name":"sess-main","status":"captured","summary":"Use /permissions if blocked; shift+tab to cycle","content":"Use /permissions if blocked; shift+tab to cycle","needs_input":false,"input_hint":""},"error":null}'
    ;;
  *)
    printf '{"ok":false,"error":{"code":"unexpected","message":"unexpected args: %s"}}' "$*"
    ;;
esac
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))

	payload := runScriptJSON(t, scriptPath, env, "status", "--workspace", "ws-main")

	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data missing or wrong type: %T", payload["data"])
	}
	modeSignals, ok := data["mode_signals"].([]any)
	if !ok {
		t.Fatalf("mode_signals missing: %#v", data["mode_signals"])
	}
	if len(modeSignals) != 0 {
		t.Fatalf("expected no mode_signals for codex workspace, got %#v", modeSignals)
	}

	quickActions, ok := payload["quick_actions"].([]any)
	if !ok {
		t.Fatalf("quick_actions missing or wrong type: %#v", payload["quick_actions"])
	}
	for _, raw := range quickActions {
		action, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, _ := action["id"].(string)
		if id == "claude_permissions" || strings.HasPrefix(id, "droid_") {
			t.Fatalf("unexpected mode switch action %q in %#v", id, quickActions)
		}
	}
}

func TestAssistantDXStatus_NonModeNeedsInputStillAddsActiveModeActions(t *testing.T) {
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
    printf '%s' '{"ok":true,"data":[{"id":"ws-codex","name":"codex-main","repo":"/tmp/demo","scope":"project","assistant":"codex","created":"2026-01-01T00:00:00Z"},{"id":"ws-claude","name":"claude-main","repo":"/tmp/demo","scope":"project","assistant":"claude","created":"2026-01-01T00:00:01Z"},{"id":"ws-droid","name":"droid-main","repo":"/tmp/demo","scope":"project","assistant":"droid","created":"2026-01-01T00:00:02Z"}],"error":null}'
    ;;
  "agent list")
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-codex","agent_id":"agent-codex","workspace_id":"ws-codex","tab_id":"tab-c","type":"agent"},{"session_name":"sess-claude","agent_id":"agent-claude","workspace_id":"ws-claude","tab_id":"tab-cl","type":"agent"},{"session_name":"sess-droid","agent_id":"agent-droid","workspace_id":"ws-droid","tab_id":"tab-dr","type":"agent"}],"error":null}'
    ;;
  "terminal list")
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
  "session list")
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-codex"},{"session_name":"sess-claude"},{"session_name":"sess-droid"}],"error":null}'
    ;;
  "session prune")
    printf '%s' '{"ok":true,"data":{"dry_run":true,"pruned":[],"total":0,"errors":[]},"error":null}'
    ;;
  "agent capture")
    if [[ "${3:-}" == "sess-codex" ]]; then
      printf '%s' '{"ok":true,"data":{"session_name":"sess-codex","status":"captured","summary":"Pick one:\n1. Continue\n2. Stop","content":"Pick one:\n1. Continue\n2. Stop","needs_input":true,"input_hint":"Pick one:\n1. Continue\n2. Stop"},"error":null}'
    elif [[ "${3:-}" == "sess-claude" ]]; then
      printf '%s' '{"ok":true,"data":{"session_name":"sess-claude","status":"captured","summary":"⏵⏵ accept edits on (shift+tab to cycle)","content":"⏵⏵ accept edits on (shift+tab to cycle)","needs_input":false,"input_hint":""},"error":null}'
    else
      printf '%s' '{"ok":true,"data":{"session_name":"sess-droid","status":"captured","summary":"Auto (Medium) - allow safe commands","content":"Auto (Medium) - allow safe commands\nshift+tab to cycle modes (auto/spec), ctrl+L for autonomy","needs_input":false,"input_hint":""},"error":null}'
    fi
    ;;
  *)
    printf '{"ok":false,"error":{"code":"unexpected","message":"unexpected args: %s"}}' "$*"
    ;;
esac
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))

	payload := runScriptJSON(t, scriptPath, env, "status")

	if got, _ := payload["status"].(string); got != "needs_input" {
		t.Fatalf("status = %q, want %q", got, "needs_input")
	}
	quickActions, ok := payload["quick_actions"].([]any)
	if !ok || len(quickActions) == 0 {
		t.Fatalf("quick_actions missing or empty: %#v", payload["quick_actions"])
	}
	var sawReply1 bool
	var sawClaudePermissions bool
	var sawDroidMode bool
	for _, raw := range quickActions {
		action, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, _ := action["id"].(string)
		command, _ := action["command"].(string)
		if id == "reply_1" {
			sawReply1 = true
		}
		if id == "claude_permissions" && strings.Contains(command, "continue --agent agent-claude --text \"/permissions\" --enter") {
			sawClaudePermissions = true
		}
		if id == "droid_mode" && strings.Contains(command, "continue --agent agent-droid --text \"/mode\" --enter") {
			sawDroidMode = true
		}
	}
	if !sawReply1 {
		t.Fatalf("expected reply_1 quick action in %#v", quickActions)
	}
	if !sawClaudePermissions {
		t.Fatalf("expected claude_permissions quick action in %#v", quickActions)
	}
	if !sawDroidMode {
		t.Fatalf("expected droid_mode quick action in %#v", quickActions)
	}
}

func TestAssistantDXStatus_NeedsInputExtendedChoiceActions(t *testing.T) {
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
    printf '%s' '{"ok":true,"data":[{"id":"ws-main","name":"mainline","repo":"/tmp/demo","scope":"project","assistant":"codex","created":"2026-01-01T00:00:00Z"}],"error":null}'
    ;;
  "agent list")
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-main","agent_id":"agent-main","workspace_id":"ws-main","tab_id":"tab-main","type":"agent"}],"error":null}'
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
    printf '%s' '{"ok":true,"data":{"session_name":"sess-main","status":"captured","summary":"Pick one:\n1. One\n2. Two\n3. Three\n4. Four\n5. Five\nA. A\nB. B\nC. C\nD. D\nE. E","content":"Pick one:\n1. One\n2. Two\n3. Three\n4. Four\n5. Five\nA. A\nB. B\nC. C\nD. D\nE. E","needs_input":true,"input_hint":"Pick one:\n1. One\n2. Two\n3. Three\n4. Four\n5. Five\nA. A\nB. B\nC. C\nD. D\nE. E"},"error":null}'
    ;;
  *)
    printf '{"ok":false,"error":{"code":"unexpected","message":"unexpected args: %s"}}' "$*"
    ;;
esac
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))

	payload := runScriptJSON(t, scriptPath, env, "status")

	quickActions, ok := payload["quick_actions"].([]any)
	if !ok || len(quickActions) == 0 {
		t.Fatalf("quick_actions missing or empty: %#v", payload["quick_actions"])
	}
	wantIDs := map[string]bool{
		"reply_4": false,
		"reply_5": false,
		"reply_d": false,
		"reply_e": false,
	}
	for _, raw := range quickActions {
		action, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, _ := action["id"].(string)
		if _, exists := wantIDs[id]; exists {
			wantIDs[id] = true
		}
	}
	for id, seen := range wantIDs {
		if !seen {
			t.Fatalf("missing %s quick action in %#v", id, quickActions)
		}
	}
}
