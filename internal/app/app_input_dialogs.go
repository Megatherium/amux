package app

import (
	"errors"
	"reflect"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/logging"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/tickets"
	"github.com/andyrewlee/amux/internal/ui/common"
	"github.com/andyrewlee/amux/internal/ui/sidebar"
	"github.com/andyrewlee/amux/internal/update"
	"github.com/andyrewlee/amux/internal/validation"
)

func (a *App) handleDialogResultMsg(msg tea.Msg) (bool, tea.Cmd) {
	result, ok := msg.(common.DialogResult)
	if !ok {
		return false, nil
	}
	logging.Info("Received DialogResult: id=%s confirmed=%v", result.ID, result.Confirmed)
	switch result.ID {
	case DialogAddProject, DialogCreateWorkspace, DialogDeleteWorkspace, DialogRemoveProject, DialogSelectAssistant, "agent-picker", DialogSelectTicket, "ticket-picker", DialogQuit, DialogCleanupTmux:
		return true, common.SafeCmd(a.handleDialogResult(result))
	}
	// If not an App-level dialog, let it fall through to components.
	newCenter, cmd := a.ui.center.Update(msg)
	a.ui.center = newCenter
	return true, common.SafeCmd(cmd)
}

func (a *App) handleErrorOverlayDismiss(msg tea.Msg) bool {
	mouseMsg, ok := msg.(tea.MouseClickMsg)
	if !ok || mouseMsg.Button != tea.MouseLeft {
		return false
	}
	if a.err == nil {
		return false
	}
	a.err = nil
	return true
}

// handleOverlayInput updates a modal overlay and reports whether the message was consumed.
// When consumePaste is true, tea.PasteMsg is treated as consumed input.
func handleOverlayInput[T interface {
	Visible() bool
	Update(tea.Msg) (T, tea.Cmd)
}](overlay T, msg tea.Msg, cmds *[]tea.Cmd, consumePaste bool) (T, bool) {
	if isNilOverlay(overlay) || !overlay.Visible() {
		return overlay, false
	}
	updated, cmd := overlay.Update(msg)
	if cmd != nil {
		*cmds = append(*cmds, cmd)
	}
	switch msg.(type) {
	case tea.KeyPressMsg, tea.MouseClickMsg:
		return updated, true
	case tea.PasteMsg:
		return updated, consumePaste
	}
	return updated, false
}

