package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssistantDogfoodScript_MissingFlagValueFailsClearly(t *testing.T) {
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dogfood.sh")
	cmd := exec.Command(scriptPath, "--repo")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for missing flag value")
	}
	text := string(out)
	if !strings.Contains(text, "missing value for --repo") {
		t.Fatalf("output = %q, want missing flag guidance", text)
	}
}

func TestAssistantDogfoodScript_ResolvesPrimaryWorkspaceFromWorkspaceCreate(t *testing.T) {
	requireBinary(t, "bash")
	requireBinary(t, "jq")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dogfood.sh")
	fakeBinDir := t.TempDir()
	writeExecutable(t, filepath.Join(fakeBinDir, "amux"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s' '{"ok":true}'
`)
	writeExecutable(t, filepath.Join(fakeBinDir, "assistant"), `#!/usr/bin/env bash
set -euo pipefail
cmd="${1:-}"
shift || true
case "$cmd" in
  health)
    printf '%s' '{"ok":true}'
    ;;
  agent)
    local_mode="false"
    msg=""
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --local)
          local_mode="true"
          shift
          ;;
        --message)
          msg="${2-}"
          shift 2
          ;;
        *)
          shift
          ;;
      esac
    done
    if [[ "$local_mode" == "true" ]]; then
      jq -cn --arg text "local ping ok" '{status:"ok",payloads:[{text:$text}]}'
    else
      jq -cn --arg text "$msg" '{status:"ok",result:{payloads:[{text:$text}]}}'
    fi
    ;;
  *)
    printf '%s' '{"ok":true}'
    ;;
esac
`)

	fakeDX := filepath.Join(fakeBinDir, "assistant-dx.sh")
	writeExecutable(t, fakeDX, `#!/usr/bin/env bash
set -euo pipefail
cmd="${1:-}"
sub="${2:-}"
case "$cmd $sub" in
  "project add")
    jq -cn --arg path "${4:-${3:-}}" '{ok:true,command:"project.add",status:"ok",summary:"Project registered.",next_action:"Create workspace.",suggested_command:"",data:{name:"repo",path:$path},quick_actions:[]}'
    ;;
  "workspace create")
    name="${3:-}"
    ws_id="ws-primary"
    if [[ "$name" == *"parallel"* ]]; then
      ws_id="ws-secondary"
    fi
    jq -cn --arg ws "$ws_id" '{ok:true,command:"workspace.create",status:"ok",summary:"Workspace created.",next_action:"",suggested_command:"",data:{id:$ws,assistant:"codex"},quick_actions:[]}'
    ;;
  *)
    jq -cn '{ok:true,command:"noop",status:"ok",summary:"ok",next_action:"",suggested_command:"",data:{},quick_actions:[]}'
    ;;
esac
`)

	repoPath := t.TempDir()
	reportDir := t.TempDir()
	cmd := exec.Command(
		scriptPath,
		"--repo", repoPath,
		"--workspace", "dogfood-ws",
		"--assistant", "codex",
		"--report-dir", reportDir,
	)
	env := os.Environ()
	env = withEnv(env, "PATH", fakeBinDir+":"+os.Getenv("PATH"))
	env = withEnv(env, "AMUX_ASSISTANT_DOGFOOD_DX_SCRIPT", fakeDX)
	env = withEnv(env, "AMUX_ASSISTANT_DOGFOOD_CHANNEL_EPHEMERAL_AGENT", "false")
	env = withEnv(env, "AMUX_ASSISTANT_DOGFOOD_CHANNEL_REQUIRE_PROOF", "false")
	env = withEnv(env, "AMUX_ASSISTANT_DOGFOOD_REQUIRE_CHANNEL_EXECUTION", "false")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("assistant-dogfood.sh failed: %v\noutput:\n%s", err, string(out))
	}

	summaryPath := filepath.Join(reportDir, "summary.txt")
	summaryRaw, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read summary: %v\noutput:\n%s", err, string(out))
	}
	summary := string(summaryRaw)
	if !strings.Contains(summary, "workspace_primary=ws-primary") {
		t.Fatalf("summary missing resolved primary workspace id:\n%s", summary)
	}
	if strings.Contains(string(out), "failed to resolve ws1 id from project_add") {
		t.Fatalf("script still relied on project_add workspace id:\n%s", string(out))
	}
}
