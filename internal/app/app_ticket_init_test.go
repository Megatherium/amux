package app

import (
	"testing"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/discovery"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/tickets"
	"github.com/andyrewlee/amux/internal/tickets/dolt"
)

func TestNew_CreatesModelRegistry(t *testing.T) {
	app, err := New("test", "abc123", "2026-01-01")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if app.modelRegistry == nil {
		t.Fatal("expected modelRegistry to be non-nil")
	}
}

func TestNew_CreatesTicketRenderer(t *testing.T) {
	app, err := New("test", "abc123", "2026-01-01")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if app.ticketRenderer == nil {
		t.Fatal("expected ticketRenderer to be non-nil")
	}
}

func TestNew_NoTicketStoresInitially(t *testing.T) {
	app, err := New("test", "abc123", "2026-01-01")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if app.ticketServices != nil {
		t.Fatal("expected ticketServices to be nil before async init")
	}
	if app.doltStores != nil {
		t.Fatal("expected doltStores to be nil before async init")
	}
}

func TestNew_DiscoveryNotReadyInitially(t *testing.T) {
	app, err := New("test", "abc123", "2026-01-01")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if app.discoveryReady {
		t.Fatal("expected discoveryReady to be false before DiscoveryLoadedMsg")
	}
}

func TestShutdown_NilDoltStores_NoPanic(t *testing.T) {
	app := &App{ui: &UICompositor{}}
	app.Shutdown()
}

func TestShutdown_EmptyDoltStoresMap_NoPanic(t *testing.T) {
	app := &App{doltStores: map[string]*dolt.ServerStore{}}
	app.Shutdown()
}

func TestHandleDiscoveryLoaded_SetsDiscoveryReady(t *testing.T) {
	app := &App{ui: &UICompositor{}}
	cmds := app.handleDiscoveryLoaded(messages.DiscoveryLoadedMsg{})
	if !app.discoveryReady {
		t.Fatal("expected discoveryReady to be true after DiscoveryLoadedMsg")
	}
	if len(cmds) != 0 {
		t.Fatalf("expected no cmds without loaded projects, got %d", len(cmds))
	}
}

func TestHandleDiscoveryLoaded_NoCmdsWithoutBeadsDir(t *testing.T) {
	dir := t.TempDir()
	app := &App{
		projects:       []data.Project{{Path: dir}},
		projectsLoaded: true,
		doltStores:     make(map[string]*dolt.ServerStore),
	}
	cmds := app.handleDiscoveryLoaded(messages.DiscoveryLoadedMsg{})
	if !app.discoveryReady {
		t.Fatal("expected discoveryReady to be true")
	}
	if len(cmds) != 0 {
		t.Fatalf("expected no cmds without .beads/ dir, got %d", len(cmds))
	}
}

func TestInitTicketStoresForLoadedProjects_GatedByDiscoveryReady(t *testing.T) {
	app := &App{
		projects:       []data.Project{{Path: t.TempDir()}},
		projectsLoaded: true,
		doltStores:     make(map[string]*dolt.ServerStore),
	}
	cmds := app.initTicketStoresForLoadedProjects()
	if len(cmds) != 0 {
		t.Fatal("expected no cmds when discovery not ready")
	}
}

func TestInitTicketStoresForLoadedProjects_GatedByProjectsLoaded(t *testing.T) {
	app := &App{
		discoveryReady: true,
		doltStores:     make(map[string]*dolt.ServerStore),
	}
	cmds := app.initTicketStoresForLoadedProjects()
	if len(cmds) != 0 {
		t.Fatal("expected no cmds when projects not loaded")
	}
}

func TestInitTicketStoresForLoadedProjects_SkipsExistingStore(t *testing.T) {
	dir := t.TempDir()
	app := &App{
		discoveryReady: true,
		projectsLoaded: true,
		projects:       []data.Project{{Path: dir}},
		doltStores: map[string]*dolt.ServerStore{
			dir: nil,
		},
	}
	cmds := app.initTicketStoresForLoadedProjects()
	if len(cmds) != 0 {
		t.Fatal("expected no cmds for project with existing store entry")
	}
}

func TestHandleTicketStoreResult_Error(t *testing.T) {
	app := &App{ui: &UICompositor{}}
	result := ticketStoreResult{projectPath: "/test", err: errTestTicketStoreFailed}
	cmds := app.handleTicketStoreResult(result)
	if len(cmds) != 0 {
		t.Fatalf("expected no cmds on error, got %d", len(cmds))
	}
	if app.doltStores != nil {
		t.Fatal("doltStores should remain nil on error")
	}
	if app.ticketServices != nil {
		t.Fatal("ticketServices should remain nil on error")
	}
}

func TestHandleTicketStoreResult_Success_SetsPerProjectFields(t *testing.T) {
	reg, _ := discovery.NewRegistry(t.TempDir())
	app := &App{ui: &UICompositor{}}
	result := ticketStoreResult{
		projectPath: "/test",
		service:     tickets.NewTicketService(nil, reg, tickets.NewRenderer()),
	}
	cmds := app.handleTicketStoreResult(result)
	if len(cmds) != 1 {
		t.Fatalf("expected 1 cmd (loadTicketsForProject) on success, got %d", len(cmds))
	}
	if app.ticketServices["/test"] == nil {
		t.Fatal("ticketServices[\"/test\"] should be set on success")
	}
	if app.doltStores["/test"] != nil {
		t.Fatal("doltStores[\"/test\"] should be nil (not set in result)")
	}
}

func TestHandleTicketStoreResult_Success_NilService(t *testing.T) {
	app := &App{ui: &UICompositor{}}
	result := ticketStoreResult{
		projectPath: "/test",
		service:     nil,
	}
	app.handleTicketStoreResult(result)
	if app.ticketServices["/test"] != nil {
		t.Fatal("ticketServices entry should be nil when result has nil service")
	}
}

func TestLoadDiscoveryRegistry_NilRegistry_NilCmd(t *testing.T) {
	app := &App{modelRegistry: nil}
	cmd := app.loadDiscoveryRegistry()
	if cmd != nil {
		t.Fatal("expected nil cmd when modelRegistry is nil")
	}
}

var errTestTicketStoreFailed = errTest("ticket store init failed")

type errTest string

func (e errTest) Error() string { return string(e) }
