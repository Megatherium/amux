package cli

import (
	"bytes"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/andyrewlee/amux/internal/tmux"
)

func TestCmdTaskStatus_PrefersNewerCompletedRunOverOlderExitedTab(t *testing.T) {
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
				Name: "amux-" + string(wsID) + "-t_done",
				Tags: map[string]string{
					"@amux_tab":          "t_done",
					"@amux_assistant":    "droid",
					"@amux_created_at":   strconv.FormatInt(now.Add(-1*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-2*time.Minute).Unix(), 10),
				},
			},
			{
				Name: "amux-" + string(wsID) + "-t_dead",
				Tags: map[string]string{
					"@amux_tab":        "t_dead",
					"@amux_assistant":  "droid",
					"@amux_created_at": strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10),
				},
			},
		}, nil
	}
	tmuxCapturePaneTail = func(session string, _ int, _ tmux.Options) (string, bool) {
		switch session {
		case "amux-" + string(wsID) + "-t_done":
			return "Review completed with findings.", true
		case "amux-" + string(wsID) + "-t_dead":
			return "", false
		default:
			t.Fatalf("unexpected session capture: %s", session)
			return "", false
		}
	}
	tmuxSessionStateFor = func(session string, _ tmux.Options) (tmux.SessionState, error) {
		switch session {
		case "amux-" + string(wsID) + "-t_done":
			return tmux.SessionState{Exists: true, HasLivePane: true}, nil
		case "amux-" + string(wsID) + "-t_dead":
			return tmux.SessionState{Exists: true, HasLivePane: false}, nil
		default:
			t.Fatalf("unexpected session lookup: %s", session)
			return tmux.SessionState{}, nil
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
	if got := taskString(payload, "status"); got != "idle" {
		t.Fatalf("status = %q, want %q", got, "idle")
	}
	if got := taskString(payload, "overall_status"); got != "completed" {
		t.Fatalf("overall_status = %q, want %q", got, "completed")
	}
	if got := taskString(payload, "session_name"); got != "amux-"+string(wsID)+"-t_done" {
		t.Fatalf("session_name = %q, want newer completed session", got)
	}
}

func TestCmdTaskStart_BlocksOnNewerUncertainSessionOverOlderCompletedRun(t *testing.T) {
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
				Name: "amux-" + string(wsID) + "-t_done",
				Tags: map[string]string{
					"@amux_tab":          "t_done",
					"@amux_assistant":    "codex",
					"@amux_created_at":   strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-2*time.Minute).Unix(), 10),
				},
			},
		}, nil
	}
	tmuxCapturePaneTail = func(session string, _ int, _ tmux.Options) (string, bool) {
		switch session {
		case "amux-" + string(wsID) + "-t_uncertain":
			return "", false
		case "amux-" + string(wsID) + "-t_done":
			return "Review completed with findings.", true
		default:
			t.Fatalf("unexpected session capture: %s", session)
			return "", false
		}
	}
	tmuxSessionStateFor = func(session string, _ tmux.Options) (tmux.SessionState, error) {
		switch session {
		case "amux-" + string(wsID) + "-t_uncertain":
			return tmux.SessionState{}, errors.New("tmux state lookup timed out")
		case "amux-" + string(wsID) + "-t_done":
			return tmux.SessionState{Exists: true, HasLivePane: true}, nil
		default:
			t.Fatalf("unexpected session lookup: %s", session)
			return tmux.SessionState{}, nil
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
	if got := taskString(payload, "latest_line"); got != "(no visible output yet)" {
		t.Fatalf("latest_line = %q, want placeholder for uncertain capture miss", got)
	}
}

func TestFindLatestTaskAgentSnapshot_PrefersOlderUncertainOverNewerCompleted(t *testing.T) {
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
				Name: "amux-ws-priority-t_done",
				Tags: map[string]string{
					"@amux_tab":          "t_done",
					"@amux_assistant":    "codex",
					"@amux_created_at":   strconv.FormatInt(now.Add(-1*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-2*time.Minute).Unix(), 10),
				},
			},
			{
				Name: "amux-ws-priority-t_uncertain",
				Tags: map[string]string{
					"@amux_tab":          "t_uncertain",
					"@amux_assistant":    "codex",
					"@amux_created_at":   strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-5*time.Second).Unix(), 10),
				},
			},
		}, nil
	}
	tmuxCapturePaneTail = func(session string, _ int, _ tmux.Options) (string, bool) {
		switch session {
		case "amux-ws-priority-t_done":
			return "Review completed with findings.", true
		case "amux-ws-priority-t_uncertain":
			return "", false
		default:
			t.Fatalf("unexpected session capture: %s", session)
			return "", false
		}
	}
	tmuxSessionStateFor = func(session string, _ tmux.Options) (tmux.SessionState, error) {
		switch session {
		case "amux-ws-priority-t_done":
			return tmux.SessionState{Exists: true, HasLivePane: true}, nil
		case "amux-ws-priority-t_uncertain":
			return tmux.SessionState{Exists: true, HasLivePane: true}, nil
		default:
			t.Fatalf("unexpected session lookup: %s", session)
			return tmux.SessionState{}, nil
		}
	}

	for _, mode := range []taskAgentLookupMode{taskAgentLookupForStart, taskAgentLookupForStatus} {
		candidate, snap, err := findLatestTaskAgentSnapshot(tmux.Options{}, "ws-priority", "codex", mode)
		if err != nil {
			t.Fatalf("findLatestTaskAgentSnapshot(mode=%d) error = %v", mode, err)
		}
		if candidate == nil {
			t.Fatalf("findLatestTaskAgentSnapshot(mode=%d) returned nil candidate", mode)
		}
		if candidate.SessionName != "amux-ws-priority-t_uncertain" {
			t.Fatalf("mode=%d session = %q, want older uncertain session", mode, candidate.SessionName)
		}
		if taskStatusLooksComplete(*candidate, snap) {
			t.Fatalf("mode=%d selected snapshot should not be complete", mode)
		}
	}
}

