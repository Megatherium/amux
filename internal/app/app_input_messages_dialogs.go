package app

import (
	"fmt"
	"os"
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/logging"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/tickets"
	"github.com/andyrewlee/amux/internal/ui/common"
	"github.com/andyrewlee/amux/internal/validation"
)

// handleShowAddProjectDialog shows the add project file picker.
func (a *App) handleShowAddProjectDialog() {
	logging.Info("Showing Add Project file picker")
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/"
	}
	a.ui.filePicker = common.NewFilePicker(DialogAddProject, home, true)
	a.ui.filePicker.SetTitle("Add Project")
	a.ui.filePicker.SetPrimaryActionLabel("Add as project")
	a.ui.filePicker.SetSize(a.width, a.height)
	a.ui.filePicker.SetShowKeymapHints(a.config.UI.ShowKeymapHints)
	a.ui.filePicker.Show()
}

// handleShowCreateWorkspaceDialog shows the create workspace dialog.
func (a *App) handleShowCreateWorkspaceDialog(msg messages.ShowCreateWorkspaceDialog) {
	a.dialogProject = msg.Project
	a.ui.dialog = common.NewInputDialog(DialogCreateWorkspace, "Create Workspace", "Enter workspace name...")
	a.ui.dialog.SetInputValidate(func(s string) string {
		s = validation.SanitizeInput(s)
		if s == "" {
			return "" // Don't show error for empty input
		}
		if err := validation.ValidateWorkspaceName(s); err != nil {
			return err.Error()
		}
		return ""
	})
	a.ui.dialog.SetSize(a.width, a.height)
	a.ui.dialog.SetShowKeymapHints(a.config.UI.ShowKeymapHints)
	a.ui.dialog.Show()
}

// handleShowDeleteWorkspaceDialog shows the delete workspace dialog.
func (a *App) handleShowDeleteWorkspaceDialog(msg messages.ShowDeleteWorkspaceDialog) {
	a.dialogProject = msg.Project
	a.dialogWorkspace = msg.Workspace
	a.ui.dialog = common.NewConfirmDialog(
		DialogDeleteWorkspace,
		"Delete Workspace",
		fmt.Sprintf("Delete workspace '%s' and its branch?", msg.Workspace.Name),
	)
	a.ui.dialog.SetSize(a.width, a.height)
	a.ui.dialog.SetShowKeymapHints(a.config.UI.ShowKeymapHints)
	a.ui.dialog.Show()
}

// handleShowRemoveProjectDialog shows the remove project dialog.
func (a *App) handleShowRemoveProjectDialog(msg messages.ShowRemoveProjectDialog) {
	a.dialogProject = msg.Project
	projectName := ""
	if msg.Project != nil {
		projectName = msg.Project.Name
	}
	a.ui.dialog = common.NewConfirmDialog(
		DialogRemoveProject,
		"Remove Project",
		fmt.Sprintf("Remove project '%s' from AMUX? This won't delete any files.", projectName),
	)
	a.ui.dialog.SetSize(a.width, a.height)
	a.ui.dialog.SetShowKeymapHints(a.config.UI.ShowKeymapHints)
	a.ui.dialog.Show()
}

// handleShowSelectAssistantDialog shows the select assistant dialog.
func (a *App) handleShowSelectAssistantDialog() {
	if a.activeWorkspace == nil && a.pendingWorkspaceProject == nil {
		return
	}
	a.ui.dialog = common.NewAgentPicker(a.assistantNames())
	a.ui.dialog.SetSize(a.width, a.height)
	a.ui.dialog.SetShowKeymapHints(a.config.UI.ShowKeymapHints)
	a.ui.dialog.Show()
}

// handleShowSelectTicketDialog shows the ticket picker dialog.
// If no ticket service is available for the active project, it falls back
// directly to the assistant picker.
func (a *App) handleShowSelectTicketDialog() tea.Cmd {
	if a.activeWorkspace == nil || a.activeProject == nil {
		return nil
	}
	svc := a.ticketServices[a.activeProject.Path]
	if svc == nil {
		return func() tea.Msg { return messages.ShowSelectAssistantDialog{} }
	}
	return func() tea.Msg {
		t, _ := loadOpenAndInProgress(svc, a.activeProject.Path, 50)
		return ticketsForPickerLoaded{tickets: t}
	}
}

