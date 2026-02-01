package tools

import (
	"context"

	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/openrouter"
)

// BlockedTools are NEVER allowed in sub-minds (prevents nesting).
var BlockedTools = map[string]bool{
	"spawn_submind":  true,
	"manage_submind": true,
}

// FilteredExecutor wraps an executor to only allow specific tools.
type FilteredExecutor struct {
	Inner        core.ToolExecutor
	AllowedTools map[string]bool
}

// NewFilteredExecutor creates a new filtered executor.
func NewFilteredExecutor(inner core.ToolExecutor, allowed []string) *FilteredExecutor {
	allowedMap := make(map[string]bool)
	for _, name := range allowed {
		allowedMap[name] = true
	}
	return &FilteredExecutor{
		Inner:        inner,
		AllowedTools: allowedMap,
	}
}

// Execute runs the tool if allowed, otherwise returns an error.
func (f *FilteredExecutor) Execute(ctx context.Context, name, args string) (string, error) {
	// Always block sub-mind spawning from within sub-minds
	if BlockedTools[name] {
		return `{"error": "tool not available in sub-mind context"}`, nil
	}
	// Check if tool is in allowed list
	if !f.AllowedTools[name] {
		return `{"error": "tool not available in this sub-mind mode"}`, nil
	}
	return f.Inner.Execute(ctx, name, args)
}

func (f *FilteredExecutor) SetSpawner(spawner core.SubmindSpawner) {
	f.Inner.SetSpawner(spawner)
}

// FilterToolDefs returns only tools matching the allowed list, excluding blocked tools.
func FilterToolDefs(all []openrouter.ToolDefinition, allowed []string) []openrouter.ToolDefinition {
	allowSet := make(map[string]bool)
	for _, name := range allowed {
		allowSet[name] = true
	}
	var out []openrouter.ToolDefinition
	for _, td := range all {
		name := td.Function.Name
		if allowSet[name] && !BlockedTools[name] {
			out = append(out, td)
		}
	}
	return out
}
