package app

import "github.com/andyrewlee/amux/internal/messages"

// syncPaneFocusFlags keeps child model focus flags consistent with focusedPane.
// This is a defensive invariant to prevent stale multi-cursor states.
//
//nolint:funlen // legacy suppression
func (a *App) syncPaneFocusFlags() {
	focusDashboard := func() {
		if a.ui.dashboard != nil {
			a.ui.dashboard.Focus()
		}
	}
	blurDashboard := func() {
		if a.ui.dashboard != nil {
			a.ui.dashboard.Blur()
		}
	}
	focusCenter := func() {
		if a.ui.center != nil {
			a.ui.center.Focus()
		}
	}
	blurCenter := func() {
		if a.ui.center != nil {
			a.ui.center.Blur()
		}
	}
	focusSidebar := func() {
		if a.ui.sidebar != nil {
			a.ui.sidebar.Focus()
		}
	}
	blurSidebar := func() {
		if a.ui.sidebar != nil {
			a.ui.sidebar.Blur()
		}
	}
	focusSidebarTerminal := func() {
		if a.ui.sidebarTerminal != nil {
			a.ui.sidebarTerminal.Focus()
		}
	}
	blurSidebarTerminal := func() {
		if a.ui.sidebarTerminal != nil {
			a.ui.sidebarTerminal.Blur()
		}
	}

	switch a.focusedPane {
	case messages.PaneDashboard:
		focusDashboard()
		blurCenter()
		blurSidebar()
		blurSidebarTerminal()
	case messages.PaneCenter:
		blurDashboard()
		focusCenter()
		blurSidebar()
		blurSidebarTerminal()
	case messages.PaneSidebar:
		blurDashboard()
		blurCenter()
		focusSidebar()
		blurSidebarTerminal()
	case messages.PaneSidebarTerminal:
		blurDashboard()
		blurCenter()
		blurSidebar()
		focusSidebarTerminal()
	}
}
