package app

import (
	"context"
	"fmt"
	"os"
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/logging"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/tickets"
	"github.com/andyrewlee/amux/internal/ui/common"
	"github.com/andyrewlee/amux/internal/update"
	"github.com/andyrewlee/amux/internal/validation"
)

// ticketsForPickerLoaded is an internal message carrying tickets for the picker dialog.
type ticketsForPickerLoaded struct {
	tickets []tickets.Ticket
}

// ShowAddProjectDialog shows the add project file picker.
func (u *UICompositor) ShowAddProjectDialog() {
	logging.Info("Showing Add Project file picker")
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/"
	}
	u.filePicker = common.NewFilePicker(DialogAddProject, home, true)
	u.filePicker.SetTitle("Add Project")
	u.filePicker.SetPrimaryActionLabel("Add as project")
	u.filePicker.SetSize(u.width, u.height)
	u.filePicker.SetShowKeymapHints(u.config.UI.ShowKeymapHints)
	u.filePicker.Show()
}

// ShowCreateWorkspaceDialog shows the create workspace dialog.
func (u *UICompositor) ShowCreateWorkspaceDialog(msg messages.ShowCreateWorkspaceDialog) {
	u.dialogProject = msg.Project
	u.dialog = common.NewInputDialog(DialogCreateWorkspace, "Create Workspace", "Enter workspace name...")
	u.dialog.SetInputValidate(func(s string) string {
		s = validation.SanitizeInput(s)
		if s == "" {
			return "" // Don't show error for empty input
		}
		if err := validation.ValidateWorkspaceName(s); err != nil {
			return err.Error()
		}
		return ""
	})
	u.dialog.SetSize(u.width, u.height)
	u.dialog.SetShowKeymapHints(u.config.UI.ShowKeymapHints)
	u.dialog.Show()
}

// ShowDeleteWorkspaceDialog shows the delete workspace dialog.
func (u *UICompositor) ShowDeleteWorkspaceDialog(msg messages.ShowDeleteWorkspaceDialog) {
	u.dialogProject = msg.Project
	u.dialogWorkspace = msg.Workspace
	u.dialog = common.NewConfirmDialog(
		DialogDeleteWorkspace,
		"Delete Workspace",
		fmt.Sprintf("Delete workspace '%s' and its branch?", msg.Workspace.Name),
	)
	u.dialog.SetSize(u.width, u.height)
	u.dialog.SetShowKeymapHints(u.config.UI.ShowKeymapHints)
	u.dialog.Show()
}

// ShowRemoveProjectDialog shows the remove project dialog.
func (u *UICompositor) ShowRemoveProjectDialog(msg messages.ShowRemoveProjectDialog) {
	u.dialogProject = msg.Project
	projectName := ""
	if msg.Project != nil {
		projectName = msg.Project.Name
	}
	u.dialog = common.NewConfirmDialog(
		DialogRemoveProject,
		"Remove Project",
		fmt.Sprintf("Remove project '%s' from AMUX? This won't delete any files.", projectName),
	)
	u.dialog.SetSize(u.width, u.height)
	u.dialog.SetShowKeymapHints(u.config.UI.ShowKeymapHints)
	u.dialog.Show()
}

// ShowSelectAssistantDialog shows the select assistant dialog.
func (u *UICompositor) ShowSelectAssistantDialog(activeWorkspace *data.Workspace) {
	if activeWorkspace == nil && u.pendingWorkspaceProject == nil {
		return
	}
	u.dialog = common.NewAgentPicker(u.assistantNames())
	u.dialog.SetSize(u.width, u.height)
	u.dialog.SetShowKeymapHints(u.config.UI.ShowKeymapHints)
	u.dialog.Show()
}

// ShowSelectTicketDialog shows the ticket picker dialog.
// If no ticket service is available for the active project, it falls back
// directly to the assistant picker.
func (u *UICompositor) ShowSelectTicketDialog(ticketServices map[string]*tickets.TicketService, activeWorkspace *data.Workspace, activeProject *data.Project) tea.Cmd {
	if activeWorkspace == nil || activeProject == nil {
		return nil
	}
	svc := ticketServices[activeProject.Path]
	if svc == nil {
		return func() tea.Msg { return messages.ShowSelectAssistantDialog{} }
	}
	return func() tea.Msg {
		t, _ := loadOpenAndInProgress(context.Background(), svc, activeProject.Path, 50)
		return ticketsForPickerLoaded{tickets: t}
	}
}

// HandleTicketsForPickerLoaded processes tickets loaded for the picker dialog.
func (u *UICompositor) HandleTicketsForPickerLoaded(msg ticketsForPickerLoaded) {
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
	u.pendingTickets = sorted
	u.dialog = common.NewTicketPicker(items)
	u.dialog.SetSize(u.width, u.height)
	u.dialog.SetShowKeymapHints(u.config.UI.ShowKeymapHints)
	u.dialog.Show()
}

// ShowCleanupTmuxDialog shows the tmux cleanup dialog.
func (u *UICompositor) ShowCleanupTmuxDialog(serverName string) {
	if u.dialog != nil && u.dialog.Visible() {
		return
	}
	u.dialog = common.NewConfirmDialog(
		DialogCleanupTmux,
		"Cleanup tmux sessions",
		fmt.Sprintf("Kill all amux-* tmux sessions on server %q?", serverName),
	)
	u.dialog.SetSize(u.width, u.height)
	u.dialog.SetShowKeymapHints(u.config.UI.ShowKeymapHints)
	u.dialog.Show()
}

