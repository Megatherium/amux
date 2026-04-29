package app

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/logging"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/ui/common"
	"github.com/andyrewlee/amux/internal/validation"
)

// ---------------------------------------------------------------------------
// Workspace lifecycle handlers
// ---------------------------------------------------------------------------

// handleCreateWorkspace handles the CreateWorkspace message.
func (wm *WorkspaceManager) HandleCreateWorkspace(msg messages.CreateWorkspace) []tea.Cmd {
	var cmds []tea.Cmd
	name := strings.TrimSpace(msg.Name)
	base := msg.Base
	assistant := strings.TrimSpace(msg.Assistant)
	if assistant == "" {
		cmds = append(cmds, func() tea.Msg {
			return messages.WorkspaceCreateFailed{Err: errors.New("assistant is required")}
		})
		return cmds
	}
	if err := validation.ValidateAssistant(assistant); err != nil {
		cmds = append(cmds, func() tea.Msg {
			return messages.WorkspaceCreateFailed{Err: err}
		})
		return cmds
	}
	if wm.isKnownAssistant != nil && !wm.isKnownAssistant(assistant) {
		cmds = append(cmds, func() tea.Msg {
			return messages.WorkspaceCreateFailed{Err: fmt.Errorf("unknown assistant: %s", assistant)}
		})
		return cmds
	}
	if msg.Project != nil && name != "" && wm.workspaceService != nil {
		pending := wm.workspaceService.pendingWorkspace(msg.Project, name, base)
		if pending != nil {
			pending.Assistant = assistant
			wm.setCreatingWorkspace(string(pending.ID()))
			if wm.dashboard != nil {
				if cmd := wm.dashboard.SetWorkspaceCreating(pending, true); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
	}
	if wm.workspaceService != nil {
		cmds = append(cmds, wm.workspaceService.CreateWorkspace(msg.Project, name, base, assistant))
	}
	return cmds
}

// HandleWorkspaceCreated handles the WorkspaceCreated message.
func (wm *WorkspaceManager) HandleWorkspaceCreated(msg messages.WorkspaceCreated) []tea.Cmd {
	var cmds []tea.Cmd
	if msg.Workspace != nil {
		wm.clearCreatingWorkspace(string(msg.Workspace.ID()))
		if wm.dashboard != nil {
			if cmd := wm.dashboard.SetWorkspaceCreating(msg.Workspace, false); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if wm.workspaceService != nil {
			cmds = append(cmds, wm.workspaceService.RunSetupAsync(msg.Workspace))
		}
	}
	if wm.workspaceService != nil {
		cmds = append(cmds, wm.workspaceService.LoadProjects())
	}
	return cmds
}

// HandleWorkspaceCreatedWithWarning handles the WorkspaceCreatedWithWarning message.
func (wm *WorkspaceManager) HandleWorkspaceCreatedWithWarning(msg messages.WorkspaceCreatedWithWarning) []tea.Cmd {
	var cmds []tea.Cmd
	if wm.setAppError != nil {
		wm.setAppError(fmt.Errorf("workspace created with warning: %s", msg.Warning))
	}
	if msg.Workspace != nil {
		wm.clearCreatingWorkspace(string(msg.Workspace.ID()))
		if wm.dashboard != nil {
			if cmd := wm.dashboard.SetWorkspaceCreating(msg.Workspace, false); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	if wm.workspaceService != nil {
		cmds = append(cmds, wm.workspaceService.LoadProjects())
	}
	return cmds
}

// HandleWorkspaceSetupComplete handles the WorkspaceSetupComplete message.
func (wm *WorkspaceManager) HandleWorkspaceSetupComplete(msg messages.WorkspaceSetupComplete) tea.Cmd {
	if msg.Err != nil {
		if wm.toast != nil {
			return wm.toast.ShowWarning(fmt.Sprintf("Setup failed for %s: %v", msg.Workspace.Name, msg.Err))
		}
	}
	return nil
}

// HandleWorkspaceCreateFailed handles the WorkspaceCreateFailed message.
func (wm *WorkspaceManager) HandleWorkspaceCreateFailed(msg messages.WorkspaceCreateFailed) tea.Cmd {
	var cmds []tea.Cmd
	if msg.Workspace != nil {
		wm.clearCreatingWorkspace(string(msg.Workspace.ID()))
		if wm.dashboard != nil {
			if cmd := wm.dashboard.SetWorkspaceCreating(msg.Workspace, false); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	if errCmd := common.ReportError(errorContext(errorServiceWorkspace, "creating workspace"), msg.Err, ""); errCmd != nil {
		cmds = append(cmds, errCmd)
	}
	return common.SafeBatch(cmds...)
}

// HandleDeleteWorkspace handles the DeleteWorkspace message.
func (wm *WorkspaceManager) HandleDeleteWorkspace(msg messages.DeleteWorkspace) []tea.Cmd {
	var cmds []tea.Cmd
	if msg.Project == nil || msg.Workspace == nil {
		logging.Warn("DeleteWorkspace received with nil project or workspace")
		return nil
	}
	wm.markWorkspaceDeleteInFlight(msg.Workspace, true)
	if wm.cleanupTmuxSessions != nil {
		if cleanup := wm.cleanupTmuxSessions(msg.Workspace); cleanup != nil {
			cmds = append(cmds, cleanup)
		}
	}
	if wm.dashboard != nil {
		if cmd := wm.dashboard.SetWorkspaceDeleting(msg.Workspace.Root, true); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if wm.deleteWorkspace != nil {
		cmds = append(cmds, wm.deleteWorkspace(msg.Project, msg.Workspace))
	}
	return cmds
}

// HandleWorkspaceDeleted handles the WorkspaceDeleted message.
func (wm *WorkspaceManager) HandleWorkspaceDeleted(msg messages.WorkspaceDeleted) []tea.Cmd {
	var cmds []tea.Cmd
	if msg.Workspace != nil {
		wm.markWorkspaceDeleteInFlight(msg.Workspace, false)
		wm.clearWorkspaceDirty(string(msg.Workspace.ID()))
		if wm.cleanupTmuxSessions != nil {
			if cleanup := wm.cleanupTmuxSessions(msg.Workspace); cleanup != nil {
				cmds = append(cmds, cleanup)
			}
		}
		if wm.dashboard != nil {
			if cmd := wm.dashboard.SetWorkspaceDeleting(msg.Workspace.Root, false); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if wm.gitStatus != nil {
			wm.gitStatus.Invalidate(msg.Workspace.Root)
		}
		if wm.center != nil {
			newCenter, cmd := wm.center.Update(msg)
			wm.center = newCenter
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if wm.sidebarTerminal != nil {
			newTerminal, cmd := wm.sidebarTerminal.Update(msg)
			wm.sidebarTerminal = newTerminal
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	if wm.workspaceService != nil {
		cmds = append(cmds, wm.workspaceService.LoadProjects())
	}
	return cmds
}

// HandleWorkspaceDeleteFailed handles the WorkspaceDeleteFailed message.
func (wm *WorkspaceManager) HandleWorkspaceDeleteFailed(msg messages.WorkspaceDeleteFailed) tea.Cmd {
	var cmds []tea.Cmd
	if msg.Workspace != nil {
		// Ordering is intentional: clear delete-in-flight first so the
		// persistence requeue below is not suppressed.
		wm.markWorkspaceDeleteInFlight(msg.Workspace, false)
		if wm.dashboard != nil {
			if cmd := wm.dashboard.SetWorkspaceDeleting(msg.Workspace.Root, false); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if wm.persistWorkspaceTabs != nil {
			if cmd := wm.persistWorkspaceTabs(string(msg.Workspace.ID())); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	if errCmd := common.ReportError(errorContext(errorServiceWorkspace, "removing workspace"), msg.Err, ""); errCmd != nil {
		cmds = append(cmds, errCmd)
	}
	return common.SafeBatch(cmds...)
}

// HandlePersistDebounce handles the persistDebounceMsg message.
func (wm *WorkspaceManager) HandlePersistDebounce(msg persistDebounceMsg) tea.Cmd {
	// Ignore stale tokens (newer persist request superseded this one)
	if msg.token != wm.currentPersistToken() {
		return nil
	}
	if wm.center == nil || wm.workspaceService == nil {
		return nil
	}
	if wm.dirtyWorkspaceCount() == 0 {
		return nil
	}

	// Collect snapshots for all dirty workspaces
	dirty := wm.dirtyWorkspaceIDs()
	var snapshots []*data.Workspace
	processed := make(map[string]bool, len(dirty))
	for wsID := range dirty {
		if wm.isWorkspaceDeleteInFlight(wsID) {
			// Keep dirty marker while delete is in flight. If delete fails, the
			// marker must remain so pending workspace state can still be saved.
			continue
		}
		ws := wm.findWorkspaceByID(wsID)
		if ws == nil {
			processed[wsID] = true
			continue
		}
		// Update in-memory state from center tabs
		tabs, activeIdx := wm.center.GetTabsInfoForWorkspace(wsID)
		ws.OpenTabs = tabs
		ws.ActiveTabIndex = activeIdx
		snapshots = append(snapshots, snapshotWorkspaceForSave(ws))
		processed[wsID] = true
	}
	// Clear only workspaces processed above; keep in-flight delete markers dirty.
	for wsID := range processed {
		wm.clearWorkspaceDirty(wsID)
	}

	if len(snapshots) == 0 {
		return nil
	}
	service := wm.workspaceService
	return func() tea.Msg {
		for _, snap := range snapshots {
			wsID := string(snap.ID())
			var saveErr error
			saved := wm.runUnlessWorkspaceDeleteInFlight(wsID, func() {
				saveErr = service.Save(snap)
			})
			if !saved {
				continue
			}
			if saveErr != nil {
				logging.Warn("Failed to save workspace tabs: %v", saveErr)
			} else {
				// Marker bookkeeping is intentionally outside delete-state guard.
				// Delete safety is enforced by the guarded Save above.
				wm.markLocalWorkspaceSaveForID(wm.metadataRoot, wsID)
			}
		}
		return nil
	}
}

// HandleTabDetached handles the TabDetached message.
func (wm *WorkspaceManager) HandleTabDetached(msg messages.TabDetached) tea.Cmd {
	if msg.WorkspaceID != "" {
		if wm.persistWorkspaceTabs != nil {
			return wm.persistWorkspaceTabs(msg.WorkspaceID)
		}
		return nil
	}
	if wm.persistActiveTabs != nil {
		return wm.persistActiveTabs()
	}
	return nil
}
