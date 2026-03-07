package cli

import (
	"strconv"
	"testing"
	"time"

	"github.com/andyrewlee/amux/internal/tmux"
)

func TestFindLatestTaskAgentSnapshot_SkipsPaneCaptureForExitedSessions(t *testing.T) {
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
				Name: "amux-ws-priority-t_dead_new",
				Tags: map[string]string{
					"@amux_tab":        "t_dead_new",
					"@amux_assistant":  "codex",
					"@amux_created_at": strconv.FormatInt(now.Add(-1*time.Minute).Unix(), 10),
				},
			},
			{
				Name: "amux-ws-priority-t_dead_old",
				Tags: map[string]string{
					"@amux_tab":        "t_dead_old",
					"@amux_assistant":  "codex",
					"@amux_created_at": strconv.FormatInt(now.Add(-2*time.Minute).Unix(), 10),
				},
			},
			{
				Name: "amux-ws-priority-t_live",
				Tags: map[string]string{
					"@amux_tab":          "t_live",
					"@amux_assistant":    "codex",
					"@amux_created_at":   strconv.FormatInt(now.Add(-3*time.Minute).Unix(), 10),
					tmux.TagLastOutputAt: strconv.FormatInt(now.Add(-5*time.Second).Unix(), 10),
				},
			},
		}, nil
	}
	tmuxSessionStateFor = func(session string, _ tmux.Options) (tmux.SessionState, error) {
		switch session {
		case "amux-ws-priority-t_dead_new", "amux-ws-priority-t_dead_old":
			return tmux.SessionState{Exists: true, HasLivePane: false}, nil
		case "amux-ws-priority-t_live":
			return tmux.SessionState{Exists: true, HasLivePane: true}, nil
		default:
			t.Fatalf("unexpected session lookup: %s", session)
			return tmux.SessionState{}, nil
		}
	}
	tmuxCapturePaneTail = func(session string, _ int, _ tmux.Options) (string, bool) {
		switch session {
		case "amux-ws-priority-t_dead_new", "amux-ws-priority-t_dead_old":
			t.Fatalf("capture should not run for dead session %s", session)
			return "", false
		case "amux-ws-priority-t_live":
			return "Running task now", true
		default:
			t.Fatalf("unexpected session capture: %s", session)
			return "", false
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
		if candidate.SessionName != "amux-ws-priority-t_live" {
			t.Fatalf("mode=%d session = %q, want live session after skipping dead tabs", mode, candidate.SessionName)
		}
		if snap.SessionExited {
			t.Fatalf("mode=%d selected snapshot should not be exited", mode)
		}
	}
}
