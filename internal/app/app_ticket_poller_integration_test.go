package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andyrewlee/amux/internal/discovery"
	"github.com/andyrewlee/amux/internal/supervisor"
	"github.com/andyrewlee/amux/internal/tickets"
)

// ---------------------------------------------------------------------------
// Integration: ensureTicketPoller
// ---------------------------------------------------------------------------

func TestEnsureTicketPoller_CreatesPollerOnFirstCall(t *testing.T) {
	app := &App{
		supervisor: newTestSupervisor(),
	}
	defer app.supervisor.Stop()

	reg, _ := discovery.NewRegistry(t.TempDir())
	svc := tickets.NewTicketService(&mockTicketStore{}, reg, tickets.NewRenderer())

	app.ensureTicketPoller("/project", svc)
	if app.ticketPoller == nil {
		t.Fatal("expected ticketPoller to be created")
	}
	if len(app.ticketPoller.services) != 1 {
		t.Fatalf("expected 1 service in poller, got %d", len(app.ticketPoller.services))
	}
}

func TestEnsureTicketPoller_AddsToExistingPoller(t *testing.T) {
	app := &App{
		supervisor: newTestSupervisor(),
	}
	defer app.supervisor.Stop()

	reg, _ := discovery.NewRegistry(t.TempDir())
	svc1 := tickets.NewTicketService(&mockTicketStore{}, reg, tickets.NewRenderer())
	svc2 := tickets.NewTicketService(&mockTicketStore{}, reg, tickets.NewRenderer())

	app.ensureTicketPoller("/a", svc1)
	app.ensureTicketPoller("/b", svc2)

	if app.ticketPoller == nil {
		t.Fatal("expected ticketPoller to be created")
	}
	if len(app.ticketPoller.services) != 2 {
		t.Fatalf("expected 2 services in poller, got %d", len(app.ticketPoller.services))
	}
}

func TestEnsureTicketPoller_NilSupervisor_Noop(t *testing.T) {
	app := &App{}
	reg, _ := discovery.NewRegistry(t.TempDir())
	svc := tickets.NewTicketService(&mockTicketStore{}, reg, tickets.NewRenderer())

	// Should not panic.
	app.ensureTicketPoller("/project", svc)
	if app.ticketPoller != nil {
		t.Fatal("expected no poller with nil supervisor")
	}
}

func TestEnsureTicketPoller_NilService_Noop(t *testing.T) {
	app := &App{
		supervisor: newTestSupervisor(),
	}
	defer app.supervisor.Stop()

	app.ensureTicketPoller("/project", nil)
	if app.ticketPoller != nil {
		t.Fatal("expected no poller with nil service")
	}
}

// ---------------------------------------------------------------------------
// Integration: handleTicketStoreResult with poller
// ---------------------------------------------------------------------------

func TestHandleTicketStoreResult_StartsPoller(t *testing.T) {
	app := &App{
		supervisor: newTestSupervisor(),
		ui:         &UICompositor{},
	}
	defer app.supervisor.Stop()

	reg, _ := discovery.NewRegistry(t.TempDir())
	svc := tickets.NewTicketService(&mockTicketStore{}, reg, tickets.NewRenderer())

	result := ticketStoreResult{
		projectPath: "/project",
		service:     svc,
	}
	cmds := app.handleTicketStoreResult(result)

	// Should return 1 cmd (loadTicketsForProject).
	if len(cmds) != 1 {
		t.Fatalf("expected 1 cmd, got %d", len(cmds))
	}

	// Poller should be created and have the service.
	if app.ticketPoller == nil {
		t.Fatal("expected ticketPoller to be created")
	}
	if len(app.ticketPoller.services) != 1 {
		t.Fatalf("expected 1 service in poller, got %d", len(app.ticketPoller.services))
	}
}

func TestHandleTicketStoreResult_ErrorDoesNotStartPoller(t *testing.T) {
	app := &App{
		supervisor: newTestSupervisor(),
		ui:         &UICompositor{},
	}
	defer app.supervisor.Stop()

	result := ticketStoreResult{
		projectPath: "/project",
		err:         errTestTicketStoreFailed,
	}
	app.handleTicketStoreResult(result)

	if app.ticketPoller != nil {
		t.Fatal("expected no poller on error result")
	}
}

