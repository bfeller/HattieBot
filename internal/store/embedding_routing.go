package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// EmbeddingProviderEntry describes one embedding provider (e.g. embeddinggood).
type EmbeddingProviderEntry struct {
	Type       string `json:"type"`                 // "embeddinggood", etc.
	BaseURLEnv string `json:"base_url_env,omitempty"`
	APIKeyEnv  string `json:"api_key_env,omitempty"`
	Dimension  int    `json:"dimension,omitempty"` // 128, 256, 512, 768; 0 = use default
}

// EmbeddingRoutingConfig holds embedding_providers and default_provider for dynamic routing.
type EmbeddingRoutingConfig struct {
	EmbeddingProviders map[string]EmbeddingProviderEntry `json:"embedding_providers"`
	DefaultProvider    string                             `json:"default_provider"`
}

const embeddingRoutingFilename = "embedding_routing.json"

// LoadEmbeddingRouting reads embedding_routing.json from dir. If the file is missing, returns (nil, nil).
func LoadEmbeddingRouting(dir string) (*EmbeddingRoutingConfig, error) {
	p := filepath.Join(dir, embeddingRoutingFilename)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var c EmbeddingRoutingConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if c.EmbeddingProviders == nil {
		c.EmbeddingProviders = make(map[string]EmbeddingProviderEntry)
	}
	return &c, nil
}

// SaveEmbeddingRouting writes embedding_routing.json to dir.
func SaveEmbeddingRouting(dir string, c *EmbeddingRoutingConfig) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	p := filepath.Join(dir, embeddingRoutingFilename)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

// HasDefaultProvider returns true if config has a non-empty default provider.
func (c *EmbeddingRoutingConfig) HasDefaultProvider() bool {
	if c == nil {
		return false
	}
	return c.DefaultProvider != ""
}
