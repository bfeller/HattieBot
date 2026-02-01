package registry

import (
	"sync"

	"github.com/hattiebot/hattiebot/internal/config"
	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/store"
)

// Factory types for components
type ContextFactory func(db *store.DB) (core.ContextSelector, error)
type ClientFactory func(apiKey, model string) (core.LLMClient, error)
type ExecutorFactory func(cfg *config.Config, db *store.DB, client core.LLMClient) (core.ToolExecutor, error)

var (
	mu               sync.RWMutex
	ContextSelectors = make(map[string]ContextFactory)
	LLMClients       = make(map[string]ClientFactory)
	ToolExecutors    = make(map[string]ExecutorFactory)
)

func RegisterContext(name string, f ContextFactory) {
	mu.Lock()
	defer mu.Unlock()
	ContextSelectors[name] = f
}

func RegisterClient(name string, f ClientFactory) {
	mu.Lock()
	defer mu.Unlock()
	LLMClients[name] = f
}

func RegisterExecutor(name string, f ExecutorFactory) {
	mu.Lock()
	defer mu.Unlock()
	ToolExecutors[name] = f
}

// Getters with Safe Read
func GetContextFactory(name string) (ContextFactory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := ContextSelectors[name]
	return f, ok
}

func GetClientFactory(name string) (ClientFactory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := LLMClients[name]
	return f, ok
}

func GetExecutorFactory(name string) (ExecutorFactory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := ToolExecutors[name]
	return f, ok
}