func TestHandleTicketStoreResult_NilServiceDoesNotStartPoller(t *testing.T) {
	app := &App{
		supervisor: newTestSupervisor(),
		ui:         &UICompositor{},
	}
	defer app.supervisor.Stop()

	result := ticketStoreResult{
		projectPath: "/project",
		service:     nil,
	}
	app.handleTicketStoreResult(result)

	if app.ticketPoller != nil {
		t.Fatal("expected no poller when service is nil")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestSupervisor creates a supervisor suitable for tests.
func newTestSupervisor() *supervisor.Supervisor {
	return supervisor.New(context.Background())
}

// ---------------------------------------------------------------------------
// removeService
// ---------------------------------------------------------------------------

func TestTicketPoller_RemoveService_NilPoller(t *testing.T) {
	var p *ticketPoller
	// Should not panic.
	p.removeService("/test")
}

func TestTicketPoller_RemoveService_DeletesFromMaps(t *testing.T) {
	p := newTicketPoller(nil, time.Second)
	store := &mockTicketStore{}
	reg, _ := discovery.NewRegistry(t.TempDir())
	svc := tickets.NewTicketService(store, reg, tickets.NewRenderer())

	p.addService("/project", svc)

	// Set a lastUpdate entry to verify it gets cleaned up.
	p.mu.Lock()
	p.lastUpdate["/project"] = time.Now()
	p.mu.Unlock()

	if len(p.services) != 1 || len(p.lastUpdate) != 1 {
		t.Fatal("expected 1 service and 1 lastUpdate before remove")
	}

	p.removeService("/project")

	if len(p.services) != 0 {
		t.Fatalf("expected 0 services after remove, got %d", len(p.services))
	}
	if len(p.lastUpdate) != 0 {
		t.Fatalf("expected 0 lastUpdate entries after remove, got %d", len(p.lastUpdate))
	}
}

func TestTicketPoller_RemoveService_NonExistentPath_Noop(t *testing.T) {
	p := newTicketPoller(nil, time.Second)
	store := &mockTicketStore{}
	reg, _ := discovery.NewRegistry(t.TempDir())
	svc := tickets.NewTicketService(store, reg, tickets.NewRenderer())

	p.addService("/project", svc)

	// Remove a path that was never added — should not affect existing entries.
	p.removeService("/nonexistent")

	if len(p.services) != 1 {
		t.Fatalf("expected 1 service after removing nonexistent path, got %d", len(p.services))
	}
}

func TestTicketPoller_RemoveService_StopsPollingProject(t *testing.T) {
	collector := &msgCollector{}
	p := newTicketPoller(collector.send, time.Second)

	now := time.Now()
	store := &mockTicketStore{
		tickets: []tickets.Ticket{{ID: "bmx-1", Title: "Test", Status: "open", UpdatedAt: now}},
		latest:  now,
	}
	reg, _ := discovery.NewRegistry(t.TempDir())
	svc := tickets.NewTicketService(store, reg, tickets.NewRenderer())
	p.addService("/project", svc)

	// First poll: should send.
	p.pollOnce(context.Background())
	if collector.count() != 1 {
		t.Fatalf("expected 1 message after first poll, got %d", collector.count())
	}

	// Remove the service.
	p.removeService("/project")

	// Advance the timestamp.
	store.setLatest(now.Add(1 * time.Minute))

	// Second poll: should NOT send because the project was removed.
	p.pollOnce(context.Background())
	if collector.count() != 1 {
		t.Fatalf("expected no new message after remove, got %d total", collector.count())
	}
}

// ---------------------------------------------------------------------------
// run — supervisor worker lifecycle
// ---------------------------------------------------------------------------

func TestTicketPoller_Run_StopsOnContextCancel(t *testing.T) {
	collector := &msgCollector{}
	p := newTicketPoller(collector.send, 10*time.Millisecond)

	now := time.Now()
	store := &mockTicketStore{latest: now}
	reg, _ := discovery.NewRegistry(t.TempDir())
	svc := tickets.NewTicketService(store, reg, tickets.NewRenderer())
	p.addService("/project", svc)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = p.run(ctx)
		close(done)
	}()

	// Let it tick at least once.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// OK — poller exited cleanly.
	case <-time.After(time.Second):
		t.Fatal("poller did not stop on context cancel")
	}
}

