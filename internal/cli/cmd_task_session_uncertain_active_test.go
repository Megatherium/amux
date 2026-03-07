package cli

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/andyrewlee/amux/internal/tmux"
)

func TestCmdTaskStart_PrefersNewerUncertainSessionOverOlderActiveRun(t *testing.T) {
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
				Name: "amux-" + string(wsID) + "-t_uncertain",
				Tags: map[string]string{
					"@amux_tab":          "t_uncertain",
					"@amux_assistant":    "codex",
					"@amux_created_at":   strconv.FormatInt(now.Add(-1*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-5*time.Second).Unix(), 10),
				},
			},
			{
				Name: "amux-" + string(wsID) + "-t_active",
				Tags: map[string]string{
					"@amux_tab":          "t_active",
					"@amux_assistant":    "codex",
					"@amux_created_at":   strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-5*time.Second).Unix(), 10),
				},
			},
		}, nil
	}
	tmuxSessionStateFor = func(session string, _ tmux.Options) (tmux.SessionState, error) {
		switch session {
		case "amux-" + string(wsID) + "-t_uncertain", "amux-" + string(wsID) + "-t_active":
			return tmux.SessionState{Exists: true, HasLivePane: true}, nil
		default:
			t.Fatalf("unexpected session lookup: %s", session)
			return tmux.SessionState{}, nil
		}
	}
	tmuxCapturePaneTail = func(session string, _ int, _ tmux.Options) (string, bool) {
		switch session {
		case "amux-" + string(wsID) + "-t_uncertain":
			return "", false
		case "amux-" + string(wsID) + "-t_active":
			return "Running task now", true
		default:
			t.Fatalf("unexpected session capture: %s", session)
			return "", false
		}
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
	if got := taskString(payload, "status"); got != "needs_input" {
		t.Fatalf("status = %q, want %q", got, "needs_input")
	}
	if got := taskString(payload, "session_name"); got != "amux-"+string(wsID)+"-t_uncertain" {
		t.Fatalf("session_name = %q, want newer uncertain session", got)
	}
	if got := taskString(payload, "agent_id"); got != string(wsID)+":t_uncertain" {
		t.Fatalf("agent_id = %q, want uncertain session agent id", got)
	}
	if suggested := taskString(payload, "suggested_command"); !strings.Contains(suggested, string(wsID)+":t_uncertain") {
		t.Fatalf("suggested_command = %q, want uncertain session agent id", suggested)
	}
}

func TestCmdTaskStatus_PrefersNewerUncertainSessionOverOlderActiveRun(t *testing.T) {
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
				Name: "amux-" + string(wsID) + "-t_uncertain",
				Tags: map[string]string{
					"@amux_tab":          "t_uncertain",
					"@amux_assistant":    "droid",
					"@amux_created_at":   strconv.FormatInt(now.Add(-1*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-5*time.Second).Unix(), 10),
				},
			},
			{
				Name: "amux-" + string(wsID) + "-t_active",
				Tags: map[string]string{
					"@amux_tab":          "t_active",
					"@amux_assistant":    "droid",
					"@amux_created_at":   strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-5*time.Second).Unix(), 10),
				},
			},
		}, nil
	}
	tmuxSessionStateFor = func(session string, _ tmux.Options) (tmux.SessionState, error) {
		switch session {
		case "amux-" + string(wsID) + "-t_uncertain", "amux-" + string(wsID) + "-t_active":
			return tmux.SessionState{Exists: true, HasLivePane: true}, nil
		default:
			t.Fatalf("unexpected session lookup: %s", session)
			return tmux.SessionState{}, nil
		}
	}
	tmuxCapturePaneTail = func(session string, _ int, _ tmux.Options) (string, bool) {
		switch session {
		case "amux-" + string(wsID) + "-t_uncertain":
			return "", false
		case "amux-" + string(wsID) + "-t_active":
			return "Running task now", true
		default:
			t.Fatalf("unexpected session capture: %s", session)
			return "", false
		}
	}

	var out, errOut bytes.Buffer
	code := cmdTaskStatus(&out, &errOut, GlobalFlags{JSON: true}, []string{
		"--workspace", string(wsID),
		"--assistant", "droid",
	}, "test-v1")
	if code != ExitOK {
		t.Fatalf("cmdTaskStatus() code = %d, stderr=%q out=%q", code, errOut.String(), out.String())
	}

	payload := decodeTaskResult(t, out.Bytes())
	if got := taskString(payload, "status"); got != "attention" {
		t.Fatalf("status = %q, want %q", got, "attention")
	}
	if got := taskString(payload, "overall_status"); got != "in_progress" {
		t.Fatalf("overall_status = %q, want %q", got, "in_progress")
	}
	if got := taskString(payload, "session_name"); got != "amux-"+string(wsID)+"-t_uncertain" {
		t.Fatalf("session_name = %q, want newer uncertain session", got)
	}
	if got := taskString(payload, "agent_id"); got != string(wsID)+":t_uncertain" {
		t.Fatalf("agent_id = %q, want uncertain session agent id", got)
	}
}
