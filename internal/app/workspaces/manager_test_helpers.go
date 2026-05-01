package workspaces

// ManagerConfig is used by NewManagerWithConfig to initialize a Manager with
// pre-populated internal state. This is only intended for tests.
type ManagerConfig struct {
	DirtyWorkspaceIDs    map[string]bool
	DeletingWorkspaceIDs map[string]bool
	CreatingWorkspaceIDs map[string]bool
}

// NewManagerWithConfig creates a Manager initialized with the given config.
// This is only intended for tests that need to set up specific internal state.
func NewManagerWithConfig(cfg ManagerConfig) *Manager {
	m := NewManager()
	for id := range cfg.DirtyWorkspaceIDs {
		if id != "" {
			m.dirtyWorkspaces[id] = true
		}
	}
	for id := range cfg.DeletingWorkspaceIDs {
		if id != "" {
			m.deletingWorkspaceIDs[id] = true
		}
	}
	for id := range cfg.CreatingWorkspaceIDs {
		if id != "" {
			m.creatingWorkspaceIDs[id] = true
		}
	}
	return m
}
