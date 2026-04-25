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
	a.filePicker = common.NewFilePicker(DialogAddProject, home, true)
	a.filePicker.SetTitle("Add Project")
	a.filePicker.SetPrimaryActionLabel("Add as project")
	a.filePicker.SetSize(a.width, a.height)
	a.filePicker.SetShowKeymapHints(a.config.UI.ShowKeymapHints)
	a.filePicker.Show()
}

// handleShowCreateWorkspaceDialog shows the create workspace dialog.
func (a *App) handleShowCreateWorkspaceDialog(msg messages.ShowCreateWorkspaceDialog) {
	a.dialogProject = msg.Project
	a.dialog = common.NewInputDialog(DialogCreateWorkspace, "Create Workspace", "Enter workspace name...")
	a.dialog.SetInputValidate(func(s string) string {
		s = validation.SanitizeInput(s)
		if s == "" {
			return "" // Don't show error for empty input
		}
		if err := validation.ValidateWorkspaceName(s); err != nil {
			return err.Error()
		}
		return ""
	})
	a.dialog.SetSize(a.width, a.height)
	a.dialog.SetShowKeymapHints(a.config.UI.ShowKeymapHints)
	a.dialog.Show()
}

// handleShowDeleteWorkspaceDialog shows the delete workspace dialog.
func (a *App) handleShowDeleteWorkspaceDialog(msg messages.ShowDeleteWorkspaceDialog) {
	a.dialogProject = msg.Project
	a.dialogWorkspace = msg.Workspace
	a.dialog = common.NewConfirmDialog(
		DialogDeleteWorkspace,
		"Delete Workspace",
		fmt.Sprintf("Delete workspace '%s' and its branch?", msg.Workspace.Name),
	)
	a.dialog.SetSize(a.width, a.height)
	a.dialog.SetShowKeymapHints(a.config.UI.ShowKeymapHints)
	a.dialog.Show()
}

// handleShowRemoveProjectDialog shows the remove project dialog.
func (a *App) handleShowRemoveProjectDialog(msg messages.ShowRemoveProjectDialog) {
	a.dialogProject = msg.Project
	projectName := ""
	if msg.Project != nil {
		projectName = msg.Project.Name
	}
	a.dialog = common.NewConfirmDialog(
		DialogRemoveProject,
		"Remove Project",
		fmt.Sprintf("Remove project '%s' from AMUX? This won't delete any files.", projectName),
	)
	a.dialog.SetSize(a.width, a.height)
	a.dialog.SetShowKeymapHints(a.config.UI.ShowKeymapHints)
	a.dialog.Show()
}

// handleShowSelectAssistantDialog shows the select assistant dialog.
func (a *App) handleShowSelectAssistantDialog() {
	if a.activeWorkspace == nil && a.pendingWorkspaceProject == nil {
		return
	}
	a.dialog = common.NewAgentPicker(a.assistantNames())
	a.dialog.SetSize(a.width, a.height)
	a.dialog.SetShowKeymapHints(a.config.UI.ShowKeymapHints)
	a.dialog.Show()
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
	a.dialog = common.NewTicketPicker(items)
	a.dialog.SetSize(a.width, a.height)
	a.dialog.SetShowKeymapHints(a.config.UI.ShowKeymapHints)
	a.dialog.Show()
}

// handleShowCleanupTmuxDialog shows the tmux cleanup dialog.
func (a *App) handleShowCleanupTmuxDialog() {
	if a.dialog != nil && a.dialog.Visible() {
		return
	}
	a.dialog = common.NewConfirmDialog(
		DialogCleanupTmux,
		"Cleanup tmux sessions",
		fmt.Sprintf("Kill all amux-* tmux sessions on server %q?", a.tmuxOptions.ServerName),
	)
	a.dialog.SetSize(a.width, a.height)
	a.dialog.SetShowKeymapHints(a.config.UI.ShowKeymapHints)
	a.dialog.Show()
}

// handleShowSettingsDialog shows the settings dialog.
func (a *App) handleShowSettingsDialog() {
	persistedUI := a.config.PersistedUISettings()
	a.settingsThemePersistedTheme = common.ThemeID(persistedUI.Theme)
	a.settingsThemeDirty = common.ThemeID(a.config.UI.Theme) != a.settingsThemePersistedTheme
	a.settingsDialogSession++
	a.settingsDialog = common.NewSettingsDialog(
		common.ThemeID(a.config.UI.Theme),
	)
	a.settingsDialog.SetSession(a.settingsDialogSession)
	a.settingsDialog.SetSize(a.width, a.height)

	// Set update state
	if a.updateAvailable != nil {
		a.settingsDialog.SetUpdateInfo(
			a.updateAvailable.CurrentVersion,
			a.updateAvailable.LatestVersion,
			a.updateAvailable.UpdateAvailable,
		)
	} else {
		a.settingsDialog.SetUpdateInfo(a.version, "", false)
	}
	if a.updateService != nil && a.updateService.IsHomebrewBuild() {
		a.settingsDialog.SetUpdateHint("Installed via Homebrew - update with brew upgrade amux")
	}

	a.settingsDialog.Show()
}

func (a *App) applyTheme(theme common.ThemeID) {
	common.SetCurrentTheme(theme)
	a.config.UI.Theme = string(theme)
	a.settingsThemeDirty = theme != a.settingsThemePersistedTheme
	a.styles = common.DefaultStyles()
	// Propagate styles to all components
	a.dashboard.SetStyles(a.styles)
	a.sidebar.SetStyles(a.styles)
	a.sidebarTerminal.SetStyles(a.styles)
	a.center.SetStyles(a.styles)
	a.toast.SetStyles(a.styles)
	if a.filePicker != nil {
		a.filePicker.SetStyles(a.styles)
	}
}

// handleThemePreview handles live theme preview.
func (a *App) handleThemePreview(msg common.ThemePreview) tea.Cmd {
	if msg.Session != a.settingsDialogSession {
		return nil
	}
	if a.settingsDialog != nil {
		a.settingsDialog.SetSelectedTheme(msg.Theme)
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
		return a.toast.ShowWarning("Failed to save theme setting")
	}
	a.settingsThemePersistedTheme = common.ThemeID(a.config.UI.Theme)
	a.settingsThemeDirty = false
	return nil
}

// handleSettingsResult handles settings dialog close.
func (a *App) handleSettingsResult(_ common.SettingsResult) tea.Cmd {
	if a.settingsDialog != nil {
		a.applyTheme(a.settingsDialog.SelectedTheme())
	}
	a.settingsDialog = nil
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
		return []tea.Cmd{a.toast.ShowError("No project associated with this ticket")}
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
		return []tea.Cmd{a.toast.ShowError("No workspace found for project " + project.Name)}
	}

	var cmds []tea.Cmd
	if a.activeWorkspace == nil || string(a.activeWorkspace.ID()) != string(mainWS.ID()) {
		cmds = append(cmds, a.handleWorkspaceActivated(messages.WorkspaceActivated{
			Project:   project,
			Workspace: mainWS,
		})...)
	}

	a.center.StartDraft(msg.Ticket, mainWS)
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
	if a.sidebar != nil {
		a.sidebar.SetPreviewTicket(msg.Ticket)
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
