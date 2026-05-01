package workspaces

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/andyrewlee/amux/internal/data"
)

// Manager encapsulates workspace persistence, dirty tracking,
// creation tracking, and delete-in-flight guards.
type Manager struct {
	// Workspace persistence debounce
	dirtyWorkspaces       map[string]bool
	deletingWorkspaceMu   sync.RWMutex
	deletingWorkspaceIDs  map[string]bool
	persistToken          int
	localWorkspaceSaveMu  sync.Mutex
	localWorkspaceSavesAt map[string]localWorkspaceSaveMarker

	// Workspaces in creation flow (not yet loaded into projects list)
	creatingWorkspaceIDs map[string]bool
}

// NewManager creates an initialized Manager.
func NewManager() *Manager {
	return &Manager{
		dirtyWorkspaces:       make(map[string]bool),
		deletingWorkspaceIDs:  make(map[string]bool),
		localWorkspaceSavesAt: make(map[string]localWorkspaceSaveMarker),
		creatingWorkspaceIDs:  make(map[string]bool),
	}
}

// ---------------------------------------------------------------------------
// Dirty workspace tracking
// ---------------------------------------------------------------------------

// IsWorkspaceDirty reports whether the given workspace ID has unsaved changes.
func (wm *Manager) IsWorkspaceDirty(wsID string) bool {
	return wm.dirtyWorkspaces != nil && wm.dirtyWorkspaces[wsID]
}

// MarkWorkspaceDirty marks a workspace as having unsaved changes.
func (wm *Manager) MarkWorkspaceDirty(wsID string) {
	if wm.dirtyWorkspaces == nil {
		wm.dirtyWorkspaces = make(map[string]bool)
	}
	wm.dirtyWorkspaces[wsID] = true
}

// ClearWorkspaceDirty removes a workspace from the dirty set.
func (wm *Manager) ClearWorkspaceDirty(wsID string) {
	delete(wm.dirtyWorkspaces, wsID)
}

// ClearAllDirty removes all workspaces from the dirty set.
func (wm *Manager) ClearAllDirty() {
	for k := range wm.dirtyWorkspaces {
		delete(wm.dirtyWorkspaces, k)
	}
}

// DirtyWorkspaceCount returns the number of dirty workspaces.
func (wm *Manager) DirtyWorkspaceCount() int {
	return len(wm.dirtyWorkspaces)
}

// DirtyWorkspaceIDs returns a copy of the current dirty workspace ID set.
func (wm *Manager) DirtyWorkspaceIDs() map[string]bool {
	out := make(map[string]bool, len(wm.dirtyWorkspaces))
	for k := range wm.dirtyWorkspaces {
		out[k] = true
	}
	return out
}

// MigrateDirtyWorkspaceID migrates a dirty marker from oldID to newID.
func (wm *Manager) MigrateDirtyWorkspaceID(oldID, newID string) {
	if oldID == "" || newID == "" || oldID == newID {
		return
	}
	if wm.dirtyWorkspaces == nil || !wm.dirtyWorkspaces[oldID] {
		return
	}
	wm.dirtyWorkspaces[newID] = true
	delete(wm.dirtyWorkspaces, oldID)
}

// NextPersistToken increments the persist token counter and returns the new value.
func (wm *Manager) NextPersistToken() int {
	wm.persistToken++
	return wm.persistToken
}

// CurrentPersistToken returns the current persist token value.
func (wm *Manager) CurrentPersistToken() int {
	return wm.persistToken
}

// ---------------------------------------------------------------------------
// Delete-in-flight guard
// ---------------------------------------------------------------------------

// MarkWorkspaceDeleteInFlight sets or clears the delete-in-flight marker for a workspace.
func (wm *Manager) MarkWorkspaceDeleteInFlight(ws *data.Workspace, deleting bool) {
	wm.deletingWorkspaceMu.Lock()
	defer wm.deletingWorkspaceMu.Unlock()

	if ws == nil {
		return
	}
	wsID := string(ws.ID())
	if wsID == "" {
		return
	}
	if wm.deletingWorkspaceIDs == nil {
		wm.deletingWorkspaceIDs = make(map[string]bool)
	}
	if deleting {
		wm.deletingWorkspaceIDs[wsID] = true
		return
	}
	delete(wm.deletingWorkspaceIDs, wsID)
}

// IsWorkspaceDeleteInFlight reports whether the workspace is currently being deleted.
func (wm *Manager) IsWorkspaceDeleteInFlight(wsID string) bool {
	wm.deletingWorkspaceMu.RLock()
	defer wm.deletingWorkspaceMu.RUnlock()

	if wsID == "" || wm.deletingWorkspaceIDs == nil {
		return false
	}
	return wm.deletingWorkspaceIDs[wsID]
}

// RunUnlessWorkspaceDeleteInFlight runs fn while holding a shared delete-state
// lock only when wsID is not currently marked delete-in-flight. Holding the
// lock across fn keeps the check and side effect atomic with respect to
// MarkWorkspaceDeleteInFlight updates.
func (wm *Manager) RunUnlessWorkspaceDeleteInFlight(wsID string, fn func()) bool {
	wm.deletingWorkspaceMu.RLock()
	defer wm.deletingWorkspaceMu.RUnlock()

	if wsID == "" || wm.deletingWorkspaceIDs[wsID] {
		return false
	}
	if fn != nil {
		fn()
	}
	return true
}

