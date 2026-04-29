package app

import (
	"context"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/discovery"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/tickets"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// mockTicketStore implements tickets.TicketStore for poller tests.
type mockTicketStore struct {
	mu       sync.Mutex
	tickets  []tickets.Ticket
	latest   time.Time
	listErr  error
	latestFn func(ctx context.Context) (time.Time, error) // optional override
}

func (s *mockTicketStore) ListTickets(_ context.Context, f tickets.TicketFilter) ([]tickets.Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listErr != nil {
		return nil, s.listErr
	}
	var result []tickets.Ticket
	for _, t := range s.tickets {
		if f.Status != "" && t.Status != f.Status {
			continue
		}
		result = append(result, t)
		if f.Limit > 0 && len(result) >= f.Limit {
			break
		}
	}
	return result, nil
}

func (s *mockTicketStore) LatestUpdate(ctx context.Context) (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.latestFn != nil {
		return s.latestFn(ctx)
	}
	return s.latest, nil
}

func (s *mockTicketStore) setLatest(t time.Time) {
	s.mu.Lock()
	s.latest = t
	s.mu.Unlock()
}

func (s *mockTicketStore) setTickets(ts []tickets.Ticket) {
	s.mu.Lock()
	s.tickets = ts
	s.mu.Unlock()
}

// msgCollector captures messages sent via the poller callback.
type msgCollector struct {
	mu   sync.Mutex
	msgs []tea.Msg
}

func (c *msgCollector) send(msg tea.Msg) {
	c.mu.Lock()
	c.msgs = append(c.msgs, msg)
	c.mu.Unlock()
}

func (c *msgCollector) messages() []tea.Msg {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]tea.Msg, len(c.msgs))
	copy(out, c.msgs)
	return out
}

func (c *msgCollector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.msgs)
}

// ---------------------------------------------------------------------------
// newTicketPoller
// ---------------------------------------------------------------------------

func TestNewTicketPoller_SetsFields(t *testing.T) {
	collector := &msgCollector{}
	p := newTicketPoller(collector.send, 5*time.Second)
	if p == nil {
		t.Fatal("expected non-nil poller")
	}
	if p.interval != 5*time.Second {
		t.Errorf("interval = %v, want 5s", p.interval)
	}
	if len(p.services) != 0 {
		t.Errorf("services should be empty, got %d", len(p.services))
	}
	if len(p.lastUpdate) != 0 {
		t.Errorf("lastUpdate should be empty, got %d", len(p.lastUpdate))
	}
}

// ---------------------------------------------------------------------------
// addService
// ---------------------------------------------------------------------------

func TestTicketPoller_AddService_NilPoller(t *testing.T) {
	var p *ticketPoller
	// Should not panic
	p.addService("/test", nil)
}

func TestTicketPoller_AddService_NilService(t *testing.T) {
	p := newTicketPoller(nil, time.Second)
	// Should not panic and should not add nil service
	p.addService("/test", nil)
	if len(p.services) != 0 {
		t.Fatal("should not add nil service")
	}
}

func TestTicketPoller_AddService_RegistersService(t *testing.T) {
	p := newTicketPoller(nil, time.Second)
	store := &mockTicketStore{}
	reg, _ := discovery.NewRegistry(t.TempDir())
	svc := tickets.NewTicketService(store, reg, tickets.NewRenderer())

	p.addService("/project", svc)
	if len(p.services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(p.services))
	}
	if p.services["/project"] != svc {
		t.Fatal("service not registered for /project")
	}
}

func TestTicketPoller_AddService_Overwrites(t *testing.T) {
	p := newTicketPoller(nil, time.Second)
	store := &mockTicketStore{}
	reg, _ := discovery.NewRegistry(t.TempDir())
	svc1 := tickets.NewTicketService(store, reg, tickets.NewRenderer())
	svc2 := tickets.NewTicketService(store, reg, tickets.NewRenderer())

	p.addService("/project", svc1)
	p.addService("/project", svc2)
	if len(p.services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(p.services))
	}
	if p.services["/project"] != svc2 {
		t.Fatal("expected svc2 to overwrite svc1")
	}
}

// ---------------------------------------------------------------------------
// pollOnce — change detection
// ---------------------------------------------------------------------------

func TestTicketPoller_PollOnce_SendsOnTimestampChange(t *testing.T) {
	collector := &msgCollector{}
	p := newTicketPoller(collector.send, time.Second)

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

	// First poll: should detect the initial timestamp and send TicketsLoadedMsg.
	p.pollOnce(context.Background())

	msgs := collector.messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message on first poll, got %d", len(msgs))
	}
	tlm, ok := msgs[0].(messages.TicketsLoadedMsg)
	if !ok {
		t.Fatalf("expected TicketsLoadedMsg, got %T", msgs[0])
	}
	if tlm.ProjectPath != "/project" {
		t.Errorf("ProjectPath = %q, want /project", tlm.ProjectPath)
	}
	if len(tlm.Tickets) != 1 {
		t.Errorf("expected 1 ticket, got %d", len(tlm.Tickets))
	}
}

