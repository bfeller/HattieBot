package middleware

import (
	"context"

	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/tools"
)

// TruncatingExecutor wraps a ToolExecutor and truncates tool output to maxRunes (0 = no truncation).
type TruncatingExecutor struct {
	next      core.ToolExecutor
	maxRunes  int
}

// NewTruncatingExecutor returns an executor that truncates results from next.
func NewTruncatingExecutor(next core.ToolExecutor, maxRunes int) *TruncatingExecutor {
	return &TruncatingExecutor{next: next, maxRunes: maxRunes}
}

// Execute runs the inner executor and truncates the result before returning.
func (t *TruncatingExecutor) Execute(ctx context.Context, name, argsJSON string) (string, error) {
	result, err := t.next.Execute(ctx, name, argsJSON)
	if err != nil {
		return "", err
	}
	return tools.TruncateToolOutput(result, t.maxRunes), nil
}

func (t *TruncatingExecutor) SetSpawner(spawner core.SubmindSpawner) {
	t.next.SetSpawner(spawner)
}
