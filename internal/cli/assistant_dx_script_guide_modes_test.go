package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssistantDXGuide_ClaudeModeShowsDetectedModeAndPermissionsAction(t *testing.T) {
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
    printf '%s' '{"ok":true,"data":[{"id":"ws-1","name":"mobile","repo":"/tmp/demo","root":"/tmp/ws-1","assistant":"claude"}],"error":null}'
    ;;
  "agent list")
    printf '%s' '{"ok":true,"data":[{"agent_id":"agent-1","session_name":"sess-1","workspace_id":"ws-1"}],"error":null}'
    ;;
  "terminal list")
    printf '%s' '{"ok":true,"data":[{"workspace_id":"ws-1","session_name":"term-1"}],"error":null}'
    ;;
  "agent capture")
    printf '%s' '{"ok":true,"data":{"status":"captured","summary":"⏵⏵ accept edits on (shift+tab to cycle)","content":"⏵⏵ accept edits on (shift+tab to cycle)","needs_input":false,"input_hint":""},"error":null}'
    ;;
  *)
    printf '{"ok":false,"error":{"code":"unexpected","message":"unexpected args: %s"}}' "$*"
    ;;
esac
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))

	payload := runScriptJSON(t, scriptPath, env,
		"guide",
		"--workspace", "ws-1",
	)

	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data missing or wrong type: %T", payload["data"])
	}
	captureMode, ok := data["capture_mode"].(map[string]any)
	if !ok {
		t.Fatalf("capture_mode missing or wrong type: %T", data["capture_mode"])
	}
	if got, _ := captureMode["assistant"].(string); got != "claude" {
		t.Fatalf("capture_mode.assistant = %q, want claude", got)
	}
	if got, _ := captureMode["label"].(string); got != "accept-edits" {
		t.Fatalf("capture_mode.label = %q, want accept-edits", got)
	}

	channel, ok := payload["channel"].(map[string]any)
	if !ok {
		t.Fatalf("channel missing or wrong type: %T", payload["channel"])
	}
	message, _ := channel["message"].(string)
	if !strings.Contains(message, "Detected mode: accept-edits") {
		t.Fatalf("channel.message = %q, want detected mode line", message)
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

func TestAssistantDXGuide_DroidModeAddsModeSwitchActions(t *testing.T) {
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
    printf '%s' '{"ok":true,"data":[{"id":"ws-1","name":"mobile","repo":"/tmp/demo","root":"/tmp/ws-1","assistant":"droid"}],"error":null}'
    ;;
  "agent list")
    printf '%s' '{"ok":true,"data":[{"agent_id":"agent-1","session_name":"sess-1","workspace_id":"ws-1"}],"error":null}'
    ;;
  "terminal list")
    printf '%s' '{"ok":true,"data":[{"workspace_id":"ws-1","session_name":"term-1"}],"error":null}'
    ;;
  "agent capture")
    printf '%s' '{"ok":true,"data":{"status":"captured","summary":"Auto (Medium) - allow safe commands","content":"Auto (Medium) - allow safe commands\\nshift+tab to cycle modes (auto/spec), ctrl+L for autonomy","needs_input":false,"input_hint":""},"error":null}'
    ;;
  *)
    printf '{"ok":false,"error":{"code":"unexpected","message":"unexpected args: %s"}}' "$*"
    ;;
esac
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))

	payload := runScriptJSON(t, scriptPath, env,
		"guide",
		"--workspace", "ws-1",
	)

	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data missing or wrong type: %T", payload["data"])
	}
	captureMode, ok := data["capture_mode"].(map[string]any)
	if !ok {
		t.Fatalf("capture_mode missing or wrong type: %T", data["capture_mode"])
	}
	if got, _ := captureMode["assistant"].(string); got != "droid" {
		t.Fatalf("capture_mode.assistant = %q, want droid", got)
	}
	if got, _ := captureMode["label"].(string); got != "auto-medium" {
		t.Fatalf("capture_mode.label = %q, want auto-medium", got)
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
