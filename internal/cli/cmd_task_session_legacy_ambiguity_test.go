package cli

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/tmux"
)

func TestCmdTaskStart_IgnoresAmbiguousLegacySessionWhenExactAssistantMatchCompleted(t *testing.T) {
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
				Name: "amux-" + string(wsID) + "-t_done",
				Tags: map[string]string{
					"@amux_tab":          "t_done",
					"@amux_assistant":    "codex",
					"@amux_created_at":   strconv.FormatInt(now.Add(-1*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-2*time.Minute).Unix(), 10),
				},
			},
			{
				Name: "amux-" + string(wsID) + "-t_claude",
				Tags: map[string]string{
					"@amux_tab":          "t_claude",
					"@amux_assistant":    "claude",
					"@amux_created_at":   strconv.FormatInt(now.Add(-2*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-5*time.Second).Unix(), 10),
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
	tmuxSessionStateFor = func(session string, _ tmux.Options) (tmux.SessionState, error) {
		switch session {
		case "amux-" + string(wsID) + "-t_done":
			return tmux.SessionState{Exists: true, HasLivePane: true}, nil
		case "amux-" + string(wsID) + "-t_claude", "amux-" + string(wsID) + "-t_legacy":
			t.Fatalf("assistant-scoped codex lookup should not inspect ambiguous session %s", session)
			return tmux.SessionState{}, nil
		default:
			t.Fatalf("unexpected session lookup: %s", session)
			return tmux.SessionState{}, nil
		}
	}
	tmuxCapturePaneTail = func(session string, _ int, _ tmux.Options) (string, bool) {
		switch session {
		case "amux-" + string(wsID) + "-t_done":
			return "Review completed with findings.", true
		case "amux-" + string(wsID) + "-t_claude", "amux-" + string(wsID) + "-t_legacy":
			t.Fatalf("assistant-scoped codex lookup should not capture ambiguous session %s", session)
			return "", false
		default:
			t.Fatalf("unexpected session capture: %s", session)
			return "", false
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
				Summary:    "Started new run after exact assistant session completed.",
				LatestLine: "Started new run after exact assistant session completed.",
			},
		}, nil
	}

	var out, errOut bytes.Buffer
	code := cmdTaskStart(&out, &errOut, GlobalFlags{JSON: true}, []string{
		"--workspace", string(wsID),
		"--assistant", "codex",
		"--prompt", "Run a fresh review now",
	}, "test-v1")
	if code != ExitOK {
		t.Fatalf("cmdTaskStart() code = %d, stderr=%q out=%q", code, errOut.String(), out.String())
	}

	payload := decodeTaskResult(t, out.Bytes())
	if got := taskString(payload, "status"); got == "needs_input" {
		t.Fatalf("status = %q, want completed exact match to allow fresh run", got)
	}
	if got := taskString(payload, "session_name"); got != "sess-next" {
		t.Fatalf("session_name = %q, want new task session", got)
	}
}

