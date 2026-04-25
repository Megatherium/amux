package app

import "github.com/andyrewlee/amux/internal/perf"

// Shutdown releases resources that may outlive the Bubble Tea program.
func (a *App) Shutdown() {
	a.shutdownOnce.Do(func() {
		if a.supervisor != nil {
			a.supervisor.Stop()
		}
		if a.gitStatusController != nil {
			a.gitStatusController.Shutdown()
		}
		if a.center != nil {
			a.center.Close()
		}
		if a.sidebarTerminal != nil {
			a.sidebarTerminal.CloseAll()
		}
		if a.workspaceService != nil {
			a.workspaceService.StopAll()
		}
		if a.doltStores != nil {
			for _, store := range a.doltStores {
				_ = store.Close()
			}
		}
		perf.Flush("shutdown")
	})
}
