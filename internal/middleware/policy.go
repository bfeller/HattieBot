package middleware

import (
	"context"
	"fmt"

	"github.com/hattiebot/hattiebot/internal/core"
)

// ConfirmationFunc is a callback to ask the user for permission
type ConfirmationFunc func(msg string) (bool, error)

// PolicyMiddleware wraps a ToolExecutor and enforces policies
type PolicyMiddleware struct {
	next       core.ToolExecutor
	confirm    ConfirmationFunc
	toolDefs   map[string]core.ToolDefinition
}

// NewPolicyMiddleware creates a new middleware. 
// It builds a lookup map of tool definitions to check policies at runtime.
func NewPolicyMiddleware(next core.ToolExecutor, tools []core.ToolDefinition, confirm ConfirmationFunc) *PolicyMiddleware {
	defs := make(map[string]core.ToolDefinition)
	for _, t := range tools {
		defs[t.Function.Name] = t
	}
	return &PolicyMiddleware{
		next:     next,
		confirm:  confirm,
		toolDefs: defs,
	}
}

func (m *PolicyMiddleware) Execute(ctx context.Context, toolName string, argsJSON string) (string, error) {
	def, ok := m.toolDefs[toolName]
	
	// If tool not found in definitions, assume it's safe OR fail? 
	// Let's default to safe but log warning, or maybe it's dynamic.
	// For "safe" tools, we just proceed.
	policy := "safe"
	if ok {
		policy = def.Policy
	}

	if policy == "restricted" || policy == "admin_only" {
		// Ask for confirmation
		if m.confirm != nil {
			approved, err := m.confirm(fmt.Sprintf("Allow tool '%s'? Policy: %s", toolName, policy))
			if err != nil {
				return "", fmt.Errorf("confirmation error: %w", err)
			}
			if !approved {
				return "Error: User denied permission to execute this tool.", nil
			}
		}
	}

	return m.next.Execute(ctx, toolName, argsJSON)
}

func (m *PolicyMiddleware) SetSpawner(spawner core.SubmindSpawner) {
	m.next.SetSpawner(spawner)
}