// ticketsForPickerLoaded is an internal message carrying tickets for the picker dialog.
type ticketsForPickerLoaded struct {
	tickets []tickets.Ticket
}

func (a *App) handleTicketsForPickerLoaded(msg ticketsForPickerLoaded) {
	// Sort tickets hierarchically: parents first, children after their parent
	sorted := sortTicketsHierarchically(msg.tickets)

	items := make([]common.TicketPickerItem, 0, len(sorted))
	for _, t := range sorted {
		items = append(items, common.TicketPickerItem{
			ID:        t.ID,
			Title:     t.Title,
			Status:    t.Status,
			IssueType: t.IssueType,
			Priority:  t.Priority,
			ParentID:  t.ParentID,
		})
	}
	a.pendingTickets = sorted
	a.ui.dialog = common.NewTicketPicker(items)
	a.ui.dialog.SetSize(a.width, a.height)
	a.ui.dialog.SetShowKeymapHints(a.config.UI.ShowKeymapHints)
	a.ui.dialog.Show()
}

// handleShowCleanupTmuxDialog shows the tmux cleanup dialog.
func (a *App) handleShowCleanupTmuxDialog() {
	if a.ui.dialog != nil && a.ui.dialog.Visible() {
		return
	}
	a.ui.dialog = common.NewConfirmDialog(
		DialogCleanupTmux,
		"Cleanup tmux sessions",
		fmt.Sprintf("Kill all amux-* tmux sessions on server %q?", a.tmuxOptions.ServerName),
	)
	a.ui.dialog.SetSize(a.width, a.height)
	a.ui.dialog.SetShowKeymapHints(a.config.UI.ShowKeymapHints)
	a.ui.dialog.Show()
}

// handleShowSettingsDialog shows the settings dialog.
func (a *App) handleShowSettingsDialog() {
	persistedUI := a.config.PersistedUISettings()
	a.settingsThemePersistedTheme = common.ThemeID(persistedUI.Theme)
	a.settingsThemeDirty = common.ThemeID(a.config.UI.Theme) != a.settingsThemePersistedTheme
	a.settingsDialogSession++
	a.ui.settingsDialog = common.NewSettingsDialog(
		common.ThemeID(a.config.UI.Theme),
	)
	a.ui.settingsDialog.SetSession(a.settingsDialogSession)
	a.ui.settingsDialog.SetSize(a.width, a.height)

	// Set update state
	if a.updateAvailable != nil {
		a.ui.settingsDialog.SetUpdateInfo(
			a.updateAvailable.CurrentVersion,
			a.updateAvailable.LatestVersion,
			a.updateAvailable.UpdateAvailable,
		)
	} else {
		a.ui.settingsDialog.SetUpdateInfo(a.version, "", false)
	}
	if a.updateService != nil && a.updateService.IsHomebrewBuild() {
		a.ui.settingsDialog.SetUpdateHint("Installed via Homebrew - update with brew upgrade amux")
	}

	a.ui.settingsDialog.Show()
}

func (a *App) applyTheme(theme common.ThemeID) {
	common.SetCurrentTheme(theme)
	a.config.UI.Theme = string(theme)
	a.settingsThemeDirty = theme != a.settingsThemePersistedTheme
	a.styles = common.DefaultStyles()
	// Propagate styles to all components
	a.ui.dashboard.SetStyles(a.styles)
	a.ui.sidebar.SetStyles(a.styles)
	a.ui.sidebarTerminal.SetStyles(a.styles)
	a.ui.center.SetStyles(a.styles)
	a.ui.toast.SetStyles(a.styles)
	if a.ui.filePicker != nil {
		a.ui.filePicker.SetStyles(a.styles)
	}
}

// handleThemePreview handles live theme preview.
func (a *App) handleThemePreview(msg common.ThemePreview) tea.Cmd {
	if msg.Session != a.settingsDialogSession {
		return nil
	}
	if a.ui.settingsDialog != nil {
		a.ui.settingsDialog.SetSelectedTheme(msg.Theme)
	}
	a.applyTheme(msg.Theme)
	return nil
}

