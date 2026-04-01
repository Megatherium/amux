package discovery

import (
	"fmt"
	"os"
	"sort"
)

// GetActiveModels returns a sorted list of model IDs from providers that have
// their required environment variables set.
func (r *Registry) GetActiveModels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var activeModels []string
	for _, provider := range r.providers {
		if !r.isProviderActive(provider) {
			continue
		}
		activeModels = append(activeModels, formatProviderModels(provider)...)
	}

	sort.Strings(activeModels)
	return activeModels
}

func (r *Registry) isProviderActive(provider Provider) bool {
	for _, envVar := range provider.Env {
		if os.Getenv(envVar) == "" {
			return false
		}
	}
	return true
}

// GetModelsForProvider returns a sorted list of model IDs for a specific provider.
// Returns nil if the provider is not found.
func (r *Registry) GetModelsForProvider(providerID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.providers[providerID]
	if !ok {
		return nil
	}

	models := formatProviderModels(provider)
	sort.Strings(models)
	return models
}

func formatProviderModels(provider Provider) []string {
	models := make([]string, 0, len(provider.Models))
	for _, model := range provider.Models {
		models = append(models, fmt.Sprintf("%s/%s", provider.ID, model.ID))
	}
	return models
}

// SetProviders replaces all providers in the registry, useful for testing.
func (r *Registry) SetProviders(providers map[string]Provider) {
	r.setProviders(providers)
}

func (r *Registry) setProviders(providers map[string]Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = providers
}
