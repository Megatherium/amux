package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssistantDXStatus_ActiveWorkspaceCountsUseWorkSignals(t *testing.T) {
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
    printf '%s' '{"ok":true,"data":[{"id":"ws-active","name":"feature-auth","repo":"/tmp/demo","scope":"project","created":"2026-01-03T00:00:00Z"},{"id":"ws-done","name":"cleanup","repo":"/tmp/demo","scope":"nested","parent_workspace":"ws-active","created":"2026-01-02T00:00:00Z"},{"id":"ws-idle","name":"backlog","repo":"/tmp/demo","scope":"project","created":"2026-01-01T00:00:00Z"}],"error":null}'
    ;;
  "agent list")
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-active","agent_id":"agent-active","workspace_id":"ws-active","tab_id":"tab-1","type":"agent"},{"session_name":"sess-done","agent_id":"agent-done","workspace_id":"ws-done","tab_id":"tab-2","type":"agent"}],"error":null}'
    ;;
  "terminal list")
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
  "session list")
    printf '%s' '{"ok":true,"data":[{"session_name":"sess-active"},{"session_name":"sess-done"}],"error":null}'
    ;;
  "session prune")
    printf '%s' '{"ok":true,"data":{"dry_run":true,"pruned":[],"total":0,"errors":[]},"error":null}'
    ;;
  "agent capture")
    if [[ "${3:-}" == "sess-active" ]]; then
      printf '%s' '{"ok":true,"data":{"session_name":"sess-active","status":"captured","summary":"Implementing auth middleware refactor.","needs_input":false,"input_hint":""},"error":null}'
    else
      printf '%s' '{"ok":true,"data":{"session_name":"sess-done","status":"captured","summary":"Implemented cleanup and tests passed.","needs_input":false,"input_hint":""},"error":null}'
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
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data missing or wrong type: %T", payload["data"])
	}
	counts, ok := data["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts missing or wrong type: %T", data["counts"])
	}
	if got, _ := counts["active_workspaces"].(float64); got != 1 {
		t.Fatalf("counts.active_workspaces = %v, want 1", got)
	}
	if got, _ := counts["in_progress_workspaces"].(float64); got != 1 {
		t.Fatalf("counts.in_progress_workspaces = %v, want 1", got)
	}
	if got, _ := counts["completed_workspaces"].(float64); got != 1 {
		t.Fatalf("counts.completed_workspaces = %v, want 1", got)
	}

	activeWorkspaces, ok := data["active_workspaces"].([]any)
	if !ok || len(activeWorkspaces) != 1 {
		t.Fatalf("active_workspaces = %#v, want one active workspace", data["active_workspaces"])
	}
	first, ok := activeWorkspaces[0].(map[string]any)
	if !ok {
		t.Fatalf("active_workspaces[0] wrong type: %T", activeWorkspaces[0])
	}
	if got, _ := first["id"].(string); got != "ws-active" {
		t.Fatalf("active workspace id = %q, want %q", got, "ws-active")
	}
	if got, _ := first["work_state"].(string); got != "in_progress" {
		t.Fatalf("active workspace work_state = %q, want %q", got, "in_progress")
	}

	channel, ok := payload["channel"].(map[string]any)
	if !ok {
		t.Fatalf("channel missing or wrong type: %T", payload["channel"])
	}
	message, _ := channel["message"].(string)
	if !strings.Contains(message, "Active workspaces (work in progress): 1") {
		t.Fatalf("channel.message = %q, want active workspace count", message)
	}
	if !strings.Contains(message, "ws-active (feature-auth)") {
		t.Fatalf("channel.message = %q, want active workspace detail", message)
	}
	if strings.Contains(message, "ws-done (cleanup): in progress") {
		t.Fatalf("channel.message = %q, should not classify completed workspace as active", message)
	}
}
