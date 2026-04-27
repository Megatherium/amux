package app

import (
	"testing"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/tmux"
	"github.com/andyrewlee/amux/internal/ui/sidebar"
)

func TestHandleTmuxSidebarDiscoverResultCreatesTerminalWhenEmpty(t *testing.T) {
	app := &App{ui: &UICompositor{}}
	ws := data.NewWorkspace("ws", "main", "main", "/repo/ws", "/repo/ws")
	app.projects = []data.Project{{Name: "p", Path: ws.Repo, Workspaces: []data.Workspace{*ws}}}
	app.ui.sidebarTerminal = sidebar.NewTerminalModel()
	app.activeWorkspace = ws

	cmds := app.handleTmuxSidebarDiscoverResult(tmuxSidebarDiscoverResult{
		WorkspaceID: string(ws.ID()),
		Sessions:    nil,
	})
	if len(cmds) != 1 {
		t.Fatalf("expected a command to create a terminal, got %d", len(cmds))
	}
}

func TestBuildSidebarSessionAttachInfosIncludesSessionsAcrossInstances(t *testing.T) {
	sessions := []sidebarSessionInfo{
		{name: "a1", instanceID: "inst-a", createdAt: 100},
		{name: "b1", instanceID: "inst-b", createdAt: 200},
		{name: "c1", instanceID: "inst-c", createdAt: 300},
	}
	out := buildSidebarSessionAttachInfos(sessions)
	if len(out) != 3 {
		t.Fatalf("expected 3 sessions across all instances, got %d", len(out))
	}
	names := make(map[string]bool)
	for _, s := range out {
		names[s.Name] = true
	}
	for _, expected := range []string{"a1", "b1", "c1"} {
		if !names[expected] {
			t.Fatalf("expected session %s in output", expected)
		}
	}
}

func TestBuildSidebarSessionAttachInfosHandlesEmpty(t *testing.T) {
	out := buildSidebarSessionAttachInfos(nil)
	if len(out) != 0 {
		t.Fatalf("expected empty output for nil input, got %d", len(out))
	}

	out = buildSidebarSessionAttachInfos([]sidebarSessionInfo{})
	if len(out) != 0 {
		t.Fatalf("expected empty output for empty input, got %d", len(out))
	}
}

func TestBuildSidebarSessionAttachInfosOrdersByCreatedAt(t *testing.T) {
	sessions := []sidebarSessionInfo{
		{name: "s3", instanceID: "a", createdAt: 300},
		{name: "s1", instanceID: "b", createdAt: 100},
		{name: "s2", instanceID: "c", createdAt: 200},
	}
	out := buildSidebarSessionAttachInfos(sessions)
	if len(out) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(out))
	}
	expected := []string{"s1", "s2", "s3"}
	for i, name := range expected {
		if out[i].Name != name {
			t.Fatalf("position %d: expected %s, got %s", i, name, out[i].Name)
		}
	}
}

func TestDiscoverSidebarAttachFlags(t *testing.T) {
	sessions := []sidebarSessionInfo{
		{name: "a1", instanceID: "a", createdAt: 100, hasClients: true},
		{name: "a2", instanceID: "b", createdAt: 101, hasClients: false},
		{name: "b1", instanceID: "c", createdAt: 200, hasClients: false},
	}
	out := buildSidebarSessionAttachInfos(sessions)
	if len(out) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(out))
	}
	for _, sess := range out {
		if !sess.Attach {
			t.Fatalf("expected %s to have Attach=true", sess.Name)
		}
		switch sess.Name {
		case "a1":
			if sess.DetachExisting {
				t.Fatal("expected a1 to attach without detaching (has clients)")
			}
		case "a2", "b1":
			if !sess.DetachExisting {
				t.Fatalf("expected %s to attach with detach (no clients)", sess.Name)
			}
		}
	}
}

