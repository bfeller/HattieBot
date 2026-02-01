package wiring

import (
	"log"

	"github.com/hattiebot/hattiebot/internal/config"
	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/registry"
	"github.com/hattiebot/hattiebot/internal/store"
)

// LoadContextSelector attempts to load the named component. Falls back to "default".
func LoadContextSelector(name string, db *store.DB) core.ContextSelector {
	factory, ok := registry.GetContextFactory(name)
	if !ok {
		log.Printf("ContextSelector '%s' not found. Falling back to 'default'.", name)
		return loadDefaultContext(db)
	}
	// Try init with panic recovery
	cs, err := safeInitContext(factory, db)
	if err != nil {
		log.Printf("Failed to init ContextSelector '%s': %v. Falling back to 'default'.", name, err)
		return loadDefaultContext(db)
	}
	return cs
}

func loadDefaultContext(db *store.DB) core.ContextSelector {
	f, _ := registry.GetContextFactory("default")
	c, _ := f(db) // Assume default never fails
	return c
}

func safeInitContext(f registry.ContextFactory, db *store.DB) (c core.ContextSelector, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = logPanic(r)
		}
	}()
	return f(db)
}

// LoadClient attempts to load the named client. Falls back to "default".
func LoadClient(name, apiKey, model string) core.LLMClient {
	factory, ok := registry.GetClientFactory(name)
	if !ok {
		log.Printf("Client '%s' not found. Falling back to 'default'.", name)
		return loadDefaultClient(apiKey, model)
	}
	c, err := safeInitClient(factory, apiKey, model)
	if err != nil {
		log.Printf("Failed to init Client '%s': %v. Falling back to 'default'.", name, err)
		return loadDefaultClient(apiKey, model)
	}
	return c
}

func loadDefaultClient(apiKey, model string) core.LLMClient {
	f, _ := registry.GetClientFactory("default")
	c, _ := f(apiKey, model)
	return c
}

func safeInitClient(f registry.ClientFactory, apiKey, model string) (c core.LLMClient, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = logPanic(r)
		}
	}()
	return f(apiKey, model)
}

// LoadExecutor attempts to load the named executor. Falls back to "default".
func LoadExecutor(name string, cfg *config.Config, db *store.DB, client core.LLMClient) core.ToolExecutor {
	factory, ok := registry.GetExecutorFactory(name)
	if !ok {
		log.Printf("ToolExecutor '%s' not found. Falling back to 'default'.", name)
		return loadDefaultExecutor(cfg, db, client)
	}
	e, err := safeInitExecutor(factory, cfg, db, client)
	if err != nil {
		log.Printf("Failed to init ToolExecutor '%s': %v. Falling back to 'default'.", name, err)
		return loadDefaultExecutor(cfg, db, client)
	}
	return e
}

func loadDefaultExecutor(cfg *config.Config, db *store.DB, client core.LLMClient) core.ToolExecutor {
	f, _ := registry.GetExecutorFactory("default")
	e, _ := f(cfg, db, client)
	return e
}

func safeInitExecutor(f registry.ExecutorFactory, cfg *config.Config, db *store.DB, client core.LLMClient) (e core.ToolExecutor, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = logPanic(r)
		}
	}()
	return f(cfg, db, client)
}

func logPanic(r interface{}) error {
	return logPanicError{r}
}

type logPanicError struct {
	Reason interface{}
}

func (e logPanicError) Error() string {
	return "panic during initialization"
}
