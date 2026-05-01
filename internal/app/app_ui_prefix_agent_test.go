package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/app/orchestrator"
	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/messages"
)

func TestPrefixCommand_tb_MatchesNewAgentTab(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)

	app.oc().Prefix.Active = true
	app.oc().Prefix.Sequence = []string{"t"}
	status, _ := app.handlePrefixCommand(tea.KeyPressMsg{Code: 'b', Text: "b"})
	if status != orchestrator.PrefixMatchComplete {
		t.Fatalf("prefix 't b' should complete, got %v", status)
	}
}

func TestPrefixCommand_ta_MatchesNewAgentTabDirect(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)

	app.oc().Prefix.Active = true
	app.oc().Prefix.Sequence = []string{"t"}
	status, _ := app.handlePrefixCommand(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if status != orchestrator.PrefixMatchComplete {
		t.Fatalf("prefix 't a' should complete, got %v", status)
	}
}

func TestRunPrefixAction_NewAgentTab(t *testing.T) {
	app, ws, _ := newPrefixTestApp(t)
	app.activeWorkspace = ws
	app.activeProject = &data.Project{Name: "p", Path: "/r"}
	app.tmuxAvailable = true

	cmd := app.runPrefixAction("new_agent_tab")
	if cmd == nil {
		t.Fatal("expected non-nil cmd for new_agent_tab")
	}
	msg := cmd()
	if _, ok := msg.(messages.ShowSelectTicketDialog); !ok {
		t.Fatalf("expected ShowSelectTicketDialog, got %T", msg)
	}
}

func TestRunPrefixAction_NewAgentTabDirect(t *testing.T) {
	app, ws, _ := newPrefixTestApp(t)
	app.activeWorkspace = ws
	app.activeProject = &data.Project{Name: "p", Path: "/r"}
	app.tmuxAvailable = true

	cmd := app.runPrefixAction("new_agent_tab_direct")
	if cmd == nil {
		t.Fatal("expected non-nil cmd for new_agent_tab_direct")
	}
	msg := cmd()
	if _, ok := msg.(messages.ShowSelectAssistantDialog); !ok {
		t.Fatalf("expected ShowSelectAssistantDialog, got %T", msg)
	}
}

func TestRunPrefixAction_NewAgentTabDirectRequiresWorkspace(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)
	app.activeWorkspace = nil
	app.activeProject = nil

	cmd := app.runPrefixAction("new_agent_tab_direct")
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(messages.ShowSelectAssistantDialog); ok {
			t.Fatal("should not return ShowSelectAssistantDialog without workspace")
		}
	}
}
