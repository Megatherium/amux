package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/app/orchestrator"
	"github.com/andyrewlee/amux/internal/config"
	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/ui/center"
	"github.com/andyrewlee/amux/internal/ui/dashboard"
	"github.com/andyrewlee/amux/internal/ui/layout"
	"github.com/andyrewlee/amux/internal/ui/sidebar"
)

func newPrefixTestApp(t *testing.T) (*App, *data.Workspace, *center.Model) {
	t.Helper()

	cfg := &config.Config{
		Assistants: map[string]config.AssistantConfig{
			"claude": {},
		},
	}
	ws := &data.Workspace{
		Name: "ws",
		Repo: "/repo/ws",
		Root: "/repo/ws",
	}
	centerModel := center.New(cfg)
	centerModel.SetWorkspace(ws)

	app := &App{
		ui: &UICompositor{
			center: centerModel,
		},
		keymap: DefaultKeyMap(),
	}
	return app, ws, centerModel
}

// newLayoutTestApp returns an App with all UI components initialized for layout testing.
func newLayoutTestApp(t *testing.T) *App {
	t.Helper()

	cfg := &config.Config{
		Assistants: map[string]config.AssistantConfig{
			"claude": {},
		},
	}

	app := &App{
		ui: &UICompositor{
			center:          center.New(cfg),
			dashboard:       dashboard.New(),
			sidebar:         sidebar.NewTabbedSidebar(),
			sidebarTerminal: sidebar.NewTerminalModel(),
			layout:          layout.NewManager(),
		},
		keymap: DefaultKeyMap(),
	}
	app.ui.layout.Resize(200, 40)
	app.updateLayout()
	return app
}

func TestHandlePrefixNumericTabSelection_InvalidIndexNoOp(t *testing.T) {
	app, ws, centerModel := newPrefixTestApp(t)
	centerModel.AddTab(&center.Tab{
		ID:          center.TabID("tab-1"),
		Name:        "Claude",
		Assistant:   "claude",
		Workspace:   ws,
		SessionName: "sess-1",
		Detached:    true,
	})

	status, cmd := app.handlePrefixCommand(tea.KeyPressMsg{Code: '9', Text: "9"})
	if status != orchestrator.PrefixMatchComplete {
		t.Fatalf("expected numeric shortcut to complete, got %v", status)
	}
	if cmd != nil {
		t.Fatalf("expected out-of-range numeric selection to return nil command")
	}
}

func TestHandlePrefixNumericTabSelection_ValidIndexTriggersReattach(t *testing.T) {
	app, ws, centerModel := newPrefixTestApp(t)
	centerModel.AddTab(&center.Tab{
		ID:          center.TabID("tab-1"),
		Name:        "Claude 1",
		Assistant:   "claude",
		Workspace:   ws,
		SessionName: "sess-1",
		Detached:    false,
		Running:     true,
	})
	centerModel.AddTab(&center.Tab{
		ID:          center.TabID("tab-2"),
		Name:        "Claude 2",
		Assistant:   "claude",
		Workspace:   ws,
		SessionName: "sess-2",
		Detached:    true,
	})

	status, cmd := app.handlePrefixCommand(tea.KeyPressMsg{Code: '2', Text: "2"})
	if status != orchestrator.PrefixMatchComplete {
		t.Fatalf("expected numeric shortcut to complete, got %v", status)
	}
	if cmd == nil {
		t.Fatalf("expected valid numeric selection to trigger follow-up command")
	}
}

