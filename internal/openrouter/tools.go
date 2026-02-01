package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/hattiebot/hattiebot/internal/core"
)

// ToolDefinition is a function tool for the API (OpenAI-compatible).
type ToolDefinition = core.ToolDefinition

// FunctionSpec describes a callable function.
type FunctionSpec = core.FunctionSpec

// ToolCall is a single tool call from the model.
type ToolCall = core.ToolCall

// ChatRequestWithTools extends the request with optional tools.
type ChatRequestWithTools struct {
	Model      string           `json:"model"`
	Messages   []Message        `json:"messages"`
	Tools      []ToolDefinition `json:"tools,omitempty"`
	ToolChoice interface{}      `json:"tool_choice,omitempty"` // "auto" or object
}

// ChatResponseWithTools includes tool_calls in the choice message.
type ChatResponseWithTools struct {
	Choices []struct {
		Message struct {
			Content   json.RawMessage `json:"content"`
			Role      string          `json:"role"`
			ToolCalls []ToolCall      `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ChatCompletionWithTools sends messages and optional tools; returns content and any tool_calls.
// Includes retry logic with exponential backoff for transient errors (5xx, network errors).
func (c *Client) ChatCompletionWithTools(ctx context.Context, messages []Message, tools []ToolDefinition) (content string, toolCalls []ToolCall, err error) {
	if c.APIKey == "" {
		return "", nil, fmt.Errorf("openrouter: API key not set")
	}
	if c.Model == "" {
		return "", nil, fmt.Errorf("openrouter: model not set")
	}
	body := ChatRequestWithTools{
		Model:    c.Model,
		Messages: messages,
		Tools:    tools,
	}
	if len(tools) > 0 {
		body.ToolChoice = "auto"
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", nil, err
	}

	// Retry logic with exponential backoff
	maxRetries := 3
	backoff := 1 * time.Second
	var resp *http.Response
	var lastErr error
	var bodyBytes []byte

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("[OPENROUTER] Retry %d/%d after %v...", attempt, maxRetries, backoff)
			time.Sleep(backoff)
			backoff *= 2
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, BaseURL+"/chat/completions", bytes.NewReader(raw))
		if err != nil {
			return "", nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.APIKey)

		resp, lastErr = c.HTTP.Do(req)
		if lastErr != nil {
			// Network error, retry
			log.Printf("[OPENROUTER] Network error: %v", lastErr)
			continue
		}

		bodyBytes, _ = io.ReadAll(resp.Body)
		resp.Body.Close()

		// Retry on 5xx or 429 (rate limit)
		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			log.Printf("[OPENROUTER] Retryable error: HTTP %d", resp.StatusCode)
			continue
		}

		// Success or non-retryable error, break
		break
	}

	if lastErr != nil {
		return "", nil, fmt.Errorf("openrouter: request failed after %d retries: %w", maxRetries, lastErr)
	}
	if resp == nil {
		return "", nil, fmt.Errorf("openrouter: request failed after %d retries", maxRetries)
	}

	var out ChatResponseWithTools
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		return "", nil, fmt.Errorf("openrouter: decode: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("openrouter: HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}
	if out.Error != nil {
		return "", nil, fmt.Errorf("openrouter: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", nil, fmt.Errorf("openrouter: no choices in response (body: %s)", string(bodyBytes))
	}
	msg := out.Choices[0].Message
	content = parseContent(msg.Content)
	if content == "" && len(msg.Content) > 0 && msg.Content[0] == '[' {
		content = parseContentArrayGeneric(msg.Content)
	}
	return content, msg.ToolCalls, nil
}

