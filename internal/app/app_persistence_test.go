package app

import (
	"errors"
	"os"
	"testing"

	"github.com/andyrewlee/amux/internal/app/workspaces"
	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/ui/center"
	"github.com/andyrewlee/amux/internal/ui/dashboard"
)

func TestPersistAllWorkspacesNowSavesExplicitlyEmptyTabs(t *testing.T) {
	ws := data.NewWorkspace("test-ws", "main", "main", "/repo", "/repo")
	wsID := string(ws.ID())

	storeRoot := t.TempDir()
	store := data.NewWorkspaceStore(storeRoot)

	// Save initial workspace with a tab so we can verify it gets updated to empty
	ws.OpenTabs = []data.TabInfo{{Name: "old-tab", Assistant: "claude"}}
	if err := store.Save(ws); err != nil {
		t.Fatalf("initial save: %v", err)
	}

	c := center.New(nil)
	c.SetWorkspace(ws)
	// Add a tab then close it so the workspace has explicit empty state
	tab := &center.Tab{
		Name:      "agent",
		Assistant: "claude",
		Workspace: ws,
	}
	c.AddTab(tab)
	// Close the tab — tab has no session/agent so close is lightweight
	_ = c.CloseActiveTab()

	// After close: tabs list is empty but workspace state map entry exists
	tabs, _ := c.GetTabsInfoForWorkspace(wsID)
	if len(tabs) != 0 {
		t.Fatalf("expected 0 tabs after close, got %d", len(tabs))
	}
	if !c.HasWorkspaceState(wsID) {
		t.Fatal("expected HasWorkspaceState=true after close")
	}

	svc := workspaces.NewService(nil, store, nil, "")

	// Clear old tabs from in-memory workspace before persist
	ws.OpenTabs = nil
	app := &App{
		ui: &UICompositor{
			center: c,
		},
		workspaceService: svc,
		projects:         []data.Project{{Name: "p", Path: "/repo", Workspaces: []data.Workspace{*ws}}},
		workspaceManager: workspaces.NewManager(),
	}

	app.persistAllWorkspacesNow()

	// Reload from store and verify the workspace was saved with empty tabs
	loaded, err := store.Load(ws.ID())
	if err != nil {
		t.Fatalf("load after persist: %v", err)
	}
	if len(loaded.OpenTabs) != 0 {
		t.Fatalf("expected 0 open tabs after persist, got %d", len(loaded.OpenTabs))
	}
}

func TestPersistAllWorkspacesNowSavesDeleteInFlightWorkspace(t *testing.T) {
	ws := data.NewWorkspace("test-ws", "main", "main", "/repo", "/repo")
	wsID := string(ws.ID())

	storeRoot := t.TempDir()
	store := data.NewWorkspaceStore(storeRoot)

	c := center.New(nil)
	c.SetWorkspace(ws)
	tab := &center.Tab{
		Name:      "agent",
		Assistant: "claude",
		Workspace: ws,
	}
	c.AddTab(tab)

	svc := workspaces.NewService(nil, store, nil, "")
	app := &App{
		ui: &UICompositor{
			center: c,
		},
		workspaceService: svc,
		projects:         []data.Project{{Name: "p", Path: "/repo", Workspaces: []data.Workspace{*ws}}},
		workspaceManager: workspaces.NewManagerWithConfig(workspaces.ManagerConfig{
			DeletingWorkspaceIDs: map[string]bool{wsID: true},
		}),
	}

	app.persistAllWorkspacesNow()

	loaded, err := store.Load(ws.ID())
	if err != nil {
		t.Fatalf("load after persist: %v", err)
	}
	if len(loaded.OpenTabs) == 0 {
		t.Fatal("expected delete-in-flight workspace tabs to be persisted on shutdown")
	}
}

func TestPersistWorkspaceTabsInitializesDirtyMap(t *testing.T) {
	app := &App{
		workspaceManager: nil, // explicitly nil — wm() will lazily init
	}

	cmd := app.persistWorkspaceTabs("ws-123")
	if cmd == nil {
		t.Fatal("expected a debounce command, got nil")
	}
	// wm() lazily initializes when nil, so dirty workspaces will be initialized
	_ = app.wm().DirtyWorkspaceCount()
	if !app.wm().IsWorkspaceDirty("ws-123") {
		t.Fatal("expected ws-123 to be marked dirty")
	}
}

func TestPersistWorkspaceTabsSkipsDeleteInFlightWorkspace(t *testing.T) {
	app := &App{
		workspaceManager: workspaces.NewManagerWithConfig(workspaces.ManagerConfig{
			DeletingWorkspaceIDs: map[string]bool{"ws-123": true},
		}),
	}

	cmd := app.persistWorkspaceTabs("ws-123")
	if cmd != nil {
		t.Fatal("expected no debounce command for deleting workspace")
	}
	if app.wm().IsWorkspaceDirty("ws-123") {
		t.Fatal("did not expect deleting workspace to be marked dirty")
	}
}