func isNilOverlay[T any](overlay T) bool {
	v := reflect.ValueOf(overlay)
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func (a *App) handleDialogInput(msg tea.Msg, cmds *[]tea.Cmd) bool {
	var consumed bool
	a.ui.dialog, consumed = handleOverlayInput(a.ui.dialog, msg, cmds, true)
	return consumed
}

func (a *App) handleFilePickerInput(msg tea.Msg, cmds *[]tea.Cmd) bool {
	var consumed bool
	a.ui.filePicker, consumed = handleOverlayInput(a.ui.filePicker, msg, cmds, true)
	return consumed
}

func (a *App) handleSettingsDialogInput(msg tea.Msg, cmds *[]tea.Cmd) bool {
	var consumed bool
	a.ui.settingsDialog, consumed = handleOverlayInput(a.ui.settingsDialog, msg, cmds, false)
	return consumed
}

// handleDialogResult handles dialog completion
func (a *App) handleDialogResult(result common.DialogResult) tea.Cmd {
	project := a.ui.dialogProject
	workspace := a.ui.dialogWorkspace
	a.ui.dialog = nil
	a.ui.dialogProject = nil
	a.ui.dialogWorkspace = nil
	logging.Debug("Dialog result: id=%s confirmed=%v value=%s", result.ID, result.Confirmed, result.Value)

	if !result.Confirmed {
		if result.ID == DialogSelectAssistant || result.ID == "agent-picker" {
			a.ui.pendingWorkspaceProject = nil
			a.ui.pendingWorkspaceName = ""
			a.ui.pendingWorkspaceBase = ""
		}
		if result.ID == DialogSelectTicket || result.ID == "ticket-picker" {
			a.ui.pendingTickets = nil
		}
		logging.Debug("Dialog canceled")
		return nil
	}

	switch result.ID {
	case DialogAddProject:
		if result.Value != "" {
			path := validation.SanitizeInput(result.Value)
			logging.Info("Adding project from dialog: %s", path)
			if err := validation.ValidateProjectPath(path); err != nil {
				logging.Warn("Project path validation failed: %v", err)
				return func() tea.Msg {
					return messages.Error{Err: err, Context: errorContext(errorServiceDialog, "validating project path")}
				}
			}
			return func() tea.Msg {
				return messages.AddProject{Path: path}
			}
		}

	case DialogCreateWorkspace:
		if result.Value != "" && project != nil {
			name := validation.SanitizeInput(result.Value)
			if err := validation.ValidateWorkspaceName(name); err != nil {
				return func() tea.Msg {
					return messages.Error{Err: err, Context: errorContext(errorServiceDialog, "validating workspace name")}
				}
			}
			a.ui.pendingWorkspaceProject = project
			a.ui.pendingWorkspaceName = name
			a.ui.pendingWorkspaceBase = ""
			return func() tea.Msg {
				return messages.ShowSelectAssistantDialog{}
			}
		}

	case DialogDeleteWorkspace:
		if project != nil && workspace != nil {
			ws := workspace
			return func() tea.Msg {
				return messages.DeleteWorkspace{
					Project:   project,
					Workspace: ws,
				}
			}
		}

	case DialogRemoveProject:
		if project != nil {
			proj := project
			return func() tea.Msg {
				return messages.RemoveProject{
					Project: proj,
				}
			}
		}

	case DialogSelectAssistant, "agent-picker":
		assistant := result.Value
		if err := validation.ValidateAssistant(assistant); err != nil {
			return func() tea.Msg {
				return messages.Error{Err: err, Context: errorContext(errorServiceDialog, "validating assistant")}
			}
		}
		if !a.isKnownAssistant(assistant) {
			return func() tea.Msg {
				return messages.Error{Err: errors.New("unknown assistant: " + assistant), Context: errorContext(errorServiceDialog, "validating assistant")}
			}
		}
		if a.ui.pendingWorkspaceProject != nil && a.ui.pendingWorkspaceName != "" {
			pendingProject := a.ui.pendingWorkspaceProject
			pendingName := a.ui.pendingWorkspaceName
			pendingBase := a.ui.pendingWorkspaceBase
			a.ui.pendingWorkspaceProject = nil
			a.ui.pendingWorkspaceName = ""
			a.ui.pendingWorkspaceBase = ""
			return func() tea.Msg {
				return messages.CreateWorkspace{
					Project:   pendingProject,
					Name:      pendingName,
					Base:      pendingBase,
					Assistant: assistant,
				}
			}
		}
		if a.activeWorkspace != nil {
			ws := a.activeWorkspace
			return func() tea.Msg {
				return messages.LaunchAgent{
					Assistant: assistant,
					Workspace: ws,
				}
			}
		}

	case DialogSelectTicket, "ticket-picker":
		var ticket *tickets.Ticket
		if result.Index < len(a.ui.pendingTickets) {
			ticket = &a.ui.pendingTickets[result.Index]
		}
		a.ui.pendingTickets = nil
		if ticket == nil {
			return func() tea.Msg { return messages.ShowSelectAssistantDialog{} }
		}
		return func() tea.Msg {
			return messages.TicketSelectedMsg{
				Ticket:  ticket,
				Project: a.activeProject,
			}
		}

	case DialogQuit:
		// Persist workspace tabs synchronously before shutdown.
		// Shutdown() closes tabs (sets Running=false), so we must
		// capture current state first to avoid saving "stopped" status.
		a.persistAllWorkspacesNow()
		a.Shutdown()
		a.quitting = true
		return tea.Quit

	case DialogCleanupTmux:
		return func() tea.Msg { return messages.CleanupTmuxSessions{} }
	}

	return nil
}

func (a *App) showQuitDialog() {
	if a.ui.dialog != nil && a.ui.dialog.Visible() {
		return
	}
	a.ui.dialog = common.NewConfirmDialog(
		DialogQuit,
		"Quit AMUX",
		"Are you sure you want to quit?",
	)
	a.ui.dialog.SetSize(a.width, a.height)
	a.ui.dialog.SetShowKeymapHints(a.config.UI.ShowKeymapHints)
	a.ui.dialog.Show()
}

// handleUpdateCheckComplete handles the UpdateCheckComplete message.
func (a *App) handleUpdateCheckComplete(msg messages.UpdateCheckComplete) tea.Cmd {
	if msg.Err != nil {
		logging.Debug("Update check error: %v", msg.Err)
		return nil
	}
	if !msg.UpdateAvailable {
		logging.Debug("No update available (current=%s, latest=%s)", msg.CurrentVersion, msg.LatestVersion)
		return nil
	}
	// Store update info
	a.updateAvailable = &update.CheckResult{
		CurrentVersion:  msg.CurrentVersion,
		LatestVersion:   msg.LatestVersion,
		UpdateAvailable: msg.UpdateAvailable,
		ReleaseNotes:    msg.ReleaseNotes,
	}
	logging.Info("Update available: %s -> %s", msg.CurrentVersion, msg.LatestVersion)
	// Update settings dialog if visible
	if a.ui.settingsDialog != nil && a.ui.settingsDialog.Visible() {
		a.ui.settingsDialog.SetUpdateInfo(msg.CurrentVersion, msg.LatestVersion, true)
	}
	return nil
}

// handleTriggerUpgrade handles the TriggerUpgrade message.
func (a *App) handleTriggerUpgrade() tea.Cmd {
	if a.ui.settingsDialog != nil {
		a.applyTheme(a.ui.settingsDialog.SelectedTheme())
		a.ui.settingsDialog = nil
		a.settingsDialogSession++
	}
	persistCmd := a.persistSettingsThemeIfDirty()
	if a.updateAvailable == nil || a.upgradeRunning {
		return persistCmd
	}
	a.upgradeRunning = true
	svc := a.updateService
	upgradeCmd := func() tea.Msg {
		if svc == nil {
			return messages.UpgradeComplete{Err: errors.New("update service unavailable")}
		}
		// Get the latest release
		result, err := svc.Check()
		if err != nil {
			return messages.UpgradeComplete{Err: err}
		}
		if result.Release == nil {
			return messages.UpgradeComplete{Err: errors.New("no release found")}
		}
		// Perform the upgrade
		if err := svc.Upgrade(result.Release); err != nil {
			return messages.UpgradeComplete{Err: err}
		}
		return messages.UpgradeComplete{NewVersion: result.Release.TagName}
	}
	return common.SafeBatch(persistCmd, upgradeCmd)
}

// handleUpgradeComplete handles the UpgradeComplete message.
func (a *App) handleUpgradeComplete(msg messages.UpgradeComplete) tea.Cmd {
	a.upgradeRunning = false
	if msg.Err != nil {
		logging.Error("Upgrade failed: %v", msg.Err)
		return a.ui.toast.ShowError("Upgrade failed: " + msg.Err.Error())
	}
	a.updateAvailable = nil
	// Update settings dialog if visible
	if a.ui.settingsDialog != nil && a.ui.settingsDialog.Visible() {
		a.ui.settingsDialog.SetUpdateInfo(msg.NewVersion, "", false)
	}
	logging.Info("Upgrade complete: %s", msg.NewVersion)
	return a.ui.toast.ShowSuccess("Upgraded to " + msg.NewVersion + " - restart amux to use new version")
}

// handleOpenFileInEditor handles the OpenFileInEditor message from the project tree.
// This opens the file in vim in the center pane.
func (a *App) handleOpenFileInEditor(msg sidebar.OpenFileInEditor) tea.Cmd {
	if msg.Workspace == nil || msg.Path == "" {
		return nil
	}
	logging.Info("Opening file in editor: %s", msg.Path)
	newCenter, cmd := a.ui.center.Update(messages.OpenFileInVim{
		Path:      msg.Path,
		Workspace: msg.Workspace,
	})
	a.ui.center = newCenter
	return cmd
}
