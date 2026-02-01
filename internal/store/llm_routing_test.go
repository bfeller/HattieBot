package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLLMRouting_MissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadLLMRouting(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Errorf("expected nil when file missing, got %+v", cfg)
	}
}

func TestLoadLLMRouting_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, llmRoutingFilename)
	data := []byte(`{
		"llm_providers": {
			"openrouter": {"type": "openrouter", "api_key_env": "OPENROUTER_API_KEY", "base_url": "https://openrouter.ai/api/v1"}
		},
		"model_routing": {
			"default": {"provider": "openrouter", "model": "anthropic/claude-3.5-sonnet"}
		}
	}`)
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadLLMRouting(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if len(cfg.LLMProviders) != 1 || cfg.LLMProviders["openrouter"].Type != "openrouter" {
		t.Errorf("llm_providers: %+v", cfg.LLMProviders)
	}
	r, ok := cfg.ModelRouting["default"]
	if !ok || r.Provider != "openrouter" || r.Model != "anthropic/claude-3.5-sonnet" {
		t.Errorf("model_routing: %+v", cfg.ModelRouting)
	}
	if !cfg.HasDefaultRoute() {
		t.Error("HasDefaultRoute should be true")
	}
}

func TestSaveLLMRouting_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := &LLMRoutingConfig{
		LLMProviders: map[string]LLMProviderEntry{
			"openrouter": {Type: "openrouter", APIKeyEnv: "OPENROUTER_API_KEY"},
		},
		ModelRouting: map[string]ModelRouteEntry{
			"default": {Provider: "openrouter", Model: "test-model"},
		},
	}
	if err := SaveLLMRouting(dir, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadLLMRouting(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("expected loaded config")
	}
	if loaded.LLMProviders["openrouter"].Type != "openrouter" {
		t.Errorf("loaded providers: %+v", loaded.LLMProviders)
	}
	if loaded.ModelRouting["default"].Model != "test-model" {
		t.Errorf("loaded routing: %+v", loaded.ModelRouting)
	}
}

func TestHasDefaultRoute(t *testing.T) {
	if (&LLMRoutingConfig{}).HasDefaultRoute() {
		t.Error("empty config should not have default route")
	}
	cfg := &LLMRoutingConfig{
		ModelRouting: map[string]ModelRouteEntry{
			"default": {Provider: "openrouter", Model: "m"},
		},
	}
	if !cfg.HasDefaultRoute() {
		t.Error("config with default route should have HasDefaultRoute true")
	}
	cfg.ModelRouting["default"] = ModelRouteEntry{Provider: "", Model: "m"}
	if cfg.HasDefaultRoute() {
		t.Error("empty provider should not count as default route")
	}
}