func TestHandlePrefixNextTab_SingleTabNoOp(t *testing.T) {
	app, ws, centerModel := newPrefixTestApp(t)
	centerModel.AddTab(&center.Tab{
		ID:          center.TabID("tab-1"),
		Name:        "Claude",
		Assistant:   "claude",
		Workspace:   ws,
		SessionName: "sess-1",
		Detached:    true,
	})

	status, cmd := app.handlePrefixCommand(tea.KeyPressMsg{Code: 't', Text: "t"})
	if status != orchestrator.PrefixMatchPartial {
		t.Fatalf("expected first key to narrow prefix sequence, got %v", status)
	}
	if cmd != nil {
		t.Fatalf("expected partial sequence to return nil command")
	}

	status, cmd = app.handlePrefixCommand(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if status != orchestrator.PrefixMatchComplete {
		t.Fatalf("expected next-tab sequence to complete, got %v", status)
	}
	if cmd != nil {
		t.Fatalf("expected single-tab next to be a no-op without reattach command")
	}
}

func TestHandlePrefixPrevTab_SingleTabNoOp(t *testing.T) {
	app, ws, centerModel := newPrefixTestApp(t)
	centerModel.AddTab(&center.Tab{
		ID:          center.TabID("tab-1"),
		Name:        "Claude",
		Assistant:   "claude",
		Workspace:   ws,
		SessionName: "sess-1",
		Detached:    true,
	})

	status, cmd := app.handlePrefixCommand(tea.KeyPressMsg{Code: 't', Text: "t"})
	if status != orchestrator.PrefixMatchPartial {
		t.Fatalf("expected first key to narrow prefix sequence, got %v", status)
	}
	if cmd != nil {
		t.Fatalf("expected partial sequence to return nil command")
	}

	status, cmd = app.handlePrefixCommand(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if status != orchestrator.PrefixMatchComplete {
		t.Fatalf("expected prev-tab sequence to complete, got %v", status)
	}
	if cmd != nil {
		t.Fatalf("expected single-tab prev to be a no-op without reattach command")
	}
}

func TestHandlePrefixCommand_BackspaceAtRootNoop(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)

	status, cmd := app.handlePrefixCommand(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if status != orchestrator.PrefixMatchPartial {
		t.Fatalf("expected backspace at root to keep prefix active, got %v", status)
	}
	if cmd != nil {
		t.Fatalf("expected backspace at root to return nil command")
	}
	if len(app.oc().Prefix.Sequence) != 0 {
		t.Fatalf("expected empty prefix sequence after root backspace, got %v", app.oc().Prefix.Sequence)
	}
}

func TestHandlePrefixCommand_BackspaceUndoesLastToken(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)
	app.oc().Prefix.Sequence = []string{"t", "n"}

	status, cmd := app.handlePrefixCommand(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if status != orchestrator.PrefixMatchPartial {
		t.Fatalf("expected backspace undo to keep prefix active, got %v", status)
	}
	if cmd != nil {
		t.Fatalf("expected backspace undo to return nil command")
	}
	if len(app.oc().Prefix.Sequence) != 1 || app.oc().Prefix.Sequence[0] != "t" {
		t.Fatalf("expected sequence to be reduced to [t], got %v", app.oc().Prefix.Sequence)
	}
}

func TestHandleKeyPress_BackspaceAtRootRefreshesPrefixTimeout(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)
	app.oc().Prefix.Active = true
	beforeToken := app.oc().Prefix.Token

	cmd := app.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if cmd == nil {
		t.Fatalf("expected timeout refresh command")
	}
	if !app.oc().Prefix.Active {
		t.Fatalf("expected prefix mode to remain active")
	}
	if len(app.oc().Prefix.Sequence) != 0 {
		t.Fatalf("expected prefix sequence to remain empty, got %v", app.oc().Prefix.Sequence)
	}
	if app.oc().Prefix.Token != beforeToken+1 {
		t.Fatalf("expected prefix token increment, got %d want %d", app.oc().Prefix.Token, beforeToken+1)
	}
}

func TestIsPrefixKey_DoesNotAcceptPrintableAliases(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)

	if app.isPrefixKey(tea.KeyPressMsg{Code: '?', Text: "?"}) {
		t.Fatal("expected '?' not to be treated as global prefix key")
	}
	if app.isPrefixKey(tea.KeyPressMsg{Code: 'H', Text: "H"}) {
		t.Fatal("expected 'H' not to be treated as global prefix key")
	}
}

func TestOpenCommandsPalette_EntersPrefixMode(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)

	cmd := app.openCommandsPalette()
	if cmd == nil {
		t.Fatal("expected command palette to open")
	}
	if !app.oc().Prefix.Active {
		t.Fatal("expected prefix mode to become active")
	}
}