// ShowSettingsDialog shows the settings dialog.
func (u *UICompositor) ShowSettingsDialog(version string, updateAvailable *update.CheckResult, updateService UpdateService) {
	persistedUI := u.config.PersistedUISettings()
	u.settingsThemePersistedTheme = common.ThemeID(persistedUI.Theme)
	u.settingsThemeDirty = common.ThemeID(u.config.UI.Theme) != u.settingsThemePersistedTheme
	u.settingsDialogSession++
	u.settingsDialog = common.NewSettingsDialog(
		common.ThemeID(u.config.UI.Theme),
	)
	u.settingsDialog.SetSession(u.settingsDialogSession)
	u.settingsDialog.SetSize(u.width, u.height)

	// Set update state
	if updateAvailable != nil {
		u.settingsDialog.SetUpdateInfo(
			updateAvailable.CurrentVersion,
			updateAvailable.LatestVersion,
			updateAvailable.UpdateAvailable,
		)
	} else {
		u.settingsDialog.SetUpdateInfo(version, "", false)
	}
	if updateService != nil && updateService.IsHomebrewBuild() {
		u.settingsDialog.SetUpdateHint("Installed via Homebrew - update with brew upgrade amux")
	}

	u.settingsDialog.Show()
}

// ApplyTheme applies a theme to all UI components.
func (u *UICompositor) ApplyTheme(theme common.ThemeID) {
	common.SetCurrentTheme(theme)
	u.config.UI.Theme = string(theme)
	u.settingsThemeDirty = theme != u.settingsThemePersistedTheme
	u.styles = common.DefaultStyles()
	// Propagate styles to all components
	u.dashboard.SetStyles(u.styles)
	u.sidebar.SetStyles(u.styles)
	u.sidebarTerminal.SetStyles(u.styles)
	u.center.SetStyles(u.styles)
	u.toast.SetStyles(u.styles)
	if u.filePicker != nil {
		u.filePicker.SetStyles(u.styles)
	}
}

// HandleThemePreview handles live theme preview.
func (u *UICompositor) HandleThemePreview(msg common.ThemePreview) tea.Cmd {
	if msg.Session != u.settingsDialogSession {
		return nil
	}
	if u.settingsDialog != nil {
		u.settingsDialog.SetSelectedTheme(msg.Theme)
	}
	u.ApplyTheme(msg.Theme)
	return nil
}

// PersistSettingsThemeIfDirty persists the theme if it has been changed.
func (u *UICompositor) PersistSettingsThemeIfDirty() tea.Cmd {
	if !u.settingsThemeDirty {
		return nil
	}
	if err := u.config.SaveUISettings(); err != nil {
		logging.Warn("Failed to save theme setting: %v", err)
		return u.toast.ShowWarning("Failed to save theme setting")
	}
	u.settingsThemePersistedTheme = common.ThemeID(u.config.UI.Theme)
	u.settingsThemeDirty = false
	return nil
}

// HandleSettingsResult handles settings dialog close.
func (u *UICompositor) HandleSettingsResult(_ common.SettingsResult) tea.Cmd {
	if u.settingsDialog != nil {
		u.ApplyTheme(u.settingsDialog.SelectedTheme())
	}
	u.settingsDialog = nil
	u.settingsDialogSession++
	return u.PersistSettingsThemeIfDirty()
}

// HandleDialogInput handles input for the general dialog overlay.
func (u *UICompositor) HandleDialogInput(msg tea.Msg, cmds *[]tea.Cmd) bool {
	var consumed bool
	u.dialog, consumed = handleOverlayInput(u.dialog, msg, cmds, true)
	return consumed
}

// HandleFilePickerInput handles input for the file picker overlay.
func (u *UICompositor) HandleFilePickerInput(msg tea.Msg, cmds *[]tea.Cmd) bool {
	var consumed bool
	u.filePicker, consumed = handleOverlayInput(u.filePicker, msg, cmds, true)
	return consumed
}

// HandleSettingsDialogInput handles input for the settings dialog overlay.
func (u *UICompositor) HandleSettingsDialogInput(msg tea.Msg, cmds *[]tea.Cmd) bool {
	var consumed bool
	u.settingsDialog, consumed = handleOverlayInput(u.settingsDialog, msg, cmds, false)
	return consumed
}

// assistantNames returns the configured assistant names.
func (u *UICompositor) assistantNames() []string {
	if u.config != nil {
		names := u.config.AssistantNames()
		if len(names) > 0 {
			return names
		}
	}
	return []string{data.DefaultAssistant}
}

// sortTicketsHierarchically reorders tickets so that child tickets
// (tickets with a non-empty ParentID) appear immediately after
// their parent.
func sortTicketsHierarchically(ts []tickets.Ticket) []tickets.Ticket {
	if len(ts) == 0 {
		return ts
	}

	// Build parent index map: ticket ID -> position
	parentPos := make(map[string]int, len(ts))
	for i, t := range ts {
		parentPos[t.ID] = i
	}

	// Separate children from parents
	var ordered []tickets.Ticket
	childrenOf := make(map[int][]tickets.Ticket) // parent position -> children

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
