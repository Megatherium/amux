package center

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/vterm"
)

func TestIsTabActorReady_FalseWhenHeartbeatStale(t *testing.T) {
	m := newTestModel()
	m.setTabActorReady()
	m.tabActorLastBeat = time.Now().Add(-(tabActorStallTimeout + time.Second))

	if m.isTabActorReady() {
		t.Fatal("expected stale actor heartbeat to clear readiness")
	}
	if m.tabActorRunning {
		t.Fatal("expected stale readiness flag to be cleared")
	}
}

func TestUpdatePTYFlush_StaleActorHeartbeatForcesParserResetFallback(t *testing.T) {
	m := newTestModel()
	m.setTabActorReady()
	m.tabActorLastBeat = time.Now().Add(-(tabActorStallTimeout + time.Second))

	ws := newTestWorkspace("ws", "/repo/ws")
	wsID := string(ws.ID())
	tab := &Tab{
		ID:                 TabID("tab-1"),
		Assistant:          "codex",
		Workspace:          ws,
		Terminal:           vterm.New(80, 24),
		Running:            true,
		pendingOutput:      []byte("visible"),
		lastOutputAt:       time.Now().Add(-time.Second),
		flushPendingSince:  time.Now().Add(-time.Second),
		parserResetPending: true,
		actorWritesPending: 1,
	}
	m.tabsByWorkspace[wsID] = []*Tab{tab}
	m.activeTabByWorkspace[wsID] = 0
	m.workspace = ws

	_ = m.updatePTYFlush(PTYFlush{WorkspaceID: wsID, TabID: tab.ID})

	if tab.parserResetPending {
		t.Fatal("expected stale actor flush to clear parserResetPending")
	}
	if tab.actorWritesPending != 0 {
		t.Fatalf("expected stale actor flush to clear actorWritesPending, got %d", tab.actorWritesPending)
	}
	if len(tab.pendingOutput) == 0 {
		t.Fatal("expected pending output to remain queued for the follow-up flush")
	}
	if !tab.flushScheduled {
		t.Fatal("expected follow-up flush to be scheduled after stale actor fallback")
	}
}

