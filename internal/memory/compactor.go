package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/openrouter"
)

// Compactor handles summarizing conversation history to save tokens.
type Compactor struct {
	Client    core.LLMClient
	Threshold int
}

func NewCompactor(client core.LLMClient, threshold int) *Compactor {
	if threshold <= 0 {
		threshold = 4000 // Default safe limit for compaction trigger
	}
	return &Compactor{Client: client, Threshold: threshold}
}

// Compact checks if history exceeds the threshold and compacts it if necessary.
// It returns the potentially compacted history and a boolean indicating if compaction occurred.
func (c *Compactor) Compact(ctx context.Context, history []openrouter.Message) ([]openrouter.Message, bool, error) {
	// 1. Estimate tokens (rough estimate: char count / 4)
	totalChars := 0
	for _, m := range history {
		totalChars += len(m.Content)
	}
	estimatedTokens := totalChars / 4

	if estimatedTokens < c.Threshold {
		return history, false, nil
	}

	// 2. Compaction needed.
	// Logic: Summarize the first N messages (excluding the very last few to keep context fresh).
	// Keep last 5 messages intact.
	if len(history) <= 5 {
		return history, false, nil
	}

	keepCount := 5
	toSummarize := history[:len(history)-keepCount]
	kept := history[len(history)-keepCount:]

	// 3. Create summarization prompt
	var sb strings.Builder
	sb.WriteString("Summarize the following conversation history into a concise paragraph that retains key facts, user goals, and tool outputs:\n\n")
	for _, m := range toSummarize {
		sb.WriteString(fmt.Sprintf("%s: %s\n", m.Role, m.Content))
	}

	summaryReq := []openrouter.Message{
		{Role: "system", Content: "You are a helpful assistant efficiently summarizing conversation logs."},
		{Role: "user", Content: sb.String()},
	}

	// 4. Call LLM
	// We use the same client. Note: This consumes tokens too, but results in a smaller block.
	summary, err := c.Client.ChatCompletion(ctx, summaryReq)
	if err != nil {
		return history, false, fmt.Errorf("summarization failed: %w", err)
	}

	// 5. Construct new history
	newHistory := []openrouter.Message{
		{
			Role:    "system",
			Content: fmt.Sprintf("Previous Conversation Summary:\n%s", summary),
		},
	}
	newHistory = append(newHistory, kept...)

	return newHistory, true, nil
}