func TestDiscoverWorkspaceTabsRecoversFullTicketMetadata(t *testing.T) {
	ws := data.NewWorkspace("ws", "main", "main", "/repo/ws", "/repo/ws")
	app := &App{
		tmuxAvailable: true,
		tmuxService: sessionsWithTagsStubTmuxOps{
			rows: []tmux.SessionTagValues{
				{
					Name: "amux-session-1",
					Tags: map[string]string{
						"@amux_assistant":    "claude",
						"@amux_created_at":   "1700000000",
						"@amux_ticket_id":    "bmx-dg8",
						"@amux_ticket_title": "Extend tmux discovery to recover ticket metadata",
						"@amux_model":        "claude-sonnet-4-20250514",
						"@amux_agent_mode":   "code",
					},
				},
			},
		},
	}

	cmd := app.discoverWorkspaceTabsFromTmux(ws)
	if cmd == nil {
		t.Fatal("expected a non-nil command")
	}
	msg := cmd()
	result, ok := msg.(tmuxTabsDiscoverResult)
	if !ok {
		t.Fatalf("expected tmuxTabsDiscoverResult, got %T", msg)
	}
	if len(result.Tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(result.Tabs))
	}
	tab := result.Tabs[0]
	if tab.TicketID != "bmx-dg8" {
		t.Errorf("TicketID: got %q, want %q", tab.TicketID, "bmx-dg8")
	}
	if tab.TicketTitle != "Extend tmux discovery to recover ticket metadata" {
		t.Errorf("TicketTitle: got %q, want %q", tab.TicketTitle, "Extend tmux discovery to recover ticket metadata")
	}
	if tab.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model: got %q, want %q", tab.Model, "claude-sonnet-4-20250514")
	}
	if tab.Agent != "code" {
		t.Errorf("Agent: got %q, want %q", tab.Agent, "code")
	}
}

func TestDiscoverWorkspaceTabsNoTicketMetadata(t *testing.T) {
	ws := data.NewWorkspace("ws", "main", "main", "/repo/ws", "/repo/ws")
	app := &App{
		tmuxAvailable: true,
		tmuxService: sessionsWithTagsStubTmuxOps{
			rows: []tmux.SessionTagValues{
				{
					Name: "amux-session-2",
					Tags: map[string]string{
						"@amux_assistant":  "claude",
						"@amux_created_at": "1700000000",
					},
				},
			},
		},
	}

	cmd := app.discoverWorkspaceTabsFromTmux(ws)
	if cmd == nil {
		t.Fatal("expected a non-nil command")
	}
	msg := cmd()
	result, ok := msg.(tmuxTabsDiscoverResult)
	if !ok {
		t.Fatalf("expected tmuxTabsDiscoverResult, got %T", msg)
	}
	if len(result.Tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(result.Tabs))
	}
	tab := result.Tabs[0]
	if tab.TicketID != "" {
		t.Errorf("TicketID: got %q, want empty", tab.TicketID)
	}
	if tab.TicketTitle != "" {
		t.Errorf("TicketTitle: got %q, want empty", tab.TicketTitle)
	}
	if tab.Model != "" {
		t.Errorf("Model: got %q, want empty", tab.Model)
	}
	if tab.Agent != "" {
		t.Errorf("Agent: got %q, want empty", tab.Agent)
	}
}

func TestDiscoverWorkspaceTabsPartialTicketMetadata(t *testing.T) {
	ws := data.NewWorkspace("ws", "main", "main", "/repo/ws", "/repo/ws")
	app := &App{
		tmuxAvailable: true,
		tmuxService: sessionsWithTagsStubTmuxOps{
			rows: []tmux.SessionTagValues{
				{
					Name: "amux-session-3",
					Tags: map[string]string{
						"@amux_assistant":  "claude",
						"@amux_created_at": "1700000000",
						"@amux_ticket_id":  "bmx-abc",
						"@amux_model":      "gpt-4",
					},
				},
			},
		},
	}

	cmd := app.discoverWorkspaceTabsFromTmux(ws)
	msg := cmd()
	result, ok := msg.(tmuxTabsDiscoverResult)
	if !ok {
		t.Fatalf("expected tmuxTabsDiscoverResult, got %T", msg)
	}
	tab := result.Tabs[0]
	if tab.TicketID != "bmx-abc" {
		t.Errorf("TicketID: got %q, want %q", tab.TicketID, "bmx-abc")
	}
	if tab.TicketTitle != "" {
		t.Errorf("TicketTitle: got %q, want empty (not set)", tab.TicketTitle)
	}
	if tab.Model != "gpt-4" {
		t.Errorf("Model: got %q, want %q", tab.Model, "gpt-4")
	}
	if tab.Agent != "" {
		t.Errorf("Agent: got %q, want empty (not set)", tab.Agent)
	}
}

