package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWorkspaceStoreAppendOpenTab(t *testing.T) {
	root := t.TempDir()
	store := NewWorkspaceStore(root)

	ws := NewWorkspace("ws-a", "main", "origin/main", "/repo", "/repo/ws-a")
	if err := store.Save(ws); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	tab := TabInfo{
		Assistant:   "claude",
		Name:        "claude",
		SessionName: "session-a",
		Status:      "running",
		CreatedAt:   time.Now().Unix(),
	}
	if err := store.AppendOpenTab(ws.ID(), tab); err != nil {
		t.Fatalf("AppendOpenTab() error = %v", err)
	}

	loaded, err := store.Load(ws.ID())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.OpenTabs) != 1 {
		t.Fatalf("open tabs = %d, want 1", len(loaded.OpenTabs))
	}
	if loaded.OpenTabs[0].SessionName != "session-a" {
		t.Fatalf("session_name = %q, want %q", loaded.OpenTabs[0].SessionName, "session-a")
	}
}

func TestWorkspaceStoreAppendOpenTabDedupesSessionName(t *testing.T) {
	root := t.TempDir()
	store := NewWorkspaceStore(root)

	ws := NewWorkspace("ws-a", "main", "origin/main", "/repo", "/repo/ws-a")
	if err := store.Save(ws); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	tab := TabInfo{
		Assistant:   "claude",
		Name:        "claude",
		SessionName: "session-a",
		Status:      "running",
		CreatedAt:   time.Now().Unix(),
	}
	if err := store.AppendOpenTab(ws.ID(), tab); err != nil {
		t.Fatalf("AppendOpenTab(first) error = %v", err)
	}
	if err := store.AppendOpenTab(ws.ID(), tab); err != nil {
		t.Fatalf("AppendOpenTab(second) error = %v", err)
	}

	loaded, err := store.Load(ws.ID())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.OpenTabs) != 1 {
		t.Fatalf("open tabs = %d, want 1", len(loaded.OpenTabs))
	}
}

func TestWorkspaceStoreAppendOpenTabConcurrentWriters(t *testing.T) {
	root := t.TempDir()
	store := NewWorkspaceStore(root)

	ws := NewWorkspace("ws-a", "main", "origin/main", "/repo", "/repo/ws-a")
	if err := store.Save(ws); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	tabs := []TabInfo{
		{Assistant: "claude", Name: "claude", SessionName: "session-a", Status: "running", CreatedAt: time.Now().Unix()},
		{Assistant: "codex", Name: "codex", SessionName: "session-b", Status: "running", CreatedAt: time.Now().Unix()},
	}

	var wg sync.WaitGroup
	wg.Add(len(tabs))
	for i := range tabs {
		tab := tabs[i]
		go func() {
			defer wg.Done()
			if err := store.AppendOpenTab(ws.ID(), tab); err != nil {
				t.Errorf("AppendOpenTab(%s) error = %v", tab.SessionName, err)
			}
		}()
	}
	wg.Wait()

	loaded, err := store.Load(ws.ID())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.OpenTabs) != 2 {
		t.Fatalf("open tabs = %d, want 2", len(loaded.OpenTabs))
	}
}

func TestTabInfoTicketFieldsJSONRoundTrip(t *testing.T) {
	tab := TabInfo{
		Assistant:   "claude",
		Name:        "claude",
		SessionName: "session-ticket",
		Status:      "running",
		CreatedAt:   time.Now().Unix(),
		TicketID:    "bb-42",
		TicketTitle: "Fix the frobnicator",
		Model:       "anthropic/claude/claude-sonnet-4",
		Agent:       "auto-approve",
	}

	raw, err := json.Marshal(tab)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded TabInfo
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.TicketID != tab.TicketID {
		t.Errorf("TicketID = %q, want %q", decoded.TicketID, tab.TicketID)
	}
	if decoded.TicketTitle != tab.TicketTitle {
		t.Errorf("TicketTitle = %q, want %q", decoded.TicketTitle, tab.TicketTitle)
	}
	if decoded.Model != tab.Model {
		t.Errorf("Model = %q, want %q", decoded.Model, tab.Model)
	}
	if decoded.Agent != tab.Agent {
		t.Errorf("Agent = %q, want %q", decoded.Agent, tab.Agent)
	}
}