func TestRunTabActor_SetsReadyViaMessage(t *testing.T) {
	m := newTestModel()
	m.tabEvents = make(chan tabEvent, 1)
	sinkMsgs := make(chan tea.Msg, 4)
	m.msgSink = func(msg tea.Msg) {
		sinkMsgs <- msg
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- m.RunTabActor(ctx)
	}()

	select {
	case msg := <-sinkMsgs:
		if _, ok := msg.(tabActorSignal); !ok {
			t.Fatalf("expected tabActorSignal on startup, got %T", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("expected actor startup to emit tabActorSignal message")
	}

	// Process the startup message to set readiness.
	m.Update(tabActorSignal{kind: "started"})

	if !m.isTabActorReady() {
		t.Fatal("expected actor to be ready after processing tabActorSignal")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunTabActor() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for actor to stop")
	}
}

func TestTabActorHeartbeat_RefreshesReadiness(t *testing.T) {
	m := newTestModel()
	// Simulate actor started
	m.Update(tabActorSignal{kind: "started"})

	if !m.isTabActorReady() {
		t.Fatal("expected actor ready after processing tabActorSignal")
	}

	// Advance time past the stall timeout
	before := m.tabActorLastBeat

	// A heartbeat message refreshes the timestamp
	m.Update(tabActorSignal{kind: "heartbeat"})

	if m.tabActorLastBeat.Equal(before) {
		t.Fatal("expected heartbeat message to refresh the last-beat timestamp")
	}
	if !m.isTabActorReady() {
		t.Fatal("expected actor to remain ready after heartbeat")
	}
}

func TestSetTabActorReady_SetsNonAtomicFields(t *testing.T) {
	m := newTestModel()

	m.setTabActorReady()

	if !m.tabActorRunning {
		t.Fatal("expected tabActorRunning to be set")
	}
	if m.tabActorLastBeat.IsZero() {
		t.Fatal("expected setTabActorReady to seed heartbeat timestamp")
	}
	if !m.isTabActorReady() {
		t.Fatal("expected actor to be ready immediately after setTabActorReady")
	}
}

func TestRunTabActor_EmitsRedrawForActorHandledUIEvent(t *testing.T) {
	m := newTestModel()
	m.tabEvents = make(chan tabEvent, 1)
	sinkMsgs := make(chan tea.Msg, 4)
	m.msgSink = func(msg tea.Msg) {
		sinkMsgs <- msg
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- m.RunTabActor(ctx)
	}()

	// Drain startup message
	select {
	case <-sinkMsgs:
	case <-time.After(time.Second):
		t.Fatal("expected startup message")
	}

	tab := &Tab{Terminal: vterm.New(80, 24)}
	m.tabEvents <- tabEvent{kind: tabEventSelectionStart, tab: tab}

	// The actor sends a heartbeat before processing the event, then a redraw after.
	// Drain the heartbeat first.
	var gotRedraw bool
	timeout := time.After(time.Second)
	for !gotRedraw {
		select {
		case msg := <-sinkMsgs:
			switch msg := msg.(type) {
			case tabActorSignal:
				if msg.kind == "redraw" {
					gotRedraw = true
				}
				// heartbeat is expected, drain it
			default:
				t.Fatalf("unexpected message: %T", msg)
			}
		case <-timeout:
			t.Fatal("expected actor-handled UI event to emit redraw message")
		}
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunTabActor() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for actor to stop")
	}
}

func TestRequestTabActorRedraw_AlwaysSendsViaMsgSink(t *testing.T) {
	m := newTestModel()
	sinkMsgs := make(chan tea.Msg, 4)
	m.msgSinkTry = func(msg tea.Msg) bool {
		sinkMsgs <- msg
		return true
	}

	// Multiple calls always send (no CAS coalescing).
	m.requestTabActorRedraw()
	m.requestTabActorRedraw()

	for i := 0; i < 2; i++ {
		select {
		case msg := <-sinkMsgs:
			if _, ok := msg.(tabActorSignal); !ok {
				t.Fatalf("expected redraw message, got %T", msg)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("expected redraw message %d", i+1)
		}
	}
}

func TestRequestTabActorRedraw_MsgSinkFallbackSendsEveryTime(t *testing.T) {
	m := newTestModel()
	sinkMsgs := make(chan tea.Msg, 4)
	m.msgSink = func(msg tea.Msg) {
		sinkMsgs <- msg
	}

	m.requestTabActorRedraw()
	m.requestTabActorRedraw()

	for i := 0; i < 2; i++ {
		select {
		case msg := <-sinkMsgs:
			if _, ok := msg.(tabActorSignal); !ok {
				t.Fatalf("expected redraw message, got %T", msg)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("expected redraw message %d from msgSink fallback", i+1)
		}
	}
}

func TestRequestTabActorRedraw_RetryOnDrop(t *testing.T) {
	m := newTestModel()
	attempts := 0
	sinkMsgs := make(chan tea.Msg, 2)
	m.msgSinkTry = func(msg tea.Msg) bool {
		attempts++
		if attempts == 1 {
			return false
		}
		sinkMsgs <- msg
		return true
	}
	m.msgSink = func(msg tea.Msg) {
		_ = m.msgSinkTry(msg)
	}

	// First call dropped, second succeeds.
	m.requestTabActorRedraw()

	m.requestTabActorRedraw()
	select {
	case msg := <-sinkMsgs:
		if _, ok := msg.(tabActorSignal); !ok {
			t.Fatalf("expected redraw message after retry, got %T", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected redraw retry after dropped enqueue")
	}
}

func TestRunTabActor_EventProcessingSendsHeartbeat(t *testing.T) {
	m := newTestModel()
	m.tabEvents = make(chan tabEvent, 1)
	sinkMsgs := make(chan tea.Msg, 8)
	m.msgSink = func(msg tea.Msg) {
		sinkMsgs <- msg
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- m.RunTabActor(ctx)
	}()

	// Drain startup message
	select {
	case <-sinkMsgs:
	case <-time.After(time.Second):
		t.Fatal("expected startup message")
	}

	// Make the heartbeat appear stale via direct field write (simulating Update processing)
	m.Update(tabActorSignal{kind: "started"})

	// Send an event to trigger a heartbeat
	m.tabEvents <- tabEvent{kind: tabEventSelectionClear, tab: &Tab{}}

	// Should get a heartbeat message
	var gotHeartbeat bool
	timeout := time.After(time.Second)
	for !gotHeartbeat {
		select {
		case msg := <-sinkMsgs:
			if _, ok := msg.(tabActorSignal); ok {
				gotHeartbeat = true
			}
		case <-timeout:
			t.Fatal("expected heartbeat message after event processing")
		}
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunTabActor() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for actor to stop")
	}
}
