package agent

import (
	"context"
	"encoding/json"

	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/openrouter"
	"github.com/hattiebot/hattiebot/internal/registry"
	"github.com/hattiebot/hattiebot/internal/store"
)

func init() {
	registry.RegisterContext("default", func(db *store.DB) (core.ContextSelector, error) {
		return &ContextManager{DB: db}, nil
	})
}

// ContextManager handles selecting the relevant history for the LLM context window.
type ContextManager struct {
	DB *store.DB
}

// SelectHistory returns the most recent N messages for the thread that likely fit within the token limit.
// For now, we use a simple message count limit (e.g. 20) as a proxy for token limit.
func (cm *ContextManager) SelectHistory(ctx context.Context, threadID string) ([]openrouter.Message, error) {
	// Hardcoded limit for now - in future, estimate tokens.
	const MessageLimit = 30 // Keep last 30 messages (~3-5k tokens usually)

	recent, err := cm.DB.RecentMessages(ctx, MessageLimit, threadID)
	if err != nil {
		return nil, err
	}

	var messages []openrouter.Message
	for _, m := range recent {
		msg := openrouter.Message{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		
		if m.ToolCalls != "" {
			var tcs []openrouter.ToolCall
			if err := json.Unmarshal([]byte(m.ToolCalls), &tcs); err == nil {
				msg.ToolCalls = tcs
			}
		}

		messages = append(messages, msg)
	}
	return messages, nil
}