func TestCmdTaskStatus_PrefersNewerUncertainSessionOverOlderCompletedRun(t *testing.T) {
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
				Name: "amux-" + string(wsID) + "-t_done",
				Tags: map[string]string{
					"@amux_tab":          "t_done",
					"@amux_assistant":    "droid",
					"@amux_created_at":   strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-2*time.Minute).Unix(), 10),
				},
			},
		}, nil
	}
	tmuxCapturePaneTail = func(session string, _ int, _ tmux.Options) (string, bool) {
		switch session {
		case "amux-" + string(wsID) + "-t_uncertain":
			return "", false
		case "amux-" + string(wsID) + "-t_done":
			return "Review completed with findings.", true
		default:
			t.Fatalf("unexpected session capture: %s", session)
			return "", false
		}
	}
	tmuxSessionStateFor = func(session string, _ tmux.Options) (tmux.SessionState, error) {
		switch session {
		case "amux-" + string(wsID) + "-t_uncertain":
			return tmux.SessionState{Exists: true, HasLivePane: true}, nil
		case "amux-" + string(wsID) + "-t_done":
			return tmux.SessionState{Exists: true, HasLivePane: true}, nil
		default:
			t.Fatalf("unexpected session lookup: %s", session)
			return tmux.SessionState{}, nil
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
}

func TestFindLatestTaskAgentSnapshot_PrefersOlderQuietActiveTabOverNewerCompleted(t *testing.T) {
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
				Name: "amux-ws-quiet-t_done",
				Tags: map[string]string{
					"@amux_tab":          "t_done",
					"@amux_assistant":    "codex",
					"@amux_created_at":   strconv.FormatInt(now.Add(-1*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-2*time.Minute).Unix(), 10),
				},
			},
			{
				Name: "amux-ws-quiet-t_old_active",
				Tags: map[string]string{
					"@amux_tab":          "t_old_active",
					"@amux_assistant":    "codex",
					"@amux_created_at":   strconv.FormatInt(now.Add(-20*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10),
				},
			},
		}, nil
	}
	tmuxCapturePaneTail = func(session string, _ int, _ tmux.Options) (string, bool) {
		switch session {
		case "amux-ws-quiet-t_done":
			return "Review completed with findings.", true
		case "amux-ws-quiet-t_old_active":
			return "(no visible output yet)", true
		default:
			t.Fatalf("unexpected capture for session %s", session)
			return "", false
		}
	}
	tmuxSessionStateFor = func(session string, _ tmux.Options) (tmux.SessionState, error) {
		switch session {
		case "amux-ws-quiet-t_done", "amux-ws-quiet-t_old_active":
			return tmux.SessionState{Exists: true, HasLivePane: true}, nil
		default:
			t.Fatalf("unexpected session lookup: %s", session)
			return tmux.SessionState{}, nil
		}
	}

	for _, mode := range []taskAgentLookupMode{taskAgentLookupForStart, taskAgentLookupForStatus} {
		candidate, snap, err := findLatestTaskAgentSnapshot(tmux.Options{}, "ws-quiet", "codex", mode)
		if err != nil {
			t.Fatalf("findLatestTaskAgentSnapshot(mode=%d) error = %v", mode, err)
		}
		if candidate == nil {
			t.Fatalf("findLatestTaskAgentSnapshot(mode=%d) returned nil candidate", mode)
		}
		if candidate.SessionName != "amux-ws-quiet-t_old_active" {
			t.Fatalf("mode=%d session = %q, want older quiet active session", mode, candidate.SessionName)
		}
		if taskStatusLooksComplete(*candidate, snap) {
			t.Fatalf("mode=%d selected snapshot should not be complete", mode)
		}
	}
}