func TestCmdTaskStatus_IgnoresAmbiguousLegacySessionWhenExactAssistantMatchCompleted(t *testing.T) {
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
					"@amux_assistant":    "codex",
					"@amux_created_at":   strconv.FormatInt(now.Add(-1*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-2*time.Minute).Unix(), 10),
				},
			},
			{
				Name: "amux-" + string(wsID) + "-t_claude",
				Tags: map[string]string{
					"@amux_tab":          "t_claude",
					"@amux_assistant":    "claude",
					"@amux_created_at":   strconv.FormatInt(now.Add(-2*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-5*time.Second).Unix(), 10),
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
	tmuxSessionStateFor = func(session string, _ tmux.Options) (tmux.SessionState, error) {
		switch session {
		case "amux-" + string(wsID) + "-t_done":
			return tmux.SessionState{Exists: true, HasLivePane: true}, nil
		case "amux-" + string(wsID) + "-t_claude", "amux-" + string(wsID) + "-t_legacy":
			t.Fatalf("assistant-scoped codex lookup should not inspect ambiguous session %s", session)
			return tmux.SessionState{}, nil
		default:
			t.Fatalf("unexpected session lookup: %s", session)
			return tmux.SessionState{}, nil
		}
	}
	tmuxCapturePaneTail = func(session string, _ int, _ tmux.Options) (string, bool) {
		switch session {
		case "amux-" + string(wsID) + "-t_done":
			return "Review completed with findings.", true
		case "amux-" + string(wsID) + "-t_claude", "amux-" + string(wsID) + "-t_legacy":
			t.Fatalf("assistant-scoped codex lookup should not capture ambiguous session %s", session)
			return "", false
		default:
			t.Fatalf("unexpected session capture: %s", session)
			return "", false
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
	if got := taskString(payload, "status"); got != "idle" {
		t.Fatalf("status = %q, want %q", got, "idle")
	}
	if got := taskString(payload, "overall_status"); got != "completed" {
		t.Fatalf("overall_status = %q, want %q", got, "completed")
	}
	if got := taskString(payload, "session_name"); got != "amux-"+string(wsID)+"-t_done" {
		t.Fatalf("session_name = %q, want exact assistant-tagged session", got)
	}
}

func TestCmdTaskStart_IgnoresAmbiguousLegacySessionWhenNoExactAssistantMatchExists(t *testing.T) {
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
				Name: "amux-" + string(wsID) + "-t_claude",
				Tags: map[string]string{
					"@amux_tab":          "t_claude",
					"@amux_assistant":    "claude",
					"@amux_created_at":   strconv.FormatInt(now.Add(-2*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-5*time.Second).Unix(), 10),
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
	tmuxSessionStateFor = func(session string, _ tmux.Options) (tmux.SessionState, error) {
		t.Fatalf("assistant-scoped codex lookup should not inspect ambiguous session %s", session)
		return tmux.SessionState{}, nil
	}
	tmuxCapturePaneTail = func(session string, _ int, _ tmux.Options) (string, bool) {
		t.Fatalf("assistant-scoped codex lookup should not capture ambiguous session %s", session)
		return "", false
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
				Summary:    "Started new run with no exact assistant-tagged session present.",
				LatestLine: "Started new run with no exact assistant-tagged session present.",
			},
		}, nil
	}

	var out, errOut bytes.Buffer
	code := cmdTaskStart(&out, &errOut, GlobalFlags{JSON: true}, []string{
		"--workspace", string(wsID),
		"--assistant", "codex",
		"--prompt", "Run a fresh review now",
	}, "test-v1")
	if code != ExitOK {
		t.Fatalf("cmdTaskStart() code = %d, stderr=%q out=%q", code, errOut.String(), out.String())
	}

	payload := decodeTaskResult(t, out.Bytes())
	if got := taskString(payload, "status"); got == "needs_input" {
		t.Fatalf("status = %q, want ambiguous legacy session to be ignored", got)
	}
	if got := taskString(payload, "session_name"); got != "sess-next" {
		t.Fatalf("session_name = %q, want new task session", got)
	}
}

func TestCmdTaskStart_IgnoresLegacySessionWhenNoExactAssistantMatchExists(t *testing.T) {
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
				Name: "amux-" + string(wsID) + "-t_legacy",
				Tags: map[string]string{
					"@amux_tab":          "t_legacy",
					"@amux_created_at":   strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-5*time.Second).Unix(), 10),
				},
			},
		}, nil
	}
	tmuxSessionStateFor = func(session string, _ tmux.Options) (tmux.SessionState, error) {
		t.Fatalf("assistant-scoped codex lookup should not inspect legacy-only session %s", session)
		return tmux.SessionState{}, nil
	}
	tmuxCapturePaneTail = func(session string, _ int, _ tmux.Options) (string, bool) {
		t.Fatalf("assistant-scoped codex lookup should not capture legacy-only session %s", session)
		return "", false
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
				Summary:    "Started new run with no exact assistant-tagged session present.",
				LatestLine: "Started new run with no exact assistant-tagged session present.",
			},
		}, nil
	}

	var out, errOut bytes.Buffer
	code := cmdTaskStart(&out, &errOut, GlobalFlags{JSON: true}, []string{
		"--workspace", string(wsID),
		"--assistant", "codex",
		"--prompt", "Run a fresh review now",
	}, "test-v1")
	if code != ExitOK {
		t.Fatalf("cmdTaskStart() code = %d, stderr=%q out=%q", code, errOut.String(), out.String())
	}

	payload := decodeTaskResult(t, out.Bytes())
	if got := taskString(payload, "status"); got == "needs_input" {
		t.Fatalf("status = %q, want legacy-only session to be ignored", got)
	}
	if got := taskString(payload, "session_name"); got != "sess-next" {
		t.Fatalf("session_name = %q, want new task session", got)
	}
}

func TestCmdTaskStatus_IgnoresLegacySessionWhenNoExactAssistantMatchExists(t *testing.T) {
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
				Name: "amux-" + string(wsID) + "-t_legacy",
				Tags: map[string]string{
					"@amux_tab":          "t_legacy",
					"@amux_created_at":   strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-5*time.Second).Unix(), 10),
				},
			},
		}, nil
	}
	tmuxSessionStateFor = func(session string, _ tmux.Options) (tmux.SessionState, error) {
		t.Fatalf("assistant-scoped codex lookup should not inspect legacy-only session %s", session)
		return tmux.SessionState{}, nil
	}
	tmuxCapturePaneTail = func(session string, _ int, _ tmux.Options) (string, bool) {
		t.Fatalf("assistant-scoped codex lookup should not capture legacy-only session %s", session)
		return "", false
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
	if got := taskString(payload, "status"); got != "idle" {
		t.Fatalf("status = %q, want %q", got, "idle")
	}
	if got := taskString(payload, "overall_status"); got != "completed" {
		t.Fatalf("overall_status = %q, want %q", got, "completed")
	}
	if got := taskString(payload, "session_name"); got != "" {
		t.Fatalf("session_name = %q, want no selected session", got)
	}
}