func TestHandleTmuxTabsDiscoverResultPreservesTicketMetadata(t *testing.T) {
	ws := data.NewWorkspace("ws", "main", "main", "/repo/ws", "/repo/ws")
	wsID := string(ws.ID())
	app := &App{
		projects: []data.Project{{Name: "p", Path: ws.Repo, Workspaces: []data.Workspace{*ws}}},
	}

	cmds := app.handleTmuxTabsDiscoverResult(tmuxTabsDiscoverResult{
		WorkspaceID: wsID,
		Tabs: []data.TabInfo{
			{
				Assistant:   "claude",
				Name:        "claude",
				SessionName: "amux-session-ticket",
				Status:      "running",
				CreatedAt:   1700000000,
				TicketID:    "bmx-dg8",
				TicketTitle: "Extend tmux discovery",
				Model:       "claude-sonnet-4",
				Agent:       "code",
			},
		},
	})

	if len(cmds) == 0 {
		t.Fatal("expected at least one command (persist)")
	}

	found := app.findWorkspaceByID(wsID)
	if found == nil {
		t.Fatal("workspace not found after handler")
	}
	if len(found.OpenTabs) != 1 {
		t.Fatalf("expected 1 open tab, got %d", len(found.OpenTabs))
	}
	tab := found.OpenTabs[0]
	if tab.TicketID != "bmx-dg8" {
		t.Errorf("TicketID: got %q, want %q", tab.TicketID, "bmx-dg8")
	}
	if tab.TicketTitle != "Extend tmux discovery" {
		t.Errorf("TicketTitle: got %q, want %q", tab.TicketTitle, "Extend tmux discovery")
	}
	if tab.Model != "claude-sonnet-4" {
		t.Errorf("Model: got %q, want %q", tab.Model, "claude-sonnet-4")
	}
	if tab.Agent != "code" {
		t.Errorf("Agent: got %q, want %q", tab.Agent, "code")
	}
}

func TestDiscoverWorkspaceTabsMultipleSessionsWithMixedTicketData(t *testing.T) {
	ws := data.NewWorkspace("ws", "main", "main", "/repo/ws", "/repo/ws")
	app := &App{
		tmuxAvailable: true,
		tmuxService: sessionsWithTagsStubTmuxOps{
			rows: []tmux.SessionTagValues{
				{
					Name: "amux-session-10",
					Tags: map[string]string{
						"@amux_assistant":    "claude",
						"@amux_created_at":   "1700000001",
						"@amux_ticket_id":    "bmx-aaa",
						"@amux_ticket_title": "First ticket",
						"@amux_model":        "model-a",
						"@amux_agent_mode":   "code",
					},
				},
				{
					Name: "amux-session-11",
					Tags: map[string]string{
						"@amux_assistant":  "claude",
						"@amux_created_at": "1700000002",
					},
				},
				{
					Name: "amux-session-12",
					Tags: map[string]string{
						"@amux_assistant":  "codex",
						"@amux_created_at": "1700000003",
						"@amux_ticket_id":  "bmx-ccc",
						"@amux_agent_mode": "plan",
					},
				},
			},
		},
	}

	cmd := app.discoverWorkspaceTabsFromTmux(ws)
	msg := cmd()
	result, ok := msg.(tmuxTabsDiscoverResult)
	if !ok {
		t.Fatalf("expected tmuxTabsDiscoverResult, got %T", msg)
	}
	if len(result.Tabs) != 3 {
		t.Fatalf("expected 3 tabs, got %d", len(result.Tabs))
	}

	tab0 := result.Tabs[0]
	if tab0.TicketID != "bmx-aaa" || tab0.TicketTitle != "First ticket" || tab0.Agent != "code" {
		t.Errorf("tab0 ticket metadata mismatch: %+v", tab0)
	}

	tab1 := result.Tabs[1]
	if tab1.TicketID != "" || tab1.Agent != "" {
		t.Errorf("tab1 should have no ticket metadata: %+v", tab1)
	}

	tab2 := result.Tabs[2]
	if tab2.TicketID != "bmx-ccc" || tab2.Agent != "plan" || tab2.Model != "" {
		t.Errorf("tab2 ticket metadata mismatch: %+v", tab2)
	}
}
