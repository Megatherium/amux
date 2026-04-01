package discovery

import "testing"

func TestIsProviderActive(t *testing.T) {
	registry, err := NewRegistry("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	provider := Provider{
		ID:  "test-provider",
		Env: []string{"TEST_API_KEY"},
	}

	if registry.isProviderActive(provider) {
		t.Errorf("expected provider to be inactive when env var is missing")
	}

	t.Setenv("TEST_API_KEY", "dummy")
	if !registry.isProviderActive(provider) {
		t.Errorf("expected provider to be active when env var is set")
	}

	if !registry.isProviderActive(Provider{ID: "no-env-provider"}) {
		t.Errorf("expected provider with no required env vars to be active")
	}
}

func TestGetActiveModels(t *testing.T) {
	t.Run("one provider active", func(t *testing.T) {
		registry, err := NewRegistry("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		providers := map[string]Provider{
			"p1": {
				ID:  "p1",
				Env: []string{"P1_KEY"},
				Models: map[string]Model{
					"m1": {ID: "m1", Name: "Model 1"},
				},
			},
			"p2": {
				ID:  "p2",
				Env: []string{"P2_KEY"},
				Models: map[string]Model{
					"m2": {ID: "m2", Name: "Model 2"},
				},
			},
		}
		registry.SetProviders(providers)

		t.Setenv("P1_KEY", "val")

		active := registry.GetActiveModels()
		if len(active) != 1 {
			t.Fatalf("expected 1 active model, got %d", len(active))
		}
		if active[0] != "p1/m1" {
			t.Errorf("expected active model p1/m1, got %s", active[0])
		}
	})

	t.Run("all providers active", func(t *testing.T) {
		registry, err := NewRegistry("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		providers := map[string]Provider{
			"a": {
				ID:  "a",
				Env: []string{"A_KEY"},
				Models: map[string]Model{
					"m1": {ID: "m1", Name: "Model 1"},
				},
			},
			"b": {
				ID:  "b",
				Env: []string{"B_KEY"},
				Models: map[string]Model{
					"m2": {ID: "m2", Name: "Model 2"},
				},
			},
		}
		registry.SetProviders(providers)

		t.Setenv("A_KEY", "val")
		t.Setenv("B_KEY", "val")

		active := registry.GetActiveModels()
		if len(active) != 2 {
			t.Fatalf("expected 2 active models, got %d: %v", len(active), active)
		}
		if active[0] != "a/m1" || active[1] != "b/m2" {
			t.Errorf("expected sorted [a/m1, b/m2], got %v", active)
		}
	})

	t.Run("no providers active", func(t *testing.T) {
		registry, err := NewRegistry("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		providers := map[string]Provider{
			"p1": {
				ID:  "p1",
				Env: []string{"UNSET_KEY"},
				Models: map[string]Model{
					"m1": {ID: "m1", Name: "Model 1"},
				},
			},
		}
		registry.SetProviders(providers)

		active := registry.GetActiveModels()
		if len(active) != 0 {
			t.Fatalf("expected 0 active models, got %d: %v", len(active), active)
		}
	})
}

func TestGetModelsForProvider(t *testing.T) {
	t.Run("existing provider", func(t *testing.T) {
		registry, err := NewRegistry("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		providers := map[string]Provider{
			"p1": {
				ID: "p1",
				Models: map[string]Model{
					"m1": {ID: "m1", Name: "Model 1"},
					"m2": {ID: "m2", Name: "Model 2"},
				},
			},
		}
		registry.SetProviders(providers)

		models := registry.GetModelsForProvider("p1")
		if len(models) != 2 {
			t.Fatalf("expected 2 models, got %d", len(models))
		}
	})

	t.Run("sorted output with multiple models", func(t *testing.T) {
		registry, err := NewRegistry("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		providers := map[string]Provider{
			"p1": {
				ID: "p1",
				Models: map[string]Model{
					"charlie": {ID: "charlie", Name: "C"},
					"alpha":   {ID: "alpha", Name: "A"},
					"bravo":   {ID: "bravo", Name: "B"},
				},
			},
		}
		registry.SetProviders(providers)

		models := registry.GetModelsForProvider("p1")
		if len(models) != 3 {
			t.Fatalf("expected 3 models, got %d", len(models))
		}
		expected := []string{"p1/alpha", "p1/bravo", "p1/charlie"}
		for i, exp := range expected {
			if models[i] != exp {
				t.Errorf("models[%d] = %q, want %q", i, models[i], exp)
			}
		}
	})

	t.Run("nonexistent provider", func(t *testing.T) {
		registry, err := NewRegistry("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		missing := registry.GetModelsForProvider("nonexistent")
		if len(missing) != 0 {
			t.Fatalf("expected 0 models for nonexistent provider, got %d", len(missing))
		}
	})
}

func TestFormatProviderModels(t *testing.T) {
	provider := Provider{
		ID: "openai",
		Models: map[string]Model{
			"gpt-4":   {ID: "gpt-4", Name: "GPT-4"},
			"gpt-3.5": {ID: "gpt-3.5", Name: "GPT-3.5"},
		},
	}

	models := formatProviderModels(provider)
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	seen := map[string]bool{}
	for _, m := range models {
		seen[m] = true
	}
	if !seen["openai/gpt-4"] || !seen["openai/gpt-3.5"] {
		t.Errorf("expected provider/model format, got %v", models)
	}
}
