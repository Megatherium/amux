package workspaces

import "time"

type workspaceFileFingerprint struct {
	modTimeUnixNano int64
	size            int64
	digest          [32]byte
}

type localWorkspaceSaveMarker struct {
	at          time.Time
	fingerprint workspaceFileFingerprint
}

func pruneOldLocalWorkspaceSavesLocked(saves map[string]localWorkspaceSaveMarker, now time.Time) {
	for path, marker := range saves {
		if marker.at.IsZero() || now.Sub(marker.at) > localWorkspaceReloadSuppressWindow || now.Before(marker.at) {
			delete(saves, path)
		}
	}
}