func TestHandlePersistDebounceSkipsWhenPersistenceDependenciesMissing(t *testing.T) {
	// nil center
	app := &App{
		ui: &UICompositor{
			center: nil,
		},
		workspaceService: workspaces.NewService(nil, nil, nil, ""),
		workspaceManager: workspaces.NewManagerWithConfig(workspaces.ManagerConfig{
			DirtyWorkspaceIDs: map[string]bool{"ws": true},
		}),
	}
	// Force persist token
	_ = app.wm().NextPersistToken()

	cmd := app.handlePersistDebounce(persistDebounceMsg{token: app.wm().CurrentPersistToken()})
	if cmd != nil {
		t.Fatal("expected nil cmd when center is nil")
	}

	// nil workspaceService
	app2 := &App{
		ui: &UICompositor{
			center: center.New(nil),
		},
		workspaceService: nil,
		workspaceManager: workspaces.NewManagerWithConfig(workspaces.ManagerConfig{
			DirtyWorkspaceIDs: map[string]bool{"ws": true},
		}),
	}
	_ = app2.wm().NextPersistToken()

	cmd2 := app2.handlePersistDebounce(persistDebounceMsg{token: app2.wm().CurrentPersistToken()})
	if cmd2 != nil {
		t.Fatal("expected nil cmd when workspaceService is nil")
	}
}

func TestHandlePersistDebounceSkipsDeleteInFlightWorkspace(t *testing.T) {
	ws := data.NewWorkspace("feature", "feature", "main", "/repo", "/repo/feature")
	wsID := string(ws.ID())

	storeRoot := t.TempDir()
	store := data.NewWorkspaceStore(storeRoot)
	svc := workspaces.NewService(nil, store, nil, "")

	app := &App{
		ui: &UICompositor{
			center: center.New(nil),
		},
		workspaceService: svc,
		projects:         []data.Project{{Name: "repo", Path: "/repo", Workspaces: []data.Workspace{*ws}}},
		workspaceManager: workspaces.NewManagerWithConfig(workspaces.ManagerConfig{
			DirtyWorkspaceIDs:    map[string]bool{wsID: true},
			DeletingWorkspaceIDs: map[string]bool{wsID: true},
		}),
	}

	cmd := app.handlePersistDebounce(persistDebounceMsg{token: 1})
	if cmd != nil {
		t.Fatal("expected nil cmd when only dirty workspace is delete-in-flight")
	}
	if !app.wm().IsWorkspaceDirty(wsID) {
		t.Fatal("expected dirty marker to remain while workspace delete is in-flight")
	}
	if _, err := store.Load(ws.ID()); !os.IsNotExist(err) {
		t.Fatalf("expected workspace metadata to remain absent, err=%v", err)
	}
}

func TestDeleteFailureRequeuesAndDebouncedPersistSavesWorkspace(t *testing.T) {
	ws := data.NewWorkspace("feature", "feature", "main", "/repo", "/repo/feature")
	wsID := string(ws.ID())

	storeRoot := t.TempDir()
	store := data.NewWorkspaceStore(storeRoot)
	svc := workspaces.NewService(nil, store, nil, "")

	c := center.New(nil)
	c.SetWorkspace(ws)
	c.AddTab(&center.Tab{
		Name:      "agent",
		Assistant: "claude",
		Workspace: ws,
	})

	app := &App{
		ui: &UICompositor{
			center:    c,
			dashboard: dashboard.New(),
		},
		workspaceService: svc,
		projects:         []data.Project{{Name: "repo", Path: "/repo", Workspaces: []data.Workspace{*ws}}},
		workspaceManager: workspaces.NewManagerWithConfig(workspaces.ManagerConfig{
			DirtyWorkspaceIDs:    map[string]bool{wsID: true},
			DeletingWorkspaceIDs: map[string]bool{wsID: true},
		}),
	}

	if cmd := app.handlePersistDebounce(persistDebounceMsg{token: 1}); cmd != nil {
		t.Fatal("expected nil cmd while workspace delete is in-flight")
	}
	if !app.wm().IsWorkspaceDirty(wsID) {
		t.Fatal("expected dirty marker to remain while delete is in-flight")
	}

	if cmd := app.handleWorkspaceDeleteFailed(messages.WorkspaceDeleteFailed{
		Workspace: ws,
		Err:       errors.New("delete failed"),
	}); cmd == nil {
		t.Fatal("expected non-nil command on delete failure")
	}
	if app.isWorkspaceDeleteInFlight(wsID) {
		t.Fatal("expected delete-in-flight marker to be cleared on delete failure")
	}

	persistCmd := app.handlePersistDebounce(persistDebounceMsg{token: app.wm().CurrentPersistToken()})
	if persistCmd == nil {
		t.Fatal("expected debounced persistence command after delete failure requeue")
	}
	if msg := persistCmd(); msg != nil {
		t.Fatalf("expected nil tea.Msg from persistence command, got %T", msg)
	}

	loaded, err := store.Load(ws.ID())
	if err != nil {
		t.Fatalf("load after persistence: %v", err)
	}
	if len(loaded.OpenTabs) == 0 {
		t.Fatal("expected workspace tabs to be persisted after delete failure requeue")
	}
	if app.wm().IsWorkspaceDirty(wsID) {
		t.Fatal("expected workspace to be cleared from dirty set after save")
	}
}
