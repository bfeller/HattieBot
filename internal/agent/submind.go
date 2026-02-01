package agent

import (
	"context"
	"fmt"
	"log"

	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/openrouter"
	"github.com/hattiebot/hattiebot/internal/store"
	"github.com/hattiebot/hattiebot/internal/tools"
)

// SubMind runs an isolated LLM session.
type SubMind struct {
	Config   core.SubMindConfig
	Client   core.LLMClient
	Executor core.ToolExecutor
	LogStore *store.LogStore
}

// Run executes the sub-mind with the given task (no persistence).
func (s *SubMind) Run(ctx context.Context, task string) (core.SubMindResult, error) {
	return s.RunWithSession(ctx, task, 0, "", nil)
}

// RunWithSession runs the sub-mind with optional persistence. When sessionID > 0 and db != nil,
// loads session from db (userID required), checkpoints after each turn, and saves final status.
func (s *SubMind) RunWithSession(ctx context.Context, task string, sessionID int64, userID string, db *store.DB) (core.SubMindResult, error) {
	result := core.SubMindResult{
		Success: false,
		Turns:   0,
	}
	if sessionID > 0 {
		result.SessionID = sessionID
	}

	// Log start
	if s.LogStore != nil {
		s.LogStore.LogInfo("submind", fmt.Sprintf("started mode=%s task_len=%d", s.Config.Name, len(task)))
	}

	maxTurns := s.Config.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 10 // Default
	}

	// Build filtered tool definitions
	allTools := tools.BuiltinToolDefs()
	filteredTools := tools.FilterToolDefs(allTools, s.Config.AllowedTools)
	filteredExecutor := tools.NewFilteredExecutor(s.Executor, s.Config.AllowedTools)

	var messages []openrouter.Message
	if sessionID > 0 && db != nil && userID != "" {
		// Resume: load session
		ses, err := db.GetSubmindSession(ctx, sessionID, userID)
		if err != nil {
			return result, err
		}
		loaded := ses.Messages()
		if loaded != nil {
			messages = make([]openrouter.Message, len(loaded))
			for i, m := range loaded {
				messages[i] = openrouter.Message(m)
			}
		}
		if messages == nil {
			messages = []openrouter.Message{}
		}
		result.Turns = ses.Turns
	} else {
		// New run
		messages = []openrouter.Message{
			{Role: "system", Content: s.Config.SystemPrompt},
			{Role: "user", Content: task},
		}
	}

	var content string
	var toolCalls []openrouter.ToolCall

	for result.Turns < maxTurns {
		result.Turns++

		// Call LLM with tools
		var err error
		content, toolCalls, err = s.Client.ChatCompletionWithTools(ctx, messages, filteredTools)
		if err != nil {
			result.Error = fmt.Sprintf("LLM error: %v", err)
			if s.LogStore != nil {
				s.LogStore.LogError("submind", fmt.Sprintf("failed mode=%s error=%v", s.Config.Name, err))
			}
			if sessionID > 0 && db != nil {
				_ = db.UpdateSubmindSession(ctx, sessionID, toCoreMessages(messages), result.Turns, "failed", "", result.Error)
			}
			return result, nil
		}

		// No tool calls = done
		if len(toolCalls) == 0 {
			result.Success = true
			result.Output = content
			if sessionID > 0 && db != nil {
				final := append(messages, openrouter.Message{Role: "assistant", Content: content})
				_ = db.UpdateSubmindSession(ctx, sessionID, toCoreMessages(final), result.Turns, "completed", result.Output, "")
			}
			break
		}

		// Append assistant message with tool calls
		messages = append(messages, openrouter.Message{
			Role:      "assistant",
			Content:   content,
			ToolCalls: toolCalls,
		})

		// Execute each tool call
		for _, tc := range toolCalls {
			toolResult, _ := filteredExecutor.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
			messages = append(messages, openrouter.Message{
				Role:       "tool",
				Content:    toolResult,
				ToolCallID: tc.ID,
			})
		}

		// Checkpoint
		if sessionID > 0 && db != nil {
			_ = db.UpdateSubmindSession(ctx, sessionID, toCoreMessages(messages), result.Turns, "running", "", "")
		}
	}

	// Max turns without success
	if result.Turns >= maxTurns && !result.Success {
		result.Truncated = true
		result.Success = true
		result.Output = content
		log.Printf("[SUBMIND] mode=%s hit max_turns=%d", s.Config.Name, maxTurns)
		if sessionID > 0 && db != nil {
			_ = db.UpdateSubmindSession(ctx, sessionID, toCoreMessages(messages), result.Turns, "completed", result.Output, "")
		}
	}

	if s.LogStore != nil {
		s.LogStore.LogInfo("submind", fmt.Sprintf("completed mode=%s turns=%d success=%v truncated=%v",
			s.Config.Name, result.Turns, result.Success, result.Truncated))
	}
	return result, nil
}

func toCoreMessages(msgs []openrouter.Message) []core.Message {
	out := make([]core.Message, len(msgs))
	for i, m := range msgs {
		out[i] = core.Message(m)
	}
	return out
}
