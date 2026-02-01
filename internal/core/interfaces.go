package core

import (
	"context"
)

// LLMClient abstracts the low-level API client (OpenRouter, local LLM, etc).
type LLMClient interface {
	ChatCompletion(ctx context.Context, messages []Message) (string, error)
	ChatCompletionWithTools(ctx context.Context, messages []Message, tools []ToolDefinition) (string, []ToolCall, error)
	Embed(ctx context.Context, text string) ([]float32, error)
}

// EmbeddingClient abstracts embedding APIs (EmbeddingGood, OpenRouter, etc).
// embedType is "document" or "query" for providers that distinguish them (e.g. EmbeddingGood).
type EmbeddingClient interface {
	Embed(ctx context.Context, text string, embedType string) ([]float32, error)
}

// ContextSelector decides which messages from history to include in the prompt.
type ContextSelector interface {
	SelectHistory(ctx context.Context, threadID string) ([]Message, error)
}

// ToolExecutor abstracts tool execution.
type ToolExecutor interface {
	Execute(ctx context.Context, name, argsJSON string) (string, error)
	SetSpawner(spawner SubmindSpawner)
}

// SubMindConfig defines a sub-mind mode.
type SubMindConfig struct {
	Name         string   `json:"name"`
	SystemPrompt string   `json:"system_prompt"`
	AllowedTools []string `json:"allowed_tools"`
	MaxTurns     int      `json:"max_turns"`
	Protected    bool     `json:"protected"` // Cannot be deleted by agent
}

// SubMindResult is the output of a sub-mind execution.
type SubMindResult struct {
	Success   bool   `json:"success"`
	Output    string `json:"output"`    // Final assistant response
	Error     string `json:"error"`     // If failed
	Turns     int    `json:"turns"`     // How many iterations ran
	Truncated bool   `json:"truncated"` // Hit MaxTurns limit
	SessionID int64  `json:"session_id,omitempty"` // Set for new sessions so caller can resume later
}

// SubmindSpawner spawns isolated LLM contexts for focused tasks.
// userID scopes the session; sessionID 0 = new session, non-zero = resume.
type SubmindSpawner interface {
	SpawnSubmind(ctx context.Context, userID, mode, task string, sessionID int64) (SubMindResult, error)
}

// SubmindRegistry manages sub-mind configurations.
type SubmindRegistry interface {
	Get(name string) (SubMindConfig, bool)
	Add(config SubMindConfig) error
	Delete(name string) error
	List() []SubMindConfig
}