func TestOpenCommandsPalette_ResetsActiveSequence(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)
	app.oc().Prefix.Active = true
	app.oc().Prefix.Sequence = []string{"t"}
	beforeToken := app.oc().Prefix.Token

	cmd := app.openCommandsPalette()
	if cmd == nil {
		t.Fatal("expected palette reset command")
	}
	if !app.oc().Prefix.Active {
		t.Fatal("expected prefix mode to remain active")
	}
	if len(app.oc().Prefix.Sequence) != 0 {
		t.Fatalf("expected prefix sequence reset, got %v", app.oc().Prefix.Sequence)
	}
	if app.oc().Prefix.Token != beforeToken+1 {
		t.Fatalf("expected prefix token increment, got %d want %d", app.oc().Prefix.Token, beforeToken+1)
	}
}

func TestOpenCommandsPalette_AtRootKeepsPrefixActive(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)
	app.oc().Prefix.Active = true
	app.oc().Prefix.Sequence = nil
	beforeToken := app.oc().Prefix.Token

	cmd := app.openCommandsPalette()
	if cmd == nil {
		t.Fatal("expected palette refresh command")
	}
	if !app.oc().Prefix.Active {
		t.Fatal("expected prefix mode to remain active")
	}
	if len(app.oc().Prefix.Sequence) != 0 {
		t.Fatalf("expected prefix sequence to remain empty, got %v", app.oc().Prefix.Sequence)
	}
	if app.oc().Prefix.Token != beforeToken+1 {
		t.Fatalf("expected prefix token increment, got %d want %d", app.oc().Prefix.Token, beforeToken+1)
	}
}

func TestHandleKeyPress_PrefixKeyResetsWhenActive(t *testing.T) {
	app, ws, centerModel := newPrefixTestApp(t)
	centerModel.AddTab(&center.Tab{
		ID:          center.TabID("tab-1"),
		Name:        "Claude",
		Assistant:   "claude",
		Workspace:   ws,
		SessionName: "sess-1",
		Detached:    true,
	})
	app.oc().Focus.FocusedPane = messages.PaneCenter
	app.oc().Prefix.Active = true
	app.oc().Prefix.Sequence = []string{"t"}
	beforeToken := app.oc().Prefix.Token

	cmd := app.handleKeyPress(tea.KeyPressMsg{Code: tea.KeySpace, Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected prefix reset command while palette is open")
	}
	if !app.oc().Prefix.Active {
		t.Fatal("expected prefix mode to remain active")
	}
	if len(app.oc().Prefix.Sequence) != 0 {
		t.Fatalf("expected prefix sequence reset, got %v", app.oc().Prefix.Sequence)
	}
	if app.oc().Prefix.Token != beforeToken+1 {
		t.Fatalf("expected prefix token increment, got %d want %d", app.oc().Prefix.Token, beforeToken+1)
	}
}

func TestHandleKeyPress_PrefixKeyAtRootExitsPrefixMode(t *testing.T) {
	app, _, _ := newPrefixTestApp(t)
	app.oc().Prefix.Active = true
	app.oc().Prefix.Sequence = nil

	cmd := app.handleKeyPress(tea.KeyPressMsg{Code: tea.KeySpace, Mod: tea.ModCtrl})
	if cmd != nil {
		t.Fatal("expected no command when sending literal Ctrl+Space")
	}
	if app.oc().Prefix.Active {
		t.Fatal("expected prefix mode to exit after prefix key at root")
	}
}

func TestMatchingPrefixCommands_IncludesUnavailableActionsForExecutionFallback(t *testing.T) {
	h := newCenterHarness(nil, HarnessOptions{
		Width:  120,
		Height: 24,
		Tabs:   0,
	})
	app := h.app

	matches := app.matchingPrefixCommands([]string{"t"})
	if len(matches) == 0 {
		t.Fatal("expected raw prefix matcher to keep tab actions available for typed execution")
	}
}
