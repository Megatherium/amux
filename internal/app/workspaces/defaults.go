package workspaces

import "time"

const (
	// gitPathWaitInterval is the polling interval when waiting for a new worktree to expose .git.
	gitPathWaitInterval = 100 * time.Millisecond

	// localWorkspaceReloadSuppressWindow suppresses watcher-driven workspace reloads
	// immediately after this process saves workspace metadata.
	localWorkspaceReloadSuppressWindow = 800 * time.Millisecond
)