func TestTabInfoTicketFieldsOmitEmpty(t *testing.T) {
	tab := TabInfo{
		Assistant: "claude",
		Name:      "claude",
	}

	raw, err := json.Marshal(tab)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Verify ticket fields are absent from JSON (not present as empty strings)
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("Unmarshal to map error = %v", err)
	}
	for _, key := range []string{"ticket_id", "ticket_title", "model", "agent"} {
		if _, ok := generic[key]; ok {
			t.Errorf("key %q present in JSON, want omitted for zero-value TabInfo", key)
		}
	}
}

func TestTabInfoTicketFieldsBackwardCompat(t *testing.T) {
	// Simulate loading a workspace.json that was written before ticket fields existed.
	legacyTabJSON := `{
		"assistant": "codex",
		"name": "codex",
		"session_name": "session-old",
		"status": "stopped",
		"created_at": 1712000000
	}`

	var tab TabInfo
	if err := json.Unmarshal([]byte(legacyTabJSON), &tab); err != nil {
		t.Fatalf("Unmarshal legacy JSON error = %v", err)
	}

	if tab.Assistant != "codex" {
		t.Errorf("Assistant = %q, want %q", tab.Assistant, "codex")
	}
	if tab.TicketID != "" {
		t.Errorf("TicketID = %q, want empty", tab.TicketID)
	}
	if tab.TicketTitle != "" {
		t.Errorf("TicketTitle = %q, want empty", tab.TicketTitle)
	}
	if tab.Model != "" {
		t.Errorf("Model = %q, want empty", tab.Model)
	}
	if tab.Agent != "" {
		t.Errorf("Agent = %q, want empty", tab.Agent)
	}
}

func TestWorkspaceStoreAppendOpenTabWithTicketMetadata(t *testing.T) {
	root := t.TempDir()
	store := NewWorkspaceStore(root)

	ws := NewWorkspace("ws-ticket", "main", "origin/main", "/repo", "/repo/ws-ticket")
	if err := store.Save(ws); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	tab := TabInfo{
		Assistant:   "claude",
		Name:        "claude",
		SessionName: "session-ticket",
		Status:      "running",
		CreatedAt:   time.Now().Unix(),
		TicketID:    "bb-99",
		TicketTitle: "Add widget support",
		Model:       "anthropic/claude/claude-sonnet-4",
		Agent:       "auto-approve",
	}
	if err := store.AppendOpenTab(ws.ID(), tab); err != nil {
		t.Fatalf("AppendOpenTab() error = %v", err)
	}

	loaded, err := store.Load(ws.ID())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.OpenTabs) != 1 {
		t.Fatalf("open tabs = %d, want 1", len(loaded.OpenTabs))
	}

	got := loaded.OpenTabs[0]
	if got.TicketID != tab.TicketID {
		t.Errorf("TicketID = %q, want %q", got.TicketID, tab.TicketID)
	}
	if got.TicketTitle != tab.TicketTitle {
		t.Errorf("TicketTitle = %q, want %q", got.TicketTitle, tab.TicketTitle)
	}
	if got.Model != tab.Model {
		t.Errorf("Model = %q, want %q", got.Model, tab.Model)
	}
	if got.Agent != tab.Agent {
		t.Errorf("Agent = %q, want %q", got.Agent, tab.Agent)
	}
}

