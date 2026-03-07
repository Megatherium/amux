package cli

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/andyrewlee/amux/internal/tmux"
)

func TestCmdTaskStatus_IgnoresLegacySessionWhenTaggedAssistantSessionExited(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	wsID := createTaskTestWorkspace(t, home)

	origSessionsWithTags := tmuxSessionsWithTags
	origCapture := tmuxCapturePaneTail
	origStateFor := tmuxSessionStateFor
	t.Cleanup(func() {
		tmuxSessionsWithTags = origSessionsWithTags
		tmuxCapturePaneTail = origCapture
		tmuxSessionStateFor = origStateFor
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

	var out, errOut bytes.Buffer
	code := cmdTaskStatus(&out, &errOut, GlobalFlags{JSON: true}, []string{
		"--workspace", string(wsID),
		"--assistant", "codex",
	}, "test-v1")
	if code != ExitOK {
		t.Fatalf("cmdTaskStatus() code = %d, stderr=%q out=%q", code, errOut.String(), out.String())
	}

	payload := decodeTaskResult(t, out.Bytes())
	if got := taskString(payload, "status"); got != "attention" {
		t.Fatalf("status = %q, want %q", got, "attention")
	}
	if got := taskString(payload, "overall_status"); got != "session_exited" {
		t.Fatalf("overall_status = %q, want %q", got, "session_exited")
	}
	if got := taskString(payload, "session_name"); got != "amux-"+string(wsID)+"-t_dead" {
		t.Fatalf("session_name = %q, want tagged exited session", got)
	}
}
