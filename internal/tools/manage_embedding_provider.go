package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hattiebot/hattiebot/internal/store"
)

// ManageEmbeddingProviderTool handles embedding provider and default-route management.
func ManageEmbeddingProviderTool(ctx context.Context, configDir string, argsJSON string) (string, error) {
	var args struct {
		Action       string                         `json:"action"` // list_providers, register_provider, set_default
		ProviderName string                         `json:"provider_name"`
		Provider     store.EmbeddingProviderEntry   `json:"provider_config"`
	}

	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return ErrJSON(err), nil
	}

	switch args.Action {
	case "list_providers":
		cfg, err := store.LoadEmbeddingRouting(configDir)
		if err != nil {
			return ErrJSON(err), nil
		}
		if cfg == nil {
			return "{}", nil
		}
		b, _ := json.MarshalIndent(cfg, "", "  ")
		return string(b), nil

	case "register_provider":
		cfg, err := store.LoadEmbeddingRouting(configDir)
		if err != nil {
			return ErrJSON(err), nil
		}
		if cfg == nil {
			cfg = &store.EmbeddingRoutingConfig{
				EmbeddingProviders: make(map[string]store.EmbeddingProviderEntry),
			}
		}
		if args.ProviderName == "" {
			return `{"error": "provider_name required"}`, nil
		}
		if args.Provider.Type == "" {
			return `{"error": "provider_config.type required"}`, nil
		}
		cfg.EmbeddingProviders[args.ProviderName] = args.Provider
		if err := store.SaveEmbeddingRouting(configDir, cfg); err != nil {
			return ErrJSON(err), nil
		}
		return `{"status": "registered"}`, nil

	case "set_default":
		cfg, err := store.LoadEmbeddingRouting(configDir)
		if err != nil {
			return ErrJSON(err), nil
		}
		if cfg == nil {
			return `{"error": "no config found, register a provider first"}`, nil
		}
		if args.ProviderName == "" {
			return `{"error": "provider_name required"}`, nil
		}
		if _, ok := cfg.EmbeddingProviders[args.ProviderName]; !ok {
			return fmt.Sprintf(`{"error": "provider '%s' not found"}`, args.ProviderName), nil
		}
		cfg.DefaultProvider = args.ProviderName
		if err := store.SaveEmbeddingRouting(configDir, cfg); err != nil {
			return ErrJSON(err), nil
		}
		return `{"status": "default_updated"}`, nil

	default:
		return `{"error": "unknown action"}`, nil
	}
}