func TestTicketPoller_PollOnce_NoSendWhenUnchanged(t *testing.T) {
	collector := &msgCollector{}
	p := newTicketPoller(collector.send, time.Second)

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

	// First poll establishes baseline.
	p.pollOnce(context.Background())
	if collector.count() != 1 {
		t.Fatalf("expected 1 message after first poll, got %d", collector.count())
	}

	// Second poll with same timestamp should not send.
	p.pollOnce(context.Background())
	if collector.count() != 1 {
		t.Fatalf("expected no new message on unchanged timestamp, got %d total", collector.count())
	}
}

func TestTicketPoller_PollOnce_SendsOnSecondChange(t *testing.T) {
	collector := &msgCollector{}
	p := newTicketPoller(collector.send, time.Second)

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

	// First poll.
	p.pollOnce(context.Background())
	if collector.count() != 1 {
		t.Fatalf("expected 1 message after first poll, got %d", collector.count())
	}

	// Advance timestamp.
	later := now.Add(1 * time.Minute)
	store.setLatest(later)
	store.setTickets([]tickets.Ticket{
		{ID: "bmx-1", Title: "Updated", Status: "in_progress", UpdatedAt: later},
		{ID: "bmx-2", Title: "New ticket", Status: "open", UpdatedAt: later},
	})

	// Second poll should detect the change.
	p.pollOnce(context.Background())
	if collector.count() != 2 {
		t.Fatalf("expected 2 messages after change, got %d", collector.count())
	}

	tlm, ok := collector.messages()[1].(messages.TicketsLoadedMsg)
	if !ok {
		t.Fatalf("expected TicketsLoadedMsg, got %T", collector.messages()[1])
	}
	if len(tlm.Tickets) != 2 {
		t.Errorf("expected 2 tickets after update, got %d", len(tlm.Tickets))
	}
}

func TestTicketPoller_PollOnce_SkipsZeroTimestamp(t *testing.T) {
	collector := &msgCollector{}
	p := newTicketPoller(collector.send, time.Second)

	store := &mockTicketStore{latest: time.Time{}} // zero time
	reg, _ := discovery.NewRegistry(t.TempDir())
	svc := tickets.NewTicketService(store, reg, tickets.NewRenderer())
	p.addService("/project", svc)

	p.pollOnce(context.Background())
	if collector.count() != 0 {
		t.Fatalf("expected no message for zero timestamp, got %d", collector.count())
	}
}

func TestTicketPoller_PollOnce_SkipsOnError(t *testing.T) {
	collector := &msgCollector{}
	p := newTicketPoller(collector.send, time.Second)

	store := &mockTicketStore{
		latestFn: func(_ context.Context) (time.Time, error) {
			return time.Time{}, context.DeadlineExceeded
		},
	}
	reg, _ := discovery.NewRegistry(t.TempDir())
	svc := tickets.NewTicketService(store, reg, tickets.NewRenderer())
	p.addService("/project", svc)

	p.pollOnce(context.Background())
	if collector.count() != 0 {
		t.Fatalf("expected no message on LatestUpdate error, got %d", collector.count())
	}
}

func TestTicketPoller_PollOnce_SkipsNilService(t *testing.T) {
	collector := &msgCollector{}
	p := newTicketPoller(collector.send, time.Second)

	// Manually insert a nil service entry.
	p.mu.Lock()
	p.services["/gone"] = nil
	p.mu.Unlock()

	p.pollOnce(context.Background())
	if collector.count() != 0 {
		t.Fatalf("expected no message for nil service, got %d", collector.count())
	}
}

func TestTicketPoller_PollOnce_MultipleProjects(t *testing.T) {
	collector := &msgCollector{}
	p := newTicketPoller(collector.send, time.Second)

	now := time.Now()

	// Project A
	storeA := &mockTicketStore{
		tickets: []tickets.Ticket{{ID: "a-1", Title: "A", Status: "open", UpdatedAt: now}},
		latest:  now,
	}
	regA, _ := discovery.NewRegistry(t.TempDir())
	svcA := tickets.NewTicketService(storeA, regA, tickets.NewRenderer())
	p.addService("/a", svcA)

	// Project B
	storeB := &mockTicketStore{
		tickets: []tickets.Ticket{{ID: "b-1", Title: "B", Status: "open", UpdatedAt: now}},
		latest:  now,
	}
	regB, _ := discovery.NewRegistry(t.TempDir())
	svcB := tickets.NewTicketService(storeB, regB, tickets.NewRenderer())
	p.addService("/b", svcB)

	// First poll should send for both projects.
	p.pollOnce(context.Background())
	if collector.count() != 2 {
		t.Fatalf("expected 2 messages for 2 projects, got %d", collector.count())
	}

	// Update only project A.
	storeA.setLatest(now.Add(1 * time.Minute))
	p.pollOnce(context.Background())
	if collector.count() != 3 {
		t.Fatalf("expected 3 total messages after updating only project A, got %d", collector.count())
	}
}

func TestTicketPoller_PollOnce_NilPoller(t *testing.T) {
	var p *ticketPoller
	// Should not panic.
	p.pollOnce(context.Background())
}
