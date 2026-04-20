package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/ui/common"
)

type prefixMatch int

const (
	prefixMatchNone prefixMatch = iota
	prefixMatchPartial
	prefixMatchComplete
)

type prefixCommand struct {
	Sequence []string
	Desc     string
	Action   string
}

var prefixCommandTable = []prefixCommand{
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

func (a *App) prefixCommands() []prefixCommand {
	return prefixCommandTable
}

// matchingPrefixCommands intentionally does not apply prefixActionVisible.
// Command execution remains permissive and unavailable actions fail gracefully
// in runPrefixAction with contextual no-op/toast behavior.
func (a *App) matchingPrefixCommands(sequence []string) []prefixCommand {
	commands := a.prefixCommands()
	if len(sequence) == 0 {
		return commands
	}

	matches := make([]prefixCommand, 0, len(commands))
	for _, cmd := range commands {
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
			return a.toast.ShowError("tmux required to create tabs. " + a.tmuxInstallHint)
		}
		return func() tea.Msg { return messages.ShowSelectTicketDialog{} }
	case "new_agent_tab_direct":
		if a.activeWorkspace == nil || a.activeProject == nil {
			return a.requireWorkspaceSelection("create agent tab")
		}
		if !a.tmuxAvailable {
			return a.toast.ShowError("tmux required to create tabs. " + a.tmuxInstallHint)
		}
		return func() tea.Msg { return messages.ShowSelectAssistantDialog{} }
	case "new_terminal_tab":
		if a.activeWorkspace == nil || a.activeProject == nil {
			return a.requireWorkspaceSelection("create terminal tab")
		}
		if !a.tmuxAvailable {
			return a.toast.ShowError("tmux required to create tabs. " + a.tmuxInstallHint)
		}
		return a.sidebarTerminal.CreateNewTab()
	case "next_tab":
		return a.cycleTab(a.sidebar.NextTab, a.sidebarTerminal.NextTab, a.center.NextTab)
	case "prev_tab":
		return a.cycleTab(a.sidebar.PrevTab, a.sidebarTerminal.PrevTab, a.center.PrevTab)
	case "close_tab":
		if a.focusedPane == messages.PaneSidebarTerminal {
			return a.sidebarTerminal.CloseActiveTab()
		}
		return a.center.CloseActiveTab()
	case "detach_tab":
		return a.dispatchTabAction(
			func() tea.Cmd { return common.SafeBatch(a.center.DetachActiveTab(), a.persistActiveWorkspaceTabs()) },
			a.sidebarTerminal.DetachActiveTab,
		)
	case "reattach_tab":
		return a.dispatchTabAction(a.center.ReattachActiveTab, a.sidebarTerminal.ReattachActiveTab)
	case "restart_tab":
		return a.dispatchTabAction(a.center.RestartActiveTab, a.sidebarTerminal.RestartActiveTab)
	default:
		return nil
	}
}
