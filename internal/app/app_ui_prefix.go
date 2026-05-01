package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/app/orchestrator"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/ui/common"
)

// prefixCommandAction maps string action names to handler functions.
type prefixCommandAction struct {
	Sequence []string
	Desc     string
	Action   string
}

var prefixCommandTable = []prefixCommandAction{
	{Sequence: []string{"a"}, Desc: "add project", Action: "add_project"},
	{Sequence: []string{"b", "b"}, Desc: "toggle both sidebars", Action: "toggle_both_sidebars"},
	{Sequence: []string{"b", "h"}, Desc: "toggle dashboard", Action: "toggle_dashboard"},
	{Sequence: []string{"b", "l"}, Desc: "toggle sidebar", Action: "toggle_sidebar"},
	{Sequence: []string{"d"}, Desc: "delete workspace", Action: "delete_workspace"},
	{Sequence: []string{"S"}, Desc: "Settings", Action: "open_settings"},
	{Sequence: []string{"q"}, Desc: "quit", Action: "quit"},
	{Sequence: []string{"K"}, Desc: "cleanup tmux", Action: "cleanup_tmux"},
	{Sequence: []string{"h"}, Desc: "focus left", Action: "focus_left"},
	{Sequence: []string{"l"}, Desc: "focus right", Action: "focus_right"},
	{Sequence: []string{"t", "a"}, Desc: "new agent", Action: "new_agent_tab_direct"},
	{Sequence: []string{"t", "b"}, Desc: "new agent with ticket", Action: "new_agent_tab"},
	{Sequence: []string{"t", "t"}, Desc: "new terminal tab", Action: "new_terminal_tab"},
	{Sequence: []string{"t", "n"}, Desc: "next tab", Action: "next_tab"},
	{Sequence: []string{"t", "p"}, Desc: "prev tab", Action: "prev_tab"},
	{Sequence: []string{"t", "x"}, Desc: "close tab", Action: "close_tab"},
	{Sequence: []string{"t", "d"}, Desc: "detach tab", Action: "detach_tab"},
	{Sequence: []string{"t", "r"}, Desc: "reattach tab", Action: "reattach_tab"},
	{Sequence: []string{"t", "s"}, Desc: "restart tab", Action: "restart_tab"},
}

// prefixCommands returns the prefix command table for the palette.
func (a *App) prefixCommands() []prefixCommandAction {
	return prefixCommandTable
}

// matchingPrefixCommands returns commands whose sequences start with the given prefix.
func (a *App) matchingPrefixCommands(sequence []string) []prefixCommandAction {
	if len(sequence) == 0 {
		return prefixCommandTable
	}
	var matches []prefixCommandAction
	for _, cmd := range prefixCommandTable {
		if len(sequence) > len(cmd.Sequence) {
			continue
		}
		ok := true
		for i := range sequence {
			if cmd.Sequence[i] != sequence[i] {
				ok = false
				break
			}
		}
		if ok {
			matches = append(matches, cmd)
		}
	}
	return matches
}

// buildPrefixCommands converts the app's prefix command table to
// orchestrator.PrefixCommand format, binding actions via runPrefixAction.
func (a *App) buildPrefixCommands() []orchestrator.PrefixCommand {
	result := make([]orchestrator.PrefixCommand, len(prefixCommandTable))
	for i, cmd := range prefixCommandTable {
		action := cmd.Action // capture
		result[i] = orchestrator.PrefixCommand{
			Sequence: cmd.Sequence,
			Label:    cmd.Desc,
			Help:     cmd.Desc,
			Action:   func() tea.Cmd { return a.runPrefixAction(action) },
		}
	}
	return result
}

func (a *App) runPrefixAction(action string) tea.Cmd {
	switch action {
	case "focus_left":
		return a.focusPaneLeft()
	case "focus_right":
		return a.focusPaneRight()
	case "add_project":
		return func() tea.Msg { return messages.ShowAddProjectDialog{} }
	case "delete_workspace":
		if a.activeWorkspace == nil || a.activeProject == nil {
			return a.requireWorkspaceSelection("delete workspace")
		}
		return func() tea.Msg {
			return messages.ShowDeleteWorkspaceDialog{
				Project:   a.activeProject,
				Workspace: a.activeWorkspace,
			}
		}
	case "open_settings":
		return func() tea.Msg { return messages.ShowSettingsDialog{} }
	case "toggle_both_sidebars":
		return a.togglePaneCollapse("both")
	case "toggle_dashboard":
		return a.togglePaneCollapse("dashboard")
	case "toggle_sidebar":
		return a.togglePaneCollapse("sidebar")
	case "quit":
		a.showQuitDialog()
		return nil
	case "cleanup_tmux":
		return func() tea.Msg { return messages.ShowCleanupTmuxDialog{} }
	case "new_agent_tab":
		if a.activeWorkspace == nil || a.activeProject == nil {
			return a.requireWorkspaceSelection("create agent tab")
		}
		if !a.tmuxAvailable {
			return a.ui.toast.ShowError("tmux required to create tabs. " + a.tmuxInstallHint)
		}
		return func() tea.Msg { return messages.ShowSelectTicketDialog{} }
	case "new_agent_tab_direct":
		if a.activeWorkspace == nil || a.activeProject == nil {
			return a.requireWorkspaceSelection("create agent tab")
		}
		if !a.tmuxAvailable {
			return a.ui.toast.ShowError("tmux required to create tabs. " + a.tmuxInstallHint)
		}
		return func() tea.Msg { return messages.ShowSelectAssistantDialog{} }
	case "new_terminal_tab":
		if a.activeWorkspace == nil || a.activeProject == nil {
			return a.requireWorkspaceSelection("create terminal tab")
		}
		if !a.tmuxAvailable {
			return a.ui.toast.ShowError("tmux required to create tabs. " + a.tmuxInstallHint)
		}
		return a.ui.sidebarTerminal.CreateNewTab()
	case "next_tab":
		return a.cycleTab(a.ui.sidebar.NextTab, a.ui.sidebarTerminal.NextTab, a.ui.center.NextTab)
	case "prev_tab":
		return a.cycleTab(a.ui.sidebar.PrevTab, a.ui.sidebarTerminal.PrevTab, a.ui.center.PrevTab)
	case "close_tab":
		if a.oc().Focus.FocusedPane == messages.PaneSidebarTerminal {
			return a.ui.sidebarTerminal.CloseActiveTab()
		}
		return a.ui.center.CloseActiveTab()
	case "detach_tab":
		return a.dispatchTabAction(
			func() tea.Cmd { return common.SafeBatch(a.ui.center.DetachActiveTab(), a.persistActiveWorkspaceTabs()) },
			a.ui.sidebarTerminal.DetachActiveTab,
		)
	case "reattach_tab":
		return a.dispatchTabAction(a.ui.center.ReattachActiveTab, a.ui.sidebarTerminal.ReattachActiveTab)
	case "restart_tab":
		return a.dispatchTabAction(a.ui.center.RestartActiveTab, a.ui.sidebarTerminal.RestartActiveTab)
	default:
		return nil
	}
}