// ---------------------------------------------------------------------------
// Creation tracking
// ---------------------------------------------------------------------------

// SetCreatingWorkspace marks a workspace as being created.
func (wm *Manager) SetCreatingWorkspace(wsID string) {
	wm.creatingWorkspaceIDs[wsID] = true
}

// ClearCreatingWorkspace removes a workspace from the creation tracking set.
func (wm *Manager) ClearCreatingWorkspace(wsID string) {
	delete(wm.creatingWorkspaceIDs, wsID)
}

// CreatingWorkspaceIDSet returns the full map of creating workspace IDs.
func (wm *Manager) CreatingWorkspaceIDSet() map[string]bool {
	return wm.creatingWorkspaceIDs
}

// ---------------------------------------------------------------------------
// Local workspace save markers (reload suppression)
// ---------------------------------------------------------------------------

// MarkLocalWorkspaceSaveForID records a save marker for the given workspace ID.
func (wm *Manager) MarkLocalWorkspaceSaveForID(metadataRoot, wsID string) {
	path := workspaceMetadataPath(metadataRoot, wsID)
	if path == "" {
		return
	}
	wm.MarkLocalWorkspaceSavePath(path)
}

// MarkLocalWorkspaceSavePath records a save marker for the given file path.
func (wm *Manager) MarkLocalWorkspaceSavePath(path string) {
	normalized := filepath.Clean(strings.TrimSpace(path))
	if normalized == "" {
		return
	}
	fingerprint, ok := workspaceMetadataFingerprint(normalized)
	if !ok {
		return
	}
	now := time.Now()
	wm.localWorkspaceSaveMu.Lock()
	if wm.localWorkspaceSavesAt == nil {
		wm.localWorkspaceSavesAt = make(map[string]localWorkspaceSaveMarker)
	}
	pruneOldLocalWorkspaceSavesLocked(wm.localWorkspaceSavesAt, now)
	wm.localWorkspaceSavesAt[normalized] = localWorkspaceSaveMarker{
		at:          now,
		fingerprint: fingerprint,
	}
	wm.localWorkspaceSaveMu.Unlock()
}

// ShouldSuppressWorkspaceReload returns true if all given paths were recently
// saved by this process and their on-disk content matches the saved fingerprint.
func (wm *Manager) ShouldSuppressWorkspaceReload(paths []string, now time.Time) bool {
	if len(paths) == 0 {
		return false
	}

	type pathMarker struct {
		path   string
		marker localWorkspaceSaveMarker
	}
	var toCheck []pathMarker

	wm.localWorkspaceSaveMu.Lock()
	if len(wm.localWorkspaceSavesAt) == 0 {
		wm.localWorkspaceSaveMu.Unlock()
		return false
	}
	pruneOldLocalWorkspaceSavesLocked(wm.localWorkspaceSavesAt, now)
	if len(wm.localWorkspaceSavesAt) == 0 {
		wm.localWorkspaceSaveMu.Unlock()
		return false
	}
	for _, raw := range paths {
		path := filepath.Clean(strings.TrimSpace(raw))
		if path == "" {
			continue
		}
		marker, ok := wm.localWorkspaceSavesAt[path]
		if !ok {
			wm.localWorkspaceSaveMu.Unlock()
			return false
		}
		delta := now.Sub(marker.at)
		if delta < 0 || delta > localWorkspaceReloadSuppressWindow {
			wm.localWorkspaceSaveMu.Unlock()
			return false
		}
		toCheck = append(toCheck, pathMarker{path: path, marker: marker})
	}
	wm.localWorkspaceSaveMu.Unlock()

	for _, pm := range toCheck {
		fingerprint, ok := workspaceMetadataFingerprint(pm.path)
		if !ok {
			return false
		}
		if fingerprint != pm.marker.fingerprint {
			return false
		}
	}
	return len(toCheck) > 0
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// workspaceMetadataPath returns the path to the workspace metadata JSON file.
func workspaceMetadataPath(metadataRoot, wsID string) string {
	root := strings.TrimSpace(metadataRoot)
	id := strings.TrimSpace(wsID)
	if root == "" || id == "" {
		return ""
	}
	return filepath.Clean(filepath.Join(root, id, "workspace.json"))
}

// workspaceMetadataFingerprint returns a fingerprint for the file at path.
// Note: there is a TOCTOU gap between Stat and ReadFile — if the file changes
// between the two calls the fingerprint won't match the stored one, causing
// ShouldSuppressWorkspaceReload to return false (not suppress). This is the
// safe/conservative direction so the race is benign.
func workspaceMetadataFingerprint(path string) (workspaceFileFingerprint, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return workspaceFileFingerprint{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return workspaceFileFingerprint{}, false
	}
	return workspaceFileFingerprint{
		modTimeUnixNano: info.ModTime().UnixNano(),
		size:            info.Size(),
		digest:          sha256.Sum256(data),
	}, true
}