func TestTicketPoller_Run_PollsMultipleTimes(t *testing.T) {
	collector := &msgCollector{}
	p := newTicketPoller(collector.send, 20*time.Millisecond)

	now := time.Now()
	store := &mockTicketStore{
		tickets: []tickets.Ticket{{ID: "bmx-1", Title: "Test", Status: "open", UpdatedAt: now}},
		latest:  now,
	}
	reg, _ := discovery.NewRegistry(t.TempDir())
	svc := tickets.NewTicketService(store, reg, tickets.NewRenderer())
	p.addService("/project", svc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var runErr error
	done := make(chan struct{})
	go func() {
		runErr = p.run(ctx)
		close(done)
	}()

	// Wait for first poll.
	deadline := time.After(2 * time.Second)
	for collector.count() < 1 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for first poll")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Change the timestamp to trigger a second poll.
	store.setLatest(now.Add(1 * time.Minute))

	// Wait for second poll.
	deadline = time.After(2 * time.Second)
	for collector.count() < 2 {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for second poll, got %d messages", collector.count())
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	cancel()
	<-done

	if runErr != nil {
		if runErr.Error() != "context canceled" {
			t.Errorf("run returned unexpected error: %v", runErr)
		}
	}
}

// ---------------------------------------------------------------------------
// Concurrency safety
// ---------------------------------------------------------------------------

func TestTicketPoller_ConcurrentAddAndPoll(t *testing.T) {
	collector := &msgCollector{}
	p := newTicketPoller(collector.send, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the poller running.
	go func() {
		_ = p.run(ctx)
	}()

	// Concurrently add services.
	var added atomic.Int32
	for i := 0; i < 10; i++ {
		go func(idx int) {
			now := time.Now()
			store := &mockTicketStore{
				tickets: []tickets.Ticket{
					{ID: "bmx-1", Title: "Test", Status: "open", UpdatedAt: now},
				},
				latest: now,
			}
			reg, _ := discovery.NewRegistry(t.TempDir())
			svc := tickets.NewTicketService(store, reg, tickets.NewRenderer())
			p.addService("/project", svc)
			added.Add(1)
		}(i)
	}

	// Wait for all adds.
	deadline := time.After(2 * time.Second)
	for added.Load() < 10 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for concurrent adds")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	// The poller should have processed at least one tick without panicking.
	time.Sleep(50 * time.Millisecond)

	// Verify we got at least one message.
	if collector.count() == 0 {
		t.Fatal("expected at least one message from concurrent poll")
	}
}

// ---------------------------------------------------------------------------
// Context cancellation propagation
// ---------------------------------------------------------------------------

func TestTicketPoller_PollOnce_RespectsContextCancel(t *testing.T) {
	collector := &msgCollector{}
	p := newTicketPoller(collector.send, time.Second)

	// Create a store whose LatestUpdate blocks until canceled.
	store := &mockTicketStore{
		latestFn: func(ctx context.Context) (time.Time, error) {
			<-ctx.Done()
			return time.Time{}, ctx.Err()
		},
	}
	reg, _ := discovery.NewRegistry(t.TempDir())
	svc := tickets.NewTicketService(store, reg, tickets.NewRenderer())
	p.addService("/project", svc)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// pollOnce should return (not hang) once the context expires.
	done := make(chan struct{})
	go func() {
		p.pollOnce(ctx)
		close(done)
	}()

	select {
	case <-done:
		// OK — pollOnce returned after context expiry.
	case <-time.After(2 * time.Second):
		t.Fatal("pollOnce did not return after context cancellation")
	}

	if collector.count() != 0 {
		t.Fatalf("expected no messages on canceled context, got %d", collector.count())
	}
}
