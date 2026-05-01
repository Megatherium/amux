package app

import "time"

func (a *App) markLocalWorkspaceSaveForID(wsID string) {
	metadataRoot := ""
	if a != nil && a.config != nil && a.config.Paths != nil {
		metadataRoot = a.config.Paths.MetadataRoot
	}
	a.wm().MarkLocalWorkspaceSaveForID(metadataRoot, wsID)
}

func (a *App) markLocalWorkspaceSavePath(path string) {
	a.wm().MarkLocalWorkspaceSavePath(path)
}

func (a *App) shouldSuppressWorkspaceReload(paths []string, now time.Time) bool {
	return a.wm().ShouldSuppressWorkspaceReload(paths, now)
}
