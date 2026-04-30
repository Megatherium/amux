package app

import "github.com/andyrewlee/amux/internal/messages"

//nolint:cyclop // legacy suppression
func (a *App) prefixActionVisible(action string) bool {
	// Keep behavior permissive in lightweight tests that don't fully initialize App state.
	if a == nil || a.ui == nil || a.ui.layout == nil || a.ui.center == nil || a.ui.sidebarTerminal == nil {
		return true
	}

	switch action {
	case "focus_left":
		return a.focusedPane != messages.PaneDashboard
	case "focus_right":
		switch a.focusedPane {
		case messages.PaneSidebar, messages.PaneSidebarTerminal:
			return false
		case messages.PaneCenter:
			return a.ui.layout != nil && a.ui.layout.ShowSidebar()
		default:
			return (a.ui.layout != nil && a.ui.layout.ShowCenter()) || (a.ui.layout != nil && a.ui.layout.ShowSidebar())
		}
	case "toggle_both_sidebars", "toggle_dashboard", "toggle_sidebar":
		return a.ui.layout != nil && a.ui.layout.ShowCenter()
	case "new_agent_tab", "new_agent_tab_direct", "new_terminal_tab":
		if a.activeWorkspace == nil || a.activeProject == nil {
			return false
		}
		return !a.tmuxCheckDone || a.tmuxAvailable
	case "delete_workspace":
		return a.activeWorkspace != nil && a.activeProject != nil
	case "next_tab", "prev_tab":
		switch a.focusedPane {
		case messages.PaneSidebarTerminal:
			return a.ui.sidebarTerminal.HasMultipleTabs()
		case messages.PaneSidebar:
			return true
		default:
			return a.ui.center.HasTabs()
		}
	case "close_tab", "detach_tab", "reattach_tab", "restart_tab":
		if a.focusedPane == messages.PaneSidebarTerminal {
			return true
		}
		return a.ui.center.HasTabs()
	default:
		return true
	}
}

func (a *App) showNumericTabJump() bool {
	if a == nil || a.ui == nil || a.ui.center == nil {
		return true
	}
	tabs, _ := a.ui.center.GetTabsInfo()
	return len(tabs) > 1
}
