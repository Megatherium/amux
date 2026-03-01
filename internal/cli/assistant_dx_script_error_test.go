package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssistantDXProjectList_EmitsJSONOnAmuxError(t *testing.T) {
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
    printf '%s' '{"ok":false,"error":{"code":"boom","message":"fail"}}'
    ;;
  *)
    printf '%s' '{"ok":false,"error":{"code":"unexpected","message":"unexpected args"}}'
    ;;
esac
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))

	payload := runScriptJSON(t, scriptPath, env, "project", "list")
	if got, _ := payload["ok"].(bool); got {
		t.Fatalf("ok = true, want false")
	}
	if got, _ := payload["status"].(string); got != "command_error" {
		t.Fatalf("status = %q, want command_error", got)
	}
	summary, _ := payload["summary"].(string)
	if !strings.Contains(summary, "fail") {
		t.Fatalf("summary = %q, want error message", summary)
	}
}

func TestAssistantDXProjectList_UsesJSONErrorEnvelopeWhenAmuxExitsNonZero(t *testing.T) {
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
    printf '%s' '{"ok":false,"error":{"code":"boom","message":"fail"}}'
    exit 3
    ;;
  *)
    printf '%s' '{"ok":false,"error":{"code":"unexpected","message":"unexpected args"}}'
    exit 2
    ;;
esac
`)

	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))

	payload := runScriptJSON(t, scriptPath, env, "project", "list")
	if got, _ := payload["ok"].(bool); got {
		t.Fatalf("ok = true, want false")
	}
	if got, _ := payload["status"].(string); got != "command_error" {
		t.Fatalf("status = %q, want command_error", got)
	}
	summary, _ := payload["summary"].(string)
	if !strings.Contains(summary, "fail") {
		t.Fatalf("summary = %q, want amux JSON error message", summary)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data missing or wrong type: %T", payload["data"])
	}
	if got, _ := data["details"].(string); got != "boom" {
		t.Fatalf("data.details = %q, want boom", got)
	}
}
