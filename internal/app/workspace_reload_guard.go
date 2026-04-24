package app

import (
	"time"
)

type workspaceFileFingerprint struct {
	modTimeUnixNano int64
	size            int64
	digest          [32]byte
}

type localWorkspaceSaveMarker struct {
	at          time.Time
	fingerprint workspaceFileFingerprint
}

func (a *App) markLocalWorkspaceSaveForID(wsID string) {
	metadataRoot := ""
	if a != nil && a.config != nil && a.config.Paths != nil {
		metadataRoot = a.config.Paths.MetadataRoot
	}
	a.wm().markLocalWorkspaceSaveForID(metadataRoot, wsID)
}

func (a *App) markLocalWorkspaceSavePath(path string) {
	a.wm().markLocalWorkspaceSavePath(path)
}

func (a *App) shouldSuppressWorkspaceReload(paths []string, now time.Time) bool {
	return a.wm().shouldSuppressWorkspaceReload(paths, now)
}

func pruneOldLocalWorkspaceSavesLocked(saves map[string]localWorkspaceSaveMarker, now time.Time) {
	for path, marker := range saves {
		if marker.at.IsZero() || now.Sub(marker.at) > localWorkspaceReloadSuppressWindow || now.Before(marker.at) {
			delete(saves, path)
		}
	}
}
