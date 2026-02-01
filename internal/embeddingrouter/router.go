package embeddingrouter

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/embeddinggood"
	"github.com/hattiebot/hattiebot/internal/store"
)

// Router implements core.EmbeddingClient by resolving the default provider from embedding_routing.json
// and delegating to the corresponding EmbeddingGood client.
type Router struct {
	Config   *store.EmbeddingRoutingConfig
	Fallback core.EmbeddingClient
	getEnv   func(string) string
	mu       sync.RWMutex
	cache    map[string]core.EmbeddingClient
}

// NewRouter creates a Router with the given config and fallback. getEnv resolves env var names; if nil, os.Getenv is used.
func NewRouter(cfg *store.EmbeddingRoutingConfig, fallback core.EmbeddingClient, getEnv func(string) string) *Router {
	if getEnv == nil {
		getEnv = os.Getenv
	}
	return &Router{
		Config:   cfg,
		Fallback: fallback,
		getEnv:   getEnv,
		cache:    make(map[string]core.EmbeddingClient),
	}
}

// Embed resolves the default provider and calls its Embed; on error or missing config uses Fallback.
func (r *Router) Embed(ctx context.Context, text string, embedType string) ([]float32, error) {
	c, err := r.getClient()
	if c != nil && err == nil {
		out, err := c.Embed(ctx, text, embedType)
		if err == nil {
			return out, nil
		}
		log.Printf("[EMBEDROUTER] primary failed: %v; falling back", err)
	}
	if r.Fallback != nil {
		return r.Fallback.Embed(ctx, text, embedType)
	}
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// getClient returns the EmbeddingClient for the default provider; caches by provider name.
func (r *Router) getClient() (core.EmbeddingClient, error) {
	if r.Config == nil || !r.Config.HasDefaultProvider() {
		return nil, nil
	}
	name := r.Config.DefaultProvider
	entry, ok := r.Config.EmbeddingProviders[name]
	if !ok || entry.Type == "" {
		return nil, nil
	}

	r.mu.RLock()
	c, ok := r.cache[name]
	r.mu.RUnlock()
	if ok {
		return c, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok = r.cache[name]; ok {
		return c, nil
	}

	baseURL := r.getEnv(entry.BaseURLEnv)
	apiKey := r.getEnv(entry.APIKeyEnv)
	if baseURL == "" || apiKey == "" {
		return nil, nil
	}
	dim := entry.Dimension
	if dim <= 0 {
		dim = 768
	}
	c = embeddinggood.NewClient(baseURL, apiKey, dim)
	r.cache[name] = c
	return c, nil
}
