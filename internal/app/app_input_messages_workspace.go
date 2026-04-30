package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/validation"
)

// handleProjectsLoaded processes the ProjectsLoaded message.
func (a *App) handleProjectsLoaded(msg messages.ProjectsLoaded) []tea.Cmd {
	a.projects = msg.Projects
	a.projectsLoaded = true
	var cmds []tea.Cmd
	if a.ui.dashboard != nil {
		a.ui.dashboard.SetProjects(a.projects)
	}
	cmds = append(cmds, a.rebindActiveSelection()...)
	// Request git status for all workspaces
	cmds = append(cmds, a.scanTmuxActivityNow())
	if gcCmd := a.gcOrphanedTmuxSessions(); gcCmd != nil {
		cmds = append(cmds, gcCmd)
	}
	if countCmd := a.logSessionCount(); countCmd != nil {
		cmds = append(cmds, countCmd)
	}
	for i := range a.projects {
		for j := range a.projects[i].Workspaces {
			ws := &a.projects[i].Workspaces[j]
			cmds = append(cmds, a.gitStatusController.requestGitStatus(ws.Root))
		}
	}
	cmds = append(cmds, a.initTicketStoresForLoadedProjects()...)
	return cmds
}

//nolint:cyclop,funlen // legacy suppression
func (a *App) rebindActiveSelection() []tea.Cmd {
	var cmds []tea.Cmd
	if a.activeWorkspace != nil {
		previous := a.activeWorkspace
		wsID := string(a.activeWorkspace.ID())
		ws, project := a.findWorkspaceAndProjectByID(wsID)
		if ws == nil {
			ws, project = a.findWorkspaceAndProjectByCanonicalPaths(previous.Repo, previous.Root)
		}
		if ws == nil {
			a.goHome()
			a.activeProject = nil
			return cmds
		}
		oldID := string(previous.ID())
		newID := string(ws.ID())
		hadPreviousWorkspaceState := false
		if a.ui.center != nil {
			hadPreviousWorkspaceState = a.ui.center.HasWorkspaceState(oldID)
		}
		if oldID != newID {
			a.migrateDirtyWorkspaceID(oldID, newID)
			cmds = append(cmds, a.rebindActiveWorkspaceWatch(previous.Root, ws.Root)...)
			if a.ui.center != nil {
				if cmd := a.ui.center.RebindWorkspaceID(previous, ws); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
			if a.ui.sidebarTerminal != nil {
				if cmd := a.ui.sidebarTerminal.RebindWorkspaceID(previous, ws); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
		a.activeWorkspace = ws
		a.activeProject = project
		if a.ui.center != nil {
			a.ui.center.SetWorkspace(ws)
			wsIDCurrent := string(ws.ID())
			hasWorkspaceState := a.ui.center.HasWorkspaceState(wsIDCurrent)
			existingTabs, _ := a.ui.center.GetTabsInfoForWorkspace(wsIDCurrent)
			hasLiveWorkspaceTabs := len(existingTabs) > 0
			shouldHydrateTabs := !hasWorkspaceState || hasLiveWorkspaceTabs
			if shouldHydrateTabs && oldID != newID && hadPreviousWorkspaceState {
				shouldHydrateTabs = false
			}
			if shouldHydrateTabs && a.wm().isWorkspaceDirty(wsIDCurrent) {
				shouldHydrateTabs = false
			}
			if shouldHydrateTabs {
				if cmd := a.ui.center.AddTabsFromWorkspace(ws, ws.OpenTabs); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
		if a.ui.sidebar != nil {
			a.ui.sidebar.SetWorkspace(ws)
		}
		if a.ui.sidebarTerminal != nil {
			a.ui.sidebarTerminal.SetWorkspacePreview(ws)
		}
		return cmds
	}
	if a.activeProject != nil {
		a.activeProject = a.findProjectByPath(a.activeProject.Path)
	}
	return cmds
}

func (a *App) rebindActiveWorkspaceWatch(previousRoot, currentRoot string) []tea.Cmd {
	var cmds []tea.Cmd
	oldRoot := strings.TrimSpace(previousRoot)
	newRoot := strings.TrimSpace(currentRoot)
	if oldRoot == "" || newRoot == "" || oldRoot == newRoot {
		return cmds
	}

	if a.gitStatusController != nil {
		a.gitStatusController.unwatchRoot(oldRoot)
		if err := a.gitStatusController.watchRoot(newRoot); err != nil {
			if a.gitStatusController.isWatchLimitReached() {
				if a.ui.toast != nil {
					cmds = append(cmds, a.ui.toast.ShowWarning("File watching disabled (watch limit reached); git status may be stale"))
				}
			}
		}
	}

	if a.gitStatus != nil {
		a.gitStatus.Invalidate(oldRoot)
		a.gitStatus.Invalidate(newRoot)
	}
	if a.ui.dashboard != nil {
		a.ui.dashboard.InvalidateStatus(oldRoot)
		a.ui.dashboard.InvalidateStatus(newRoot)
	}

	return cmds
}

func rootsReferToSameWorkspace(left, right string) bool {
	leftTrimmed := strings.TrimSpace(left)
	rightTrimmed := strings.TrimSpace(right)
	if leftTrimmed == "" || rightTrimmed == "" {
		return false
	}
	if leftTrimmed == rightTrimmed {
		return true
	}
	return canonicalPathForMatch(leftTrimmed) == canonicalPathForMatch(rightTrimmed)
}

func (a *App) findWorkspaceAndProjectByID(id string) (*data.Workspace, *data.Project) {
	if id == "" {
		return nil, nil
	}
	for i := range a.projects {
		project := &a.projects[i]
		for j := range project.Workspaces {
			ws := &project.Workspaces[j]
			if string(ws.ID()) == id {
				return ws, project
			}
		}
	}
	return nil, nil
}

func (a *App) findWorkspaceAndProjectByCanonicalPaths(repoPath, rootPath string) (*data.Workspace, *data.Project) {
	targetRepo := canonicalPathForMatch(repoPath)
	targetRoot := canonicalPathForMatch(rootPath)
	if targetRepo == "" && targetRoot == "" {
		return nil, nil
	}
	for i := range a.projects {
		project := &a.projects[i]
		for j := range project.Workspaces {
			ws := &project.Workspaces[j]
			repoCanonical := canonicalPathForMatch(ws.Repo)
			rootCanonical := canonicalPathForMatch(ws.Root)
			if targetRoot != "" && rootCanonical != targetRoot {
				continue
			}
			if targetRepo != "" && repoCanonical != targetRepo {
				continue
			}
			if targetRoot == "" && targetRepo != "" && repoCanonical != targetRepo {
				continue
			}
			return ws, project
		}
	}
	return nil, nil
}

func (a *App) findProjectByPath(path string) *data.Project {
	if path == "" {
		return nil
	}
	targetCanonical := canonicalProjectPathForMatch(path)
	for i := range a.projects {
		project := &a.projects[i]
		if project.Path == path {
			return project
		}
		if targetCanonical == "" {
			continue
		}
		if canonicalProjectPathForMatch(project.Path) == targetCanonical {
			return project
		}
	}
	return nil
}

func canonicalProjectPathForMatch(path string) string {
	return canonicalPathForMatch(path)
}

func canonicalPathForMatch(path string) string {
	value := strings.TrimSpace(path)
	if value == "" {
		return ""
	}
	cleaned := filepath.Clean(value)
	if abs, err := filepath.Abs(cleaned); err == nil {
		cleaned = abs
	}
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		cleaned = resolved
	}
	return filepath.Clean(cleaned)
}

// handleWorkspaceActivated processes the WorkspaceActivated message.
func (a *App) handleWorkspaceActivated(msg messages.WorkspaceActivated) []tea.Cmd {
	var cmds []tea.Cmd
	a.setWorkspaceActivationState(msg)
	cmds = append(cmds, a.discoverWorkspaceTmux(msg.Workspace)...)
	centerFocusQueuedReattach := a.routeFocusOnActivation(msg, &cmds)
	// Sync active workspaces to dashboard (fixes spinner race condition)
	a.syncActiveWorkspacesToDashboard()
	newDashboard, cmd := a.ui.dashboard.Update(msg)
	a.ui.dashboard = newDashboard
	cmds = append(cmds, cmd)
	cmds = append(cmds, a.refreshWorkspaceResources(msg.Workspace)...)
	// Ensure spinner starts if needed after sync
	if startCmd := a.ui.dashboard.StartSpinnerIfNeeded(); startCmd != nil {
		cmds = append(cmds, startCmd)
	}
	// Seamless UX: if restored active tab is detached, auto-reattach on workspace activation.
	if !centerFocusQueuedReattach {
		cmds = append(cmds, a.ui.center.ReattachActiveTabIfDetached())
	}
	cmds = append(cmds, a.enforceAttachedAgentTabLimit()...)
	return cmds
}

// handleCreateWorkspace handles the CreateWorkspace message.
func (a *App) handleCreateWorkspace(msg messages.CreateWorkspace) []tea.Cmd {
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
	if !a.isKnownAssistant(assistant) {
		cmds = append(cmds, func() tea.Msg {
			return messages.WorkspaceCreateFailed{Err: fmt.Errorf("unknown assistant: %s", assistant)}
		})
		return cmds
	}
	if msg.Project != nil && name != "" && a.workspaceService != nil {
		pending := a.workspaceService.pendingWorkspace(msg.Project, name, base)
		if pending != nil {
			pending.Assistant = assistant
			a.wm().setCreatingWorkspace(string(pending.ID()))
			if cmd := a.ui.dashboard.SetWorkspaceCreating(pending, true); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	cmds = append(cmds, a.createWorkspace(msg.Project, name, base, assistant))
	return cmds
}