func (a *App) persistSettingsThemeIfDirty() tea.Cmd {
	if !a.settingsThemeDirty {
		return nil
	}
	if err := a.config.SaveUISettings(); err != nil {
		logging.Warn("Failed to save theme setting: %v", err)
		return a.ui.toast.ShowWarning("Failed to save theme setting")
	}
	a.settingsThemePersistedTheme = common.ThemeID(a.config.UI.Theme)
	a.settingsThemeDirty = false
	return nil
}

// handleSettingsResult handles settings dialog close.
func (a *App) handleSettingsResult(_ common.SettingsResult) tea.Cmd {
	if a.ui.settingsDialog != nil {
		a.applyTheme(a.ui.settingsDialog.SelectedTheme())
	}
	a.ui.settingsDialog = nil
	a.settingsDialogSession++
	return a.persistSettingsThemeIfDirty()
}

// handleTicketSelected handles a ticket selection from the dashboard,
// starting the draft flow in the center pane.
func (a *App) handleTicketSelected(msg messages.TicketSelectedMsg) []tea.Cmd {
	if msg.Ticket == nil {
		return nil
	}

	project := msg.Project
	if project == nil {
		return []tea.Cmd{a.ui.toast.ShowError("No project associated with this ticket")}
	}

	var mainWS *data.Workspace
	for i := range project.Workspaces {
		ws := &project.Workspaces[i]
		if ws.IsMainBranch() || ws.IsPrimaryCheckout() {
			mainWS = ws
			break
		}
	}
	if mainWS == nil {
		return []tea.Cmd{a.ui.toast.ShowError("No workspace found for project " + project.Name)}
	}

	var cmds []tea.Cmd
	if a.activeWorkspace == nil || string(a.activeWorkspace.ID()) != string(mainWS.ID()) {
		cmds = append(cmds, a.handleWorkspaceActivated(messages.WorkspaceActivated{
			Project:   project,
			Workspace: mainWS,
		})...)
	}

	a.ui.center.StartDraft(msg.Ticket, mainWS)
	cmds = append(cmds, a.focusPane(messages.PaneCenter))
	return cmds
}

// handleTicketPreview updates the preview state when the cursor hovers over
// a ticket row in the dashboard. When no agent tabs are open, the ticket info
// is shown in the center pane. When agent tabs exist, the info is shown in the
// sidebar's ticket tab (without auto-focusing the sidebar).
func (a *App) handleTicketPreview(msg messages.TicketPreviewMsg) {
	a.previewTicket = msg.Ticket
	a.previewProject = msg.Project

	// Forward the preview to the sidebar for the ticket tab
	if a.ui.sidebar != nil {
		a.ui.sidebar.SetPreviewTicket(msg.Ticket)
	}
}

// sortTicketsHierarchically reorders tickets so that child tickets
// (tickets with a non-empty ParentID) appear immediately after
// their parent.
func sortTicketsHierarchically(ts []tickets.Ticket) []tickets.Ticket {
	if len(ts) == 0 {
		return ts
	}

	// Build parent index map: ticket ID → position
	parentPos := make(map[string]int, len(ts))
	for i, t := range ts {
		parentPos[t.ID] = i
	}

	// Separate children from parents
	var ordered []tickets.Ticket
	childrenOf := make(map[int][]tickets.Ticket) // parent position → children

	for _, t := range ts {
		if t.ParentID == "" {
			// Parent or root ticket
			ordered = append(ordered, t)
		} else {
			// Child ticket - defer placement
			pidx, ok := parentPos[t.ParentID]
			if ok {
				childrenOf[pidx] = append(childrenOf[pidx], t)
			} else {
				// Parent not in this batch, treat as root
				ordered = append(ordered, t)
			}
		}
	}

	// Insert children after their parents (iterate in reverse to preserve order)
	for i := len(ordered) - 1; i >= 0; i-- {
		// Find the original position of this parent to look up children
		for pidx, children := range childrenOf {
			if pidx < 0 || pidx >= len(ts) {
				continue
			}
			if ts[pidx].ID == ordered[i].ID {
				// Insert children directly after this parent
				sort.Slice(children, func(a, b int) bool {
					return children[a].CreatedAt.Before(children[b].CreatedAt)
				})
				insertAt := i + 1
				ordered = append(ordered[:insertAt], append(children, ordered[insertAt:]...)...)
				delete(childrenOf, pidx)
				break
			}
		}
	}

	return ordered
}
