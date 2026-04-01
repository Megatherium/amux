package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	registry, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	registry.retryDelay = time.Millisecond
	registry.retryCount = 1
	return registry
}

func TestRefreshContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	registry := newTestRegistry(t)
	registry.client.Transport = &rewriteTransport{targetURL: server.URL}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := registry.Refresh(ctx)
	if err == nil {
		t.Fatal("expected error due to context timeout, got nil")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context timeout error, got: %v", err)
	}
}

func TestRefreshErrors(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		errMatches string
	}{
		{
			name: "server error 500",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			errMatches: "unexpected status from models.dev",
		},
		{
			name: "invalid json",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{invalid json`))
			},
			errMatches: "decoding api.json",
		},
		{
			name: "empty json object",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{}`))
			},
			errMatches: "validation error",
		},
		{
			name: "missing provider ID",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"test": {"name": "test"}}`))
			},
			errMatches: "missing ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			registry := newTestRegistry(t)
			registry.client.Transport = &rewriteTransport{targetURL: server.URL}

			err := registry.Refresh(context.Background())
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errMatches) {
				t.Errorf("expected error containing %q, got: %v", tt.errMatches, err)
			}
		})
	}
}

func TestLoadCacheHandling(t *testing.T) {
	t.Run("missing cache triggers refresh", func(t *testing.T) {
		registry := newTestRegistry(t)
		registry.client.Transport = &errorTransport{}

		err := registry.Load(context.Background())
		if err == nil || !strings.Contains(err.Error(), "simulated network error") {
			t.Errorf("expected network error from refresh on missing cache, got: %v", err)
		}
	})

	t.Run("corrupt cache triggers refresh", func(t *testing.T) {
		registry := newTestRegistry(t)

		if err := os.WriteFile(registry.GetCachePath(), []byte("corrupt json"), 0o600); err != nil {
			t.Fatalf("failed to write corrupt cache: %v", err)
		}
		registry.client.Transport = &errorTransport{}

		err := registry.Load(context.Background())
		if err == nil || !strings.Contains(err.Error(), "cache corrupted") {
			t.Errorf("expected cache corrupted error, got: %v", err)
		}
	})

	t.Run("corrupt cache refreshes and loads providers", func(t *testing.T) {
		registry := newTestRegistry(t)

		if err := os.WriteFile(registry.GetCachePath(), []byte("corrupt json"), 0o600); err != nil {
			t.Fatalf("failed to write corrupt cache: %v", err)
		}

		validResponse := map[string]Provider{
			"openai": {ID: "openai", Name: "OpenAI"},
		}
		b, _ := json.Marshal(validResponse)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(b)
		}))
		defer server.Close()
		registry.client.Transport = &rewriteTransport{targetURL: server.URL}

		err := registry.Load(context.Background())
		if err != nil {
			t.Fatalf("expected successful fallback refresh, got: %v", err)
		}

		registry.mu.RLock()
		if len(registry.providers) != 1 {
			t.Errorf("expected 1 provider loaded after refresh fallback, got %d", len(registry.providers))
		}
		if _, ok := registry.providers["openai"]; !ok {
			t.Error("expected openai provider to be loaded after refresh fallback")
		}
		registry.mu.RUnlock()
	})

	t.Run("valid cache loads successfully", func(t *testing.T) {
		registry := newTestRegistry(t)

		validData := map[string]Provider{
			"test": {ID: "test"},
		}
		b, _ := json.Marshal(validData)
		if err := os.WriteFile(registry.GetCachePath(), b, 0o600); err != nil {
			t.Fatalf("failed to write valid cache: %v", err)
		}

		err := registry.Load(context.Background())
		if err != nil {
			t.Errorf("expected success loading valid cache, got: %v", err)
		}

		registry.mu.RLock()
		if len(registry.providers) != 1 {
			t.Errorf("expected 1 provider loaded from cache, got %d", len(registry.providers))
		}
		registry.mu.RUnlock()
	})
}

func TestRefreshSuccess(t *testing.T) {
	validResponse := map[string]Provider{
		"openai": {
			ID:   "openai",
			Name: "OpenAI",
			Env:  []string{"OPENAI_API_KEY"},
			Models: map[string]Model{
				"gpt-4": {ID: "gpt-4", Name: "GPT-4"},
			},
		},
	}
	b, _ := json.Marshal(validResponse)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(b)
	}))
	defer server.Close()

	registry := newTestRegistry(t)
	registry.client.Transport = &rewriteTransport{targetURL: server.URL}

	err := registry.Refresh(context.Background())
	if err != nil {
		t.Fatalf("expected successful refresh, got: %v", err)
	}

	registry.mu.RLock()
	if len(registry.providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(registry.providers))
	}
	if _, ok := registry.providers["openai"]; !ok {
		t.Error("expected openai provider to be loaded")
	}
	registry.mu.RUnlock()

	data, err := os.ReadFile(registry.GetCachePath())
	if err != nil {
		t.Fatalf("expected cache file to exist, got: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty cache file")
	}
}

func TestFetchWithRetryExhaustion(t *testing.T) {
	registry := newTestRegistry(t)
	registry.retryCount = 3
	registry.client.Transport = &errorTransport{}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, modelsAPIURL, http.NoBody)

	resp, err := registry.fetchWithRetry(context.Background(), req)
	if resp != nil {
		t.Error("expected nil response after retry exhaustion")
	}
	if err == nil || !strings.Contains(err.Error(), "fetching models.dev/api.json") {
		t.Errorf("expected retry exhaustion error, got: %v", err)
	}
}

func TestValidateProviders(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		err := validateProviders(map[string]Provider{})
		if err == nil || !strings.Contains(err.Error(), "decoded JSON is empty") {
			t.Errorf("expected empty validation error, got: %v", err)
		}
	})

	t.Run("missing ID", func(t *testing.T) {
		err := validateProviders(map[string]Provider{
			"test": {Name: "Test"},
		})
		if err == nil || !strings.Contains(err.Error(), "missing ID") {
			t.Errorf("expected missing ID error, got: %v", err)
		}
	})

	t.Run("valid providers", func(t *testing.T) {
		err := validateProviders(map[string]Provider{
			"openai": {ID: "openai"},
		})
		if err != nil {
			t.Errorf("expected valid providers to pass, got: %v", err)
		}
	})
}

func TestWriteProvidersCacheAtomic(t *testing.T) {
	registry := newTestRegistry(t)

	providers := map[string]Provider{
		"test": {ID: "test", Name: "Test"},
	}

	if err := registry.writeProvidersCache(providers); err != nil {
		t.Fatalf("writeProvidersCache failed: %v", err)
	}

	data, err := os.ReadFile(registry.GetCachePath())
	if err != nil {
		t.Fatalf("expected cache file to exist, got: %v", err)
	}

	var loaded map[string]Provider
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("cache file contains invalid JSON: %v", err)
	}
	if len(loaded) != 1 || loaded["test"].ID != "test" {
		t.Errorf("unexpected cache contents: %v", loaded)
	}

	// No temp files should be left behind.
	dir, _ := os.ReadDir(filepath.Dir(registry.GetCachePath()))
	for _, entry := range dir {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("leftover temp file: %s", entry.Name())
		}
	}
}

// rewriteTransport overrides the request URL to hit the test server instead of models.dev.
type rewriteTransport struct {
	targetURL string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := req.Clone(req.Context())
	newReq.URL.Scheme = "http"
	newReq.URL.Host = strings.TrimPrefix(t.targetURL, "http://")
	return http.DefaultTransport.RoundTrip(newReq)
}

// errorTransport always returns a network error.
type errorTransport struct{}

func (t *errorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("simulated network error")
}
