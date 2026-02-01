package builtin

import (
	"context"

	"github.com/hattiebot/hattiebot/internal/openrouter"
)

// Tool represents a modular tool implementation.
type Tool interface {
	Name() string
	Definition() openrouter.ToolDefinition
	Execute(ctx context.Context, argsJSON string) (string, error)
}

// Registry holds all registered built-in tools.
var Registry = map[string]Tool{}

// Register adds a tool to the registry.
func Register(t Tool) {
	Registry[t.Name()] = t
}
