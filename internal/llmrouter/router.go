package llmrouter

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/openrouter"
	"github.com/hattiebot/hattiebot/internal/store"
)

// RouterClient implements core.LLMClient by resolving the "default" route to a provider+model,
// calling that client, and falling back to openrouter_bootstrap on error.
type RouterClient struct {
	Config   *store.LLMRoutingConfig
	Fallback core.LLMClient
    Registry *ProviderRegistry
	getEnv   func(string) string
	mu       sync.RWMutex
	cache    map[string]core.LLMClient
}

// NewRouterClient creates a RouterClient with the given routing config and fallback client.
// getEnv is used to resolve api_key_env; if nil, os.Getenv is used.
func NewRouterClient(cfg *store.LLMRoutingConfig, fallback core.LLMClient, configDir string, getEnv func(string) string) *RouterClient {
	if getEnv == nil {
		getEnv = os.Getenv
	}
    registry := NewProviderRegistry(configDir)
    if err := registry.LoadTemplates(); err != nil {
        // Log error but continue?
        fmt.Printf("warning: failed to load provider templates: %v\n", err)
    }

	return &RouterClient{
		Config:   cfg,
		Fallback: fallback,
        Registry: registry,
		getEnv:   getEnv,
		cache:    make(map[string]core.LLMClient),
	}
}

// getClient resolves route "default" to (provider, model) and returns a core.LLMClient for it.
func (r *RouterClient) getClient(route string) (core.LLMClient, error) {
	if r.Config == nil {
		return nil, nil
	}
	routeEntry, ok := r.Config.ModelRouting[route]
	if !ok || routeEntry.Provider == "" || routeEntry.Model == "" {
		return nil, nil
	}
	providerEntry, ok := r.Config.LLMProviders[routeEntry.Provider]
	if !ok {
		return nil, nil
	}
	
    // Cache Check
	cacheKey := routeEntry.Provider + ":" + routeEntry.Model
	r.mu.RLock()
	c, ok := r.cache[cacheKey]
	r.mu.RUnlock()
	if ok {
		return c, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok = r.cache[cacheKey]; ok {
		return c, nil
	}

    // Build Client
    var client core.LLMClient
    
	if providerEntry.Type == "openrouter" {
        apiKey := r.getEnv(providerEntry.APIKeyEnv)
        if apiKey == "" {
            return nil, nil
        }
        client = openrouter.NewClient(apiKey, routeEntry.Model)
    } else {
        // Generic Provider lookup
        tmpl, ok := r.Registry.GetTemplate(providerEntry.Type)
        if !ok {
            return nil, fmt.Errorf("unknown provider type '%s' (no template found)", providerEntry.Type)
        }
        client = &GenericProviderClient{
            Template: tmpl,
            Instance: providerEntry,
            Route:    routeEntry,
            GetEnv:   r.getEnv,
        }
    }
	
	r.cache[cacheKey] = client
	return client, nil
}

// ChatCompletion calls the primary client for "default" route; on error uses Fallback.
func (r *RouterClient) ChatCompletion(ctx context.Context, messages []core.Message) (string, error) {
	c, err := r.getClient("default")
	if c != nil && err == nil {
		out, err := c.ChatCompletion(ctx, messages)
		if err == nil {
			return out, nil
		}
		log.Printf("[LLMROUTER] primary client failed: %v; falling back", err)
	}
	if r.Fallback != nil {
		return r.Fallback.ChatCompletion(ctx, messages)
	}
	if err != nil {
		return "", err
	}
	return "", nil
}

// ChatCompletionWithTools calls the primary client for "default" route; on error uses Fallback.
func (r *RouterClient) ChatCompletionWithTools(ctx context.Context, messages []core.Message, tools []core.ToolDefinition) (string, []core.ToolCall, error) {
	c, err := r.getClient("default")
	if c != nil && err == nil {
		out, calls, err := c.ChatCompletionWithTools(ctx, messages, tools)
		if err == nil {
			return out, calls, nil
		}
		log.Printf("[LLMROUTER] primary client failed: %v; falling back", err)
	}
	if r.Fallback != nil {
		return r.Fallback.ChatCompletionWithTools(ctx, messages, tools)
	}
	if err != nil {
		return "", nil, err
	}
	return "", nil, nil
}

// Embed calls the primary client for "default" route; on error uses Fallback.
func (r *RouterClient) Embed(ctx context.Context, text string) ([]float32, error) {
	c, err := r.getClient("default")
	if c != nil && err == nil {
		out, err := c.Embed(ctx, text)
		if err == nil {
			return out, nil
		}
		log.Printf("[LLMROUTER] primary client failed: %v; falling back", err)
	}
	if r.Fallback != nil {
		return r.Fallback.Embed(ctx, text)
	}
	if err != nil {
		return nil, err
	}
	return nil, nil
}
