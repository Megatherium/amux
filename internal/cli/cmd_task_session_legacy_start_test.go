package cli

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/tmux"
)

func TestCmdTaskStart_IgnoresLegacyTabWhenTaggedAssistantSessionExited(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	wsID := createTaskTestWorkspace(t, home)

	origSessionsWithTags := tmuxSessionsWithTags
	origCapture := tmuxCapturePaneTail
	origStateFor := tmuxSessionStateFor
	origTaskRunAgent := taskRunAgent
	t.Cleanup(func() {
		tmuxSessionsWithTags = origSessionsWithTags
		tmuxCapturePaneTail = origCapture
		tmuxSessionStateFor = origStateFor
		taskRunAgent = origTaskRunAgent
	})

	now := time.Now()
	tmuxSessionsWithTags = func(_ map[string]string, _ []string, _ tmux.Options) ([]tmux.SessionTagValues, error) {
		return []tmux.SessionTagValues{
			{
				Name: "amux-" + string(wsID) + "-t_dead",
				Tags: map[string]string{
					"@amux_tab":        "t_dead",
					"@amux_assistant":  "codex",
					"@amux_created_at": strconv.FormatInt(now.Add(-1*time.Minute).Unix(), 10),
				},
			},
			{
				Name: "amux-" + string(wsID) + "-t_legacy",
				Tags: map[string]string{
					"@amux_tab":          "t_legacy",
					"@amux_created_at":   strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-5*time.Second).Unix(), 10),
				},
			},
		}, nil
	}
	tmuxCapturePaneTail = func(session string, _ int, _ tmux.Options) (string, bool) {
		switch session {
		case "amux-" + string(wsID) + "-t_dead":
			return "", false
		case "amux-" + string(wsID) + "-t_legacy":
			return "Running task now", true
		default:
			t.Fatalf("unexpected session capture: %s", session)
			return "", false
		}
	}
	tmuxSessionStateFor = func(session string, _ tmux.Options) (tmux.SessionState, error) {
		switch session {
		case "amux-" + string(wsID) + "-t_dead":
			return tmux.SessionState{Exists: true, HasLivePane: false}, nil
		case "amux-" + string(wsID) + "-t_legacy":
			return tmux.SessionState{Exists: true, HasLivePane: true}, nil
		default:
			t.Fatalf("unexpected session lookup: %s", session)
			return tmux.SessionState{}, nil
		}
	}
	taskRunAgent = func(_ *Services, wsID data.WorkspaceID, assistant, prompt string, waitTimeout, idleThreshold time.Duration, idempotencyKey, version string) (agentRunResult, error) {
		_ = prompt
		_ = waitTimeout
		_ = idleThreshold
		_ = idempotencyKey
		_ = version
		return agentRunResult{
			SessionName: "sess-next",
			AgentID:     string(wsID) + ":t_next",
			WorkspaceID: string(wsID),
			Assistant:   assistant,
			TabID:       "t_next",
			Response: &waitResponseResult{
				Status:     "idle",
				Summary:    "Started fresh run after tagged session exited.",
				LatestLine: "Started fresh run after tagged session exited.",
			},
		}, nil
	}

	var out, errOut bytes.Buffer
	code := cmdTaskStart(&out, &errOut, GlobalFlags{JSON: true}, []string{
		"--workspace", string(wsID),
		"--assistant", "codex",
		"--prompt", "Refactor parser and run tests",
	}, "test-v1")
	if code != ExitOK {
		t.Fatalf("cmdTaskStart() code = %d, stderr=%q out=%q", code, errOut.String(), out.String())
	}

	payload := decodeTaskResult(t, out.Bytes())
	if got := taskString(payload, "status"); got == "needs_input" {
		t.Fatalf("status = %q, want tagged exited session to allow fresh run", got)
	}
	if got := taskString(payload, "session_name"); got != "sess-next" {
		t.Fatalf("session_name = %q, want new task session", got)
	}
}
