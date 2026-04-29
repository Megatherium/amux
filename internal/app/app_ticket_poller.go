package app

import (
	"context"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/logging"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/tickets"
)

// ticketPoller periodically checks all configured ticket services for changes
// and sends TicketsLoadedMsg when a project's ticket data has been updated.
//
// It runs as a supervisor worker (not a tea.Cmd) so that Dolt MySQL queries
// execute off the main rendering thread and never cause UI stutter.
type ticketPoller struct {
	// mu protects the services map for concurrent access from the UI goroutine
	// (which writes new entries) and the poller goroutine (which reads entries).
	mu       sync.RWMutex
	services map[string]*tickets.TicketService

	// lastUpdate tracks the most recent LatestUpdate timestamp per project.
	// A change in this value triggers a ticket reload for that project.
	lastUpdate map[string]time.Time

	// send delivers a message into the Bubble Tea event loop.
	// In production this is App.enqueueExternalMsg.
	send func(tea.Msg)

	// interval controls how often the poller checks for changes.
	interval time.Duration
}

// newTicketPoller creates a poller that checks every interval and delivers
// TicketsLoadedMsg via the provided send callback.
func newTicketPoller(send func(tea.Msg), interval time.Duration) *ticketPoller {
	return &ticketPoller{
		services:   make(map[string]*tickets.TicketService),
		lastUpdate: make(map[string]time.Time),
		send:       send,
		interval:   interval,
	}
}

// addService registers a ticket service for a project path.
// Safe to call from any goroutine.
func (p *ticketPoller) addService(projectPath string, svc *tickets.TicketService) {
	if p == nil || svc == nil {
		return
	}
	p.mu.Lock()
	p.services[projectPath] = svc
	p.mu.Unlock()
}

// removeService unregisters a project from polling. After this call the poller
// will no longer check LatestUpdate or reload tickets for projectPath.
// Safe to call from any goroutine.
func (p *ticketPoller) removeService(projectPath string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	delete(p.services, projectPath)
	delete(p.lastUpdate, projectPath)
	p.mu.Unlock()
}

// run is the supervisor worker function. It loops until ctx is canceled,
// checking each registered service for timestamp changes.
func (p *ticketPoller) run(ctx context.Context) error {
	// Use a ticker that we can override in tests via the interval field.
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			p.pollOnce(ctx)
		}
	}
}

// pollOnce checks all registered services and reloads tickets for any project
// whose LatestUpdate timestamp has changed since the last check.
func (p *ticketPoller) pollOnce(ctx context.Context) {
	if p == nil {
		return
	}

	// Snapshot the service keys under a read lock so we don't hold the lock
	// during network I/O.
	p.mu.RLock()
	paths := make([]string, 0, len(p.services))
	for path := range p.services {
		paths = append(paths, path)
	}
	p.mu.RUnlock()

	for _, path := range paths {
		p.mu.RLock()
		svc, ok := p.services[path]
		p.mu.RUnlock()
		if !ok || svc == nil {
			continue
		}

		latest, err := svc.LatestUpdate(ctx)
		if err != nil {
			// Transient errors are expected (Dolt server restart, network blip).
			// The supervisor will restart the worker if it returns a fatal error,
			// but here we just skip this project and try again next tick.
			logging.Debug("ticket poller: LatestUpdate failed for %s: %v", path, err)
			continue
		}

		p.mu.RLock()
		prev := p.lastUpdate[path]
		p.mu.RUnlock()

		// A zero-valued latest means the store has no tickets yet; skip.
		if latest.IsZero() {
			continue
		}

		// Only reload if the timestamp actually changed.
		if !prev.IsZero() && !latest.After(prev) {
			continue
		}

		// Timestamp changed (or first poll with data) — reload tickets.
		t, loadErr := loadOpenAndInProgress(ctx, svc, path, 20)
		if loadErr != nil {
			logging.Debug("ticket poller: reload failed for %s: %v", path, loadErr)
			continue
		}

		p.mu.Lock()
		p.lastUpdate[path] = latest
		p.mu.Unlock()

		if p.send != nil {
			p.send(messages.TicketsLoadedMsg{
				ProjectPath: path,
				Tickets:     t,
			})
		}
	}
}
