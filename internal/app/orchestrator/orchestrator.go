// Package orchestrator provides UI orchestration primitives for the amux
// Bubble Tea application. It extracts focus management, prefix-mode state,
// and external message-pump logic from the monolithic app package as part
// of a gradual decomposition (epic bmx-gzm).
package orchestrator

import (
	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/messages"
)

// Orchestrator composes UI orchestration concerns that were previously
// embedded directly in the App struct. It is owned by the App and
// delegates to FocusManager, PrefixEngine, and MessagePump.
type Orchestrator struct {
	Focus  *FocusManager
	Prefix *PrefixEngine
	Pump   *MessagePump

	// Keyboard enhancements received from the terminal.
	KeyboardEnhancements tea.KeyboardEnhancementsMsg
}

// New creates an initialized Orchestrator.
func New() *Orchestrator {
	return &Orchestrator{
		Focus: &FocusManager{
			FocusedPane: messages.PaneDashboard,
		},
		Prefix: &PrefixEngine{},
		Pump:   NewMessagePump(),
	}
}

// Shutdown releases resources held by the orchestrator.
// Currently a no-op; channels are owned by the App.
func (o *Orchestrator) Shutdown() {
	// Placeholder for future cleanup.
}

// SyncPaneFocusFlags is a pre-render hook that synchronizes
// pane-level focus flags before the View pass.
func (o *Orchestrator) SyncPaneFocusFlags() {
	o.Focus.syncPaneFocusFlags()
}
