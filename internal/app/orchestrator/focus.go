package orchestrator

import (
	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/messages"
)

// FocusManager handles pane focus state and navigation.
// It is extracted from app_ui.go to separate the focus concern
// from the monolithic App struct.
type FocusManager struct {
	// FocusedPane is the currently focused pane.
	FocusedPane messages.PaneType
}

// FocusContext provides the FocusManager with access to UI pane
// models for focus transitions. This small interface avoids a
// direct dependency on the app package.
type FocusContext interface {
	// Center returns the center pane model, or nil.
	Center() CenterTarget
	// SidebarTerminal returns the sidebar terminal model, or nil.
	SidebarTerminal() SidebarTerminalTarget
	// Layout returns layout visibility information.
	Layout() LayoutInfo
}

// CenterTarget is the subset of center.Model needed for focus management.
type CenterTarget interface {
	ReattachActiveTabIfDetached() tea.Cmd
}

// SidebarTerminalTarget is the subset of sidebar.TerminalModel needed for focus.
type SidebarTerminalTarget interface {
	EnsureTerminalTab() tea.Cmd
}

// LayoutInfo provides pane visibility for focus navigation.
type LayoutInfo interface {
	ShowCenter() bool
	ShowDashboard() bool
	ShowSidebar() bool
}

// SetFocusedPane updates the focused pane without triggering side effects.
func (fm *FocusManager) SetFocusedPane(pane messages.PaneType) {
	fm.FocusedPane = pane
}

// FocusPane changes focus to the specified pane and returns any
// side-effect command (e.g. reattach detached tab, lazy-create terminal).
func (fm *FocusManager) FocusPane(pane messages.PaneType, ctx FocusContext) tea.Cmd {
	fm.FocusedPane = pane
	switch pane {
	case messages.PaneCenter:
		if c := ctx.Center(); c != nil {
			return c.ReattachActiveTabIfDetached()
		}
	case messages.PaneSidebarTerminal:
		if st := ctx.SidebarTerminal(); st != nil {
			return st.EnsureTerminalTab()
		}
	}
	return nil
}

// FocusPaneOnWheel is a lighter variant of FocusPane used during
// hover-wheel routing. It preserves center reattach but skips
// lazy sidebar terminal creation.
func (fm *FocusManager) FocusPaneOnWheel(pane messages.PaneType, ctx FocusContext) tea.Cmd {
	fm.FocusedPane = pane
	if pane == messages.PaneCenter {
		if c := ctx.Center(); c != nil {
			return c.ReattachActiveTabIfDetached()
		}
	}
	return nil
}

// FocusLeft moves focus one pane to the left.
func (fm *FocusManager) FocusLeft(ctx FocusContext) tea.Cmd {
	switch fm.FocusedPane {
	case messages.PaneSidebarTerminal, messages.PaneSidebar:
		if ctx.Layout().ShowCenter() {
			return fm.FocusPane(messages.PaneCenter, ctx)
		}
		return fm.FocusPane(messages.PaneDashboard, ctx)
	case messages.PaneCenter:
		if ctx.Layout().ShowDashboard() {
			return fm.FocusPane(messages.PaneDashboard, ctx)
		}
	}
	return nil
}

// FocusRight moves focus one pane to the right.
func (fm *FocusManager) FocusRight(ctx FocusContext) tea.Cmd {
	switch fm.FocusedPane {
	case messages.PaneDashboard:
		if ctx.Layout().ShowCenter() {
			return fm.FocusPane(messages.PaneCenter, ctx)
		}
		if ctx.Layout().ShowSidebar() {
			return fm.FocusPane(messages.PaneSidebar, ctx)
		}
	case messages.PaneCenter:
		if ctx.Layout().ShowSidebar() {
			return fm.FocusPane(messages.PaneSidebar, ctx)
		}
	}
	return nil
}

// syncPaneFocusFlags keeps pane focus flags in sync with FocusedPane.
// Called before View to ensure flags are consistent.
func (fm *FocusManager) syncPaneFocusFlags() {
	// The original app_ui.go syncPaneFocusFlags method sets focus flags
	// on UI components. This is now done by the App's syncPaneFocusFlags
	// which reads fm.FocusedPane and applies flags.
}
