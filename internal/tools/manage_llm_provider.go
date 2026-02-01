package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/hattiebot/hattiebot/internal/llmrouter"
	"github.com/hattiebot/hattiebot/internal/store"
)

// ManageLLMProviderTool handles provider template and routing management.
func ManageLLMProviderTool(ctx context.Context, configDir string, argsJSON string) (string, error) {
	var args struct {
		Action       string                      `json:"action"` // list_templates, get_template, save_template, list_providers, register_provider, set_route
		TemplateName string                      `json:"template_name"`
		TemplateBody llmrouter.ProviderTemplate  `json:"template_body"`
		ProviderName string                      `json:"provider_name"`
		Provider     store.LLMProviderEntry      `json:"provider_config"`
		Route        string                      `json:"route"` // e.g. "default"
		Model        string                      `json:"model"`
	}

	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return ErrJSON(err), nil
	}

	registry := llmrouter.NewProviderRegistry(configDir)

	switch args.Action {
	case "list_templates":
		if err := registry.LoadTemplates(); err != nil {
			return ErrJSON(err), nil
		}
        // Hack to get list keys since registry doesn't expose List() yet
        // Actually, let's just re-read the dir or rely on private map access via a new method if we added one.
        // Or strictly use the file system here.
        // Let's assume we add a List() method to registry or just read dir.
        // Re-reading dir is safest.
		providersDir := filepath.Join(configDir, "providers")
        matches, _ := filepath.Glob(filepath.Join(providersDir, "*.json"))
        var names []string
        for _, m := range matches {
            base := filepath.Base(m)
            names = append(names, base[:len(base)-5])
        }
		b, _ := json.Marshal(names)
		return string(b), nil

	case "get_template":
		if err := registry.LoadTemplates(); err != nil {
			return ErrJSON(err), nil
		}
		tmpl, ok := registry.GetTemplate(args.TemplateName)
		if !ok {
			return `{"error": "template not found"}`, nil
		}
		b, _ := json.MarshalIndent(tmpl, "", "  ")
		return string(b), nil

	case "save_template":
		if args.TemplateBody.Name == "" {
			return `{"error": "template name required"}`, nil
		}
		if err := registry.SaveTemplate(args.TemplateBody); err != nil {
			return ErrJSON(err), nil
		}
		return `{"status": "saved"}`, nil

	case "list_providers":
		cfg, err := store.LoadLLMRouting(configDir)
		if err != nil {
			return ErrJSON(err), nil
		}
		if cfg == nil {
			return "[]", nil
		}
		b, _ := json.MarshalIndent(cfg, "", "  ")
		return string(b), nil

	case "register_provider":
		cfg, err := store.LoadLLMRouting(configDir)
		if err != nil {
			return ErrJSON(err), nil
		}
		if cfg == nil {
			cfg = &store.LLMRoutingConfig{
				LLMProviders: make(map[string]store.LLMProviderEntry),
				ModelRouting: make(map[string]store.ModelRouteEntry),
			}
		}
		if args.ProviderName == "" {
			return `{"error": "provider_name required"}`, nil
		}
        // Force type to be provided
        if args.Provider.Type == "" {
             return `{"error": "provider_config.type required"}`, nil
        }
		cfg.LLMProviders[args.ProviderName] = args.Provider
		if err := store.SaveLLMRouting(configDir, cfg); err != nil {
			return ErrJSON(err), nil
		}
		return `{"status": "registered"}`, nil

	case "set_route":
		cfg, err := store.LoadLLMRouting(configDir)
		if err != nil {
			return ErrJSON(err), nil
		}
		if cfg == nil {
			return `{"error": "no config found, register a provider first"}`, nil
		}
		if args.Route == "" || args.ProviderName == "" || args.Model == "" {
			return `{"error": "route, provider_name, and model required"}`, nil
		}
        // Validate provider exists
        if _, ok := cfg.LLMProviders[args.ProviderName]; !ok {
             return fmt.Sprintf(`{"error": "provider '%s' not found"}`, args.ProviderName), nil
        }
		cfg.ModelRouting[args.Route] = store.ModelRouteEntry{
			Provider: args.ProviderName,
			Model:    args.Model,
		}
		if err := store.SaveLLMRouting(configDir, cfg); err != nil {
			return ErrJSON(err), nil
		}
		return `{"status": "route_updated"}`, nil

	default:
		return `{"error": "unknown action"}`, nil
	}
}
