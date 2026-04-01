package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	modelsAPIURL = "https://models.dev/api.json"
)

// Refresh fetches the latest api.json from models.dev and updates the cache.
func (r *Registry) Refresh(ctx context.Context) error {
	parsed, err := r.fetchProviders(ctx)
	if err != nil {
		return err
	}

	if err := validateProviders(parsed); err != nil {
		return err
	}

	if err := r.writeProvidersCache(parsed); err != nil {
		return err
	}

	r.setProviders(parsed)
	return nil
}

// Load attempts to load providers from the local cache.
// If the cache is missing or corrupt, it triggers a Refresh.
func (r *Registry) Load(ctx context.Context) error {
	data, err := os.ReadFile(r.cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return r.Refresh(ctx)
		}
		return fmt.Errorf("reading cache file: %w", err)
	}

	var parsed map[string]Provider
	if err := json.Unmarshal(data, &parsed); err != nil {
		refreshErr := r.Refresh(ctx)
		if refreshErr != nil {
			return fmt.Errorf("cache corrupted (%w) and refresh failed: %w", err, refreshErr)
		}
		return nil
	}

	if err := validateProviders(parsed); err != nil {
		if refreshErr := r.Refresh(ctx); refreshErr != nil {
			if len(parsed) == 0 {
				return fmt.Errorf("cache is empty and refresh failed: %w", refreshErr)
			}
			return fmt.Errorf("cache invalid (%w) and refresh failed: %w", err, refreshErr)
		}
		return nil
	}

	r.setProviders(parsed)
	return nil
}

func (r *Registry) fetchProviders(ctx context.Context) (map[string]Provider, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsAPIURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := r.fetchWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status from models.dev: %s", resp.Status)
	}

	var parsed map[string]Provider
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decoding api.json: %w", err)
	}

	return parsed, nil
}

func (r *Registry) fetchWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var (
		resp     *http.Response
		fetchErr error
	)

	for attempt := 0; attempt < r.retryCount; attempt++ {
		resp, fetchErr = r.client.Do(req)
		if fetchErr == nil {
			return resp, nil
		}

		delay := r.retryDelay * time.Duration(1<<attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, fmt.Errorf("fetching models.dev/api.json: %w", fetchErr)
}

func validateProviders(parsed map[string]Provider) error {
	if len(parsed) == 0 {
		return fmt.Errorf("validation error: decoded JSON is empty")
	}

	for key, provider := range parsed {
		if provider.ID == "" {
			return fmt.Errorf("validation error: provider %q missing ID", key)
		}
	}

	return nil
}

func (r *Registry) writeProvidersCache(parsed map[string]Provider) error {
	dir := filepath.Dir(r.cachePath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	data, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling api.json for cache: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "models-api.*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing cache data: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, r.cachePath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming cache file: %w", err)
	}

	return nil
}
