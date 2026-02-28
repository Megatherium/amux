package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssistantDXGuide_NoAgentIntentRouting(t *testing.T) {
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
    printf '%s' '{"ok":true,"data":[{"id":"ws-1","name":"mobile","repo":"/tmp/demo","root":"/tmp/ws-1","assistant":"codex"}],"error":null}'
    ;;
  "agent list")
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
  "terminal list")
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
  *)
    printf '{"ok":false,"error":{"code":"unexpected","message":"unexpected args: %s"}}' "$*"
    ;;
esac
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))

	tests := []struct {
		name      string
		task      string
		summary   string
		suggested string
	}{
		{
			name:      "status task routes to status command",
			task:      "run status for this workspace",
			summary:   "Guide: check workspace status",
			suggested: "assistant-dx.sh status --workspace ws-1",
		},
		{
			name:      "active workspaces task routes to status command",
			task:      "what are my active workspaces right now",
			summary:   "Guide: check workspace status",
			suggested: "assistant-dx.sh status --workspace ws-1",
		},
		{
			name:      "review task routes to review command",
			task:      "review uncommitted changes",
			summary:   "Guide: run code review",
			suggested: "assistant-dx.sh review --workspace ws-1 --assistant codex",
		},
		{
			name:      "ship task routes to git ship push",
			task:      "commit, rebase latest, resolve conflicts, push",
			summary:   "Guide: ship current changes",
			suggested: "assistant-dx.sh git ship --workspace ws-1 --push",
		},
		{
			name:      "nested workspace task routes to workspace create nested",
			task:      "create a nested workspace for risky refactor",
			summary:   "Guide: create nested workspace",
			suggested: "assistant-dx.sh workspace create --name refactor --from-workspace ws-1 --scope nested --assistant codex",
		},
		{
			name:      "recover task routes to alerts include stale",
			task:      "help me recover from a hung assistant session",
			summary:   "Guide: recover hung assistant",
			suggested: "assistant-dx.sh alerts --workspace ws-1 --include-stale",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := runScriptJSON(t, scriptPath, env,
				"guide",
				"--workspace", "ws-1",
				"--task", tt.task,
			)
			if got, _ := payload["status"].(string); got == "command_error" {
				t.Fatalf("status = %q, want non-command_error", got)
			}
			summary, _ := payload["summary"].(string)
			if !strings.Contains(summary, tt.summary) {
				t.Fatalf("summary = %q, want contains %q", summary, tt.summary)
			}
			suggested, _ := payload["suggested_command"].(string)
			if !strings.Contains(suggested, tt.suggested) {
				t.Fatalf("suggested_command = %q, want contains %q", suggested, tt.suggested)
			}
		})
	}
}

func TestAssistantDXGuide_ActiveAgentStatusIntentPrefersStatusCommand(t *testing.T) {
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
    printf '%s' '{"ok":true,"data":[{"id":"ws-1","name":"mobile","repo":"/tmp/demo","root":"/tmp/ws-1","assistant":"codex"}],"error":null}'
    ;;
  "agent list")
    printf '%s' '{"ok":true,"data":[{"agent_id":"agent-1","session_name":"sess-1","workspace_id":"ws-1"}],"error":null}'
    ;;
  "terminal list")
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
  "agent capture")
    printf '%s' '{"ok":true,"data":{"status":"captured","summary":"working","needs_input":false,"input_hint":""},"error":null}'
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
		"--task", "show me status and blocked agents",
	)

	summary, _ := payload["summary"].(string)
	if !strings.Contains(summary, "Guide: check workspace status") {
		t.Fatalf("summary = %q, want status guidance", summary)
	}
	suggested, _ := payload["suggested_command"].(string)
	if !strings.Contains(suggested, "assistant-dx.sh status --workspace ws-1") {
		t.Fatalf("suggested_command = %q, want status command", suggested)
	}
}

func TestAssistantDXGuide_TransportOnlyUseAmuxReusesLastIntent(t *testing.T) {
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
    printf '%s' '{"ok":true,"data":[{"id":"ws-1","name":"mobile","repo":"/tmp/demo","root":"/tmp/ws-1","assistant":"codex"}],"error":null}'
    ;;
  "agent list")
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
  "terminal list")
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
  *)
    printf '{"ok":false,"error":{"code":"unexpected","message":"unexpected args: %s"}}' "$*"
    ;;
esac
`)

	contextPath := filepath.Join(t.TempDir(), "assistant-dx-context.json")
	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DX_CONTEXT_FILE", contextPath)

	first := runScriptJSON(t, scriptPath, env,
		"guide",
		"--workspace", "ws-1",
		"--task", "review uncommitted changes",
	)
	firstSuggested, _ := first["suggested_command"].(string)
	if !strings.Contains(firstSuggested, "assistant-dx.sh review --workspace ws-1 --assistant codex") {
		t.Fatalf("first suggested_command = %q, want review command", firstSuggested)
	}

	second := runScriptJSON(t, scriptPath, env,
		"guide",
		"--workspace", "ws-1",
		"--task", "use amux",
	)
	secondSummary, _ := second["summary"].(string)
	if !strings.Contains(secondSummary, "Guide: run code review") {
		t.Fatalf("second summary = %q, want review guidance", secondSummary)
	}
	secondSuggested, _ := second["suggested_command"].(string)
	if !strings.Contains(secondSuggested, "assistant-dx.sh review --workspace ws-1 --assistant codex") {
		t.Fatalf("second suggested_command = %q, want review command", secondSuggested)
	}
	data, _ := second["data"].(map[string]any)
	if got, _ := data["task_source"].(string); got != "context_last_task" {
		t.Fatalf("data.task_source = %q, want context_last_task", got)
	}
}

func TestAssistantDXGuide_InfersWorkspaceFromTaskText(t *testing.T) {
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
    printf '%s' '{"ok":true,"data":[{"name":"amux","path":"/tmp/amux"}],"error":null}'
    ;;
  "workspace list")
    printf '%s' '{"ok":true,"data":[{"id":"ws-main","name":"main","repo":"/tmp/amux","assistant":"codex"},{"id":"ws-projects-nested","name":"projects-nested","repo":"/tmp/amux","assistant":"codex"}],"error":null}'
    ;;
  "agent list")
    printf '%s' '{"ok":true,"data":[],"error":null}'
    ;;
  "terminal list")
    printf '%s' '{"ok":true,"data":[],"error":null}'
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
		"--task", "review uncommitted changes using codex in projects-nested workspace",
	)

	if got, _ := payload["status"].(string); got == "command_error" {
		t.Fatalf("status = %q, want non-command_error", got)
	}
	summary, _ := payload["summary"].(string)
	if !strings.Contains(summary, "Guide: run code review") {
		t.Fatalf("summary = %q, want review guidance", summary)
	}
	suggested, _ := payload["suggested_command"].(string)
	if !strings.Contains(suggested, "assistant-dx.sh review --workspace ws-projects-nested --assistant codex") {
		t.Fatalf("suggested_command = %q, want projects-nested workspace review command", suggested)
	}
}
