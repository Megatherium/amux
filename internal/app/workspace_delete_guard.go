package app

import "github.com/andyrewlee/amux/internal/data"

func (a *App) markWorkspaceDeleteInFlight(ws *data.Workspace, deleting bool) {
	a.wm().MarkWorkspaceDeleteInFlight(ws, deleting)
}

func (a *App) isWorkspaceDeleteInFlight(wsID string) bool {
	return a.wm().IsWorkspaceDeleteInFlight(wsID)
}

// runUnlessWorkspaceDeleteInFlight runs fn while holding a shared delete-state
// lock only when wsID is not currently marked delete-in-flight. Holding the
// lock across fn keeps the check and side effect atomic with respect to
// markWorkspaceDeleteInFlight updates.
func (a *App) runUnlessWorkspaceDeleteInFlight(wsID string, fn func()) bool {
	return a.wm().RunUnlessWorkspaceDeleteInFlight(wsID, fn)
}
