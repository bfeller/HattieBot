package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LLMProviderEntry describes one LLM provider (e.g. openrouter, ollama).
type LLMProviderEntry struct {
	Type      string `json:"type"`       // "openrouter", "ollama", etc.
	APIKeyEnv string `json:"api_key_env,omitempty"`
	BaseURL   string `json:"base_url,omitempty"`
}

// ModelRouteEntry describes which provider and model to use for a route.
type ModelRouteEntry struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// LLMRoutingConfig holds llm_providers and model_routing for dynamic routing.
type LLMRoutingConfig struct {
	LLMProviders  map[string]LLMProviderEntry `json:"llm_providers"`
	ModelRouting  map[string]ModelRouteEntry   `json:"model_routing"`
}

const llmRoutingFilename = "llm_routing.json"

// LoadLLMRouting reads llm_routing.json from dir. If the file is missing, returns (nil, nil) for backward compat.
func LoadLLMRouting(dir string) (*LLMRoutingConfig, error) {
	p := filepath.Join(dir, llmRoutingFilename)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var c LLMRoutingConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if c.LLMProviders == nil {
		c.LLMProviders = make(map[string]LLMProviderEntry)
	}
	if c.ModelRouting == nil {
		c.ModelRouting = make(map[string]ModelRouteEntry)
	}
	return &c, nil
}

// SaveLLMRouting writes llm_routing.json to dir.
func SaveLLMRouting(dir string, c *LLMRoutingConfig) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	p := filepath.Join(dir, llmRoutingFilename)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

// HasDefaultRoute returns true if config has a non-empty "default" route.
func (c *LLMRoutingConfig) HasDefaultRoute() bool {
	if c == nil {
		return false
	}
	r, ok := c.ModelRouting["default"]
	return ok && r.Provider != "" && r.Model != ""
}
