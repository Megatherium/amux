package workspaces

import (
	"time"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/git"
	"github.com/andyrewlee/amux/internal/process"
)

// GitOperations abstracts git workspace operations for testability.
type GitOperations interface {
	CreateWorkspace(repoPath, workspacePath, branch, base string) error
	RemoveWorkspace(repoPath, workspacePath string) error
	DeleteBranch(repoPath, branch string) error
	DiscoverWorkspaces(project *data.Project) ([]data.Workspace, error)
}

type defaultGitOps struct{}

func (defaultGitOps) CreateWorkspace(repoPath, workspacePath, branch, base string) error {
	return git.CreateWorkspace(repoPath, workspacePath, branch, base)
}

func (defaultGitOps) RemoveWorkspace(repoPath, workspacePath string) error {
	return git.RemoveWorkspace(repoPath, workspacePath)
}

func (defaultGitOps) DeleteBranch(repoPath, branch string) error {
	return git.DeleteBranch(repoPath, branch)
}

func (defaultGitOps) DiscoverWorkspaces(project *data.Project) ([]data.Workspace, error) {
	return git.DiscoverWorkspaces(project)
}

// ProjectRegistry is the minimal interface used for project tracking.
type ProjectRegistry interface {
	Projects() ([]string, error)
	AddProject(path string) error
	RemoveProject(path string) error
}

// WorkspaceStore is the minimal interface used for workspace metadata.
type WorkspaceStore interface {
	ListByRepo(repo string) ([]*data.Workspace, error)
	ListByRepoIncludingArchived(repo string) ([]*data.Workspace, error)
	LoadMetadataFor(workspace *data.Workspace) (bool, error)
	UpsertFromDiscovery(workspace *data.Workspace) error
	Save(workspace *data.Workspace) error
	Delete(id data.WorkspaceID) error
	ResolvedDefaultAssistant() string
}

// Service handles workspace lifecycle operations: creation, deletion, loading, and rescanning.
type Service struct {
	registry           ProjectRegistry
	store              WorkspaceStore
	scripts            *process.ScriptRunner
	workspacesRoot     string
	GitOps             GitOperations
	GitPathWaitTimeout time.Duration
}

// NewService creates a new workspace Service.
func NewService(registry ProjectRegistry, store WorkspaceStore, scripts *process.ScriptRunner, workspacesRoot string) *Service {
	return &Service{
		registry:           registry,
		store:              store,
		scripts:            scripts,
		workspacesRoot:     workspacesRoot,
		GitOps:             defaultGitOps{},
		GitPathWaitTimeout: 3 * time.Second,
	}
}

// NewServiceForTest creates a Service with custom options for testing.
func NewServiceForTest(registry ProjectRegistry, store WorkspaceStore, scripts *process.ScriptRunner, workspacesRoot string, gitOps GitOperations, gitPathWaitTimeout time.Duration) *Service {
	return &Service{
		registry:           registry,
		store:              store,
		scripts:            scripts,
		workspacesRoot:     workspacesRoot,
		GitOps:             gitOps,
		GitPathWaitTimeout: gitPathWaitTimeout,
	}
}

// ResolvedDefaultAssistant returns the default assistant from the store, or the global default.
func (s *Service) ResolvedDefaultAssistant() string {
	if s != nil && s.store != nil {
		return s.store.ResolvedDefaultAssistant()
	}
	return data.DefaultAssistant
}
