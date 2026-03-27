# Model Discovery (models.dev)

## Source Files

- `internal/discovery/models.go` — Provider/Model structs, Registry
- `internal/discovery/models_cache.go` — API fetch, cache read/write
- `internal/discovery/models_query.go` — Query helpers (active models, by provider)
- `internal/discovery/models_test.go` — Tests

## How It Works

1. `Registry` stores providers fetched from `https://models.dev/api.json`
2. Cache is stored at `~/.cache/blunderbust/models-api.json`
3. `Load()` reads cache, falls back to `Refresh()` if missing/corrupt
4. `Refresh()` fetches from API with 3 retries, validates, writes cache
5. `GetActiveModels()` returns models from providers whose env vars are set
6. `GetModelsForProvider()` returns all models for a specific provider

## Data Model

```go
type Provider struct {
    ID    string            `json:"id"`     // "openai", "anthropic"
    Name  string            `json:"name"`   // "OpenAI", "Anthropic"
    Env   []string          `json:"env"`    // ["OPENAI_API_KEY"]
    API   string            `json:"api"`    // Base URL
    Models map[string]Model `json:"models"` // key=model ID
}

type Model struct {
    ID   string `json:"id"`   // "gpt-4o"
    Name string `json:"name"` // "GPT-4o"
}
```

Full model IDs are formatted as `{providerID}/{modelID}` (e.g., `openai/gpt-4o`).

## Config Integration

Model lists in harness config support three formats:
- `"claude-sonnet-4-20250514"` — specific model (kept as-is, not resolved by registry)
- `"provider:openai"` — all models from provider (resolved at UI time)
- `"discover:active"` — all models from all active providers (resolved at UI time)

**Resolution happens in the UI** when building the model selection list, not at
config parse time. The harness stores the raw strings; the UI resolves them via
the Registry.

## API Response Format

```json
{
  "anthropic": {
    "id": "anthropic",
    "name": "Anthropic",
    "env": ["ANTHROPIC_API_KEY"],
    "api": "https://api.anthropic.com",
    "models": {
      "claude-opus-4-20250514": {
        "id": "claude-opus-4-20250514",
        "name": "Claude Opus 4"
      }
    }
  }
}
```

## Amux Integration

1. **New package: `internal/discovery/`**
   - `models.go` — Registry, Provider, Model structs
   - `models_cache.go` — fetch + cache logic
   - `models_query.go` — active/provider queries

2. **Cache path:** `~/.amux/models-api.json` (following amux conventions)

3. **Registry lifecycle:** Loaded during app init, refreshed on demand
   (CLI command or background ticker)

4. **UI integration:** Model selection list resolved from registry when
   creating a new tab with a ticket context

## Key Implementation Notes

- The `KeywordDiscoverActive` and `PrefixProvider` constants control the
  resolution of dynamic model lists
- Provider activation is determined by checking if ALL env vars in `provider.Env`
  are non-empty
- Model IDs in the registry use `{provider}/{model}` format, but config can
  also specify bare model IDs (for tools that don't use the provider prefix)
- The registry is thread-safe (sync.RWMutex protected)
