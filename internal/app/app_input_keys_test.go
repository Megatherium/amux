package app

import (
	"reflect"
	"testing"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/tickets"
)

func TestCenterButtonCount_NoWorkspace(t *testing.T) {
	app := &App{
		showWelcome:     false,
		activeWorkspace: nil,
	}
	if count := app.centerButtonCount(); count != 0 {
		t.Errorf("expected 0 buttons, got %d", count)
	}
}

func TestCenterButtonCount_WelcomeScreen(t *testing.T) {
	app := &App{
		showWelcome: true,
	}
	if count := app.centerButtonCount(); count != 2 {
		t.Errorf("expected 2 buttons, got %d", count)
	}
}

func TestCenterButtonCount_ActiveWorkspace_NoBeads(t *testing.T) {
	app := &App{
		showWelcome:     false,
		activeWorkspace: &data.Workspace{},
		activeProject:   &data.Project{Path: "/test"},
		ticketServices:  nil,
	}
	if count := app.centerButtonCount(); count != 1 {
		t.Errorf("expected 1 button, got %d", count)
	}
}

func TestCenterButtonCount_ActiveWorkspace_WithBeads(t *testing.T) {
	app := &App{
		showWelcome:     false,
		activeWorkspace: &data.Workspace{},
		activeProject:   &data.Project{Path: "/test"},
		ticketServices: map[string]*tickets.TicketService{
			"/test": {},
		},
	}
	if count := app.centerButtonCount(); count != 2 {
		t.Errorf("expected 2 buttons, got %d", count)
	}
}

func TestActivateCenterButton_WithBeads(t *testing.T) {
	app := &App{
		showWelcome:     false,
		activeWorkspace: &data.Workspace{},
		activeProject:   &data.Project{Path: "/test"},
		ticketServices: map[string]*tickets.TicketService{
			"/test": {},
		},
	}

	app.centerBtnIndex = 0
	cmd := app.activateCenterButton()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(messages.ShowSelectTicketDialog); !ok {
		t.Errorf("expected ShowSelectTicketDialog, got %v", reflect.TypeOf(msg))
	}

	app.centerBtnIndex = 1
	cmd = app.activateCenterButton()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg = cmd()
	if _, ok := msg.(messages.ShowSelectAssistantDialog); !ok {
		t.Errorf("expected ShowSelectAssistantDialog, got %v", reflect.TypeOf(msg))
	}
}
