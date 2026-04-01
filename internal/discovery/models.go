package discovery

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// PrefixProvider is the prefix used to specify all models from a specific provider.
	PrefixProvider = "provider:"
	// KeywordDiscoverActive is the keyword used to dynamically include all models from all active providers.
	KeywordDiscoverActive = "discover:active"
)

// Provider represents an LLM provider from models.dev/api.json.
type Provider struct {
	// ID is the unique identifier for the provider (e.g., "openai", "anthropic").
	ID string `json:"id"`
	// Name is the display name of the provider.
	Name string `json:"name"`
	// Env contains the list of environment variables required to activate this provider.
	Env []string `json:"env"`
	// API is the base URL for the provider's API.
	API string `json:"api"`
	// Models maps model IDs to their respective Model configurations.
	Models map[string]Model `json:"models"`
}

// Model represents a specific LLM model.
type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Registry handles model discovery and caching.
type Registry struct {
	cachePath string
	mu        sync.RWMutex
	providers map[string]Provider
	client    *http.Client
}

// NewRegistry creates a new Registry with a default cache path.
// If cacheDir is empty, it defaults to ~/.amux.
func NewRegistry(cacheDir string) (*Registry, error) {
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("could not determine user home directory: %w", err)
		}
		cacheDir = filepath.Join(home, ".amux")
	}
	return &Registry{
		cachePath: filepath.Join(cacheDir, "models-api.json"),
		providers: make(map[string]Provider),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// GetCachePath returns the path to the models-api.json cache file.
func (r *Registry) GetCachePath() string {
	return r.cachePath
}

// SetProvider allows manually setting a provider in the registry,
// useful for testing or overrides.
func (r *Registry) SetProvider(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.ID] = p
}