func TestWorkspaceStoreBackwardCompatTabsNoTicketFields(t *testing.T) {
	// Write a workspace.json file that has open_tabs without ticket fields,
	// simulating a file written by an older version of amux.
	root := t.TempDir()
	store := NewWorkspaceStore(root)

	ws := NewWorkspace("ws-old", "main", "origin/main", "/repo", "/repo/ws-old")
	id := ws.ID()
	dir := filepath.Join(root, string(id))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	legacyWorkspace := `{
		"name": "ws-old",
		"branch": "main",
		"base": "origin/main",
		"repo": "/repo",
		"root": "/repo/ws-old",
		"assistant": "claude",
		"open_tabs": [
			{
				"assistant": "claude",
				"name": "claude",
				"session_name": "session-old",
				"status": "running",
				"created_at": 1712000000
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(dir, "workspace.json"), []byte(legacyWorkspace), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	loaded, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.OpenTabs) != 1 {
		t.Fatalf("open tabs = %d, want 1", len(loaded.OpenTabs))
	}

	tab := loaded.OpenTabs[0]
	if tab.Assistant != "claude" {
		t.Errorf("Assistant = %q, want %q", tab.Assistant, "claude")
	}
	if tab.SessionName != "session-old" {
		t.Errorf("SessionName = %q, want %q", tab.SessionName, "session-old")
	}
	// Ticket fields must be zero-valued, not garbage or error
	if tab.TicketID != "" {
		t.Errorf("TicketID = %q, want empty", tab.TicketID)
	}
	if tab.TicketTitle != "" {
		t.Errorf("TicketTitle = %q, want empty", tab.TicketTitle)
	}
	if tab.Model != "" {
		t.Errorf("Model = %q, want empty", tab.Model)
	}
	if tab.Agent != "" {
		t.Errorf("Agent = %q, want empty", tab.Agent)
	}
}

func TestWorkspaceStoreMixedTabsWithAndWithoutTicketMetadata(t *testing.T) {
	root := t.TempDir()
	store := NewWorkspaceStore(root)

	ws := NewWorkspace("ws-mix", "main", "origin/main", "/repo", "/repo/ws-mix")
	if err := store.Save(ws); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	plainTab := TabInfo{
		Assistant:   "claude",
		Name:        "claude",
		SessionName: "session-plain",
		Status:      "running",
		CreatedAt:   time.Now().Unix(),
	}
	ticketTab := TabInfo{
		Assistant:   "opencode",
		Name:        "opencode",
		SessionName: "session-ticket",
		Status:      "running",
		CreatedAt:   time.Now().Unix(),
		TicketID:    "bb-7",
		TicketTitle: "Implement feature X",
		Model:       "anthropic/claude/claude-sonnet-4",
		Agent:       "coder",
	}
	if err := store.AppendOpenTab(ws.ID(), plainTab); err != nil {
		t.Fatalf("AppendOpenTab(plain) error = %v", err)
	}
	if err := store.AppendOpenTab(ws.ID(), ticketTab); err != nil {
		t.Fatalf("AppendOpenTab(ticket) error = %v", err)
	}

	loaded, err := store.Load(ws.ID())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.OpenTabs) != 2 {
		t.Fatalf("open tabs = %d, want 2", len(loaded.OpenTabs))
	}

	// First tab should have no ticket metadata
	if loaded.OpenTabs[0].TicketID != "" {
		t.Errorf("plain tab TicketID = %q, want empty", loaded.OpenTabs[0].TicketID)
	}
	// Second tab should carry full ticket metadata
	got := loaded.OpenTabs[1]
	if got.TicketID != ticketTab.TicketID {
		t.Errorf("ticket tab TicketID = %q, want %q", got.TicketID, ticketTab.TicketID)
	}
	if got.TicketTitle != ticketTab.TicketTitle {
		t.Errorf("ticket tab TicketTitle = %q, want %q", got.TicketTitle, ticketTab.TicketTitle)
	}
	if got.Model != ticketTab.Model {
		t.Errorf("ticket tab Model = %q, want %q", got.Model, ticketTab.Model)
	}
	if got.Agent != ticketTab.Agent {
		t.Errorf("ticket tab Agent = %q, want %q", got.Agent, ticketTab.Agent)
	}
}
