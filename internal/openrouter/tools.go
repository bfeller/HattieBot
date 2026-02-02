package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/hattiebot/hattiebot/internal/core"
)

// ToolDefinition is a function tool for the API (OpenAI-compatible).
type ToolDefinition = core.ToolDefinition

// FunctionSpec describes a callable function.
type FunctionSpec = core.FunctionSpec

// ToolCall is a single tool call from the model.
type ToolCall = core.ToolCall

// apiToolDefinition is the tool shape sent to the API. Policy is omitted so providers
// (e.g. Fireworks via OpenRouter) that reject extra fields do not return 400.
type apiToolDefinition struct {
	Type     string       `json:"type"`
	Function FunctionSpec `json:"function"`
}

// ChatRequestWithTools extends the request with optional tools.
type ChatRequestWithTools struct {
	Model               string                 `json:"model"`
	Messages            []Message              `json:"messages"`
	Tools               []apiToolDefinition   `json:"tools,omitempty"`
	ToolChoice          interface{}           `json:"tool_choice,omitempty"` // "auto" or object
	ProviderParameters map[string]interface{} `json:"provider_parameters,omitempty"` // e.g. enable_thinking: false
	Provider           *struct{ Ignore []string `json:"ignore,omitempty"` } `json:"provider,omitempty"` // skip provider that returned the error
}

// openRouterErrorBody is the shape of a 400 response from OpenRouter (error.metadata.provider_name).
type openRouterErrorBody struct {
	Error *struct {
		Metadata *struct {
			ProviderName string `json:"provider_name"`
		} `json:"metadata"`
	} `json:"error"`
}

// parseProviderNameFromErrorBody reads error.metadata.provider_name from an OpenRouter 400 response body.
// Returns empty string if metadata or provider_name is missing.
func parseProviderNameFromErrorBody(bodyBytes []byte) string {
	var body openRouterErrorBody
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return ""
	}
	if body.Error == nil || body.Error.Metadata == nil {
		return ""
	}
	return strings.TrimSpace(body.Error.Metadata.ProviderName)
}

// providerDisplayNameToSlug converts OpenRouter's display name (e.g. "Moonshot AI") to a provider slug for provider.ignore (e.g. "moonshotai").
func providerDisplayNameToSlug(displayName string) string {
	s := strings.ToLower(strings.TrimSpace(displayName))
	s = strings.ReplaceAll(s, " ", "")
	return s
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
	// Strip policy from tools so provider APIs (e.g. Fireworks) don't reject the request.
	apiTools := make([]apiToolDefinition, len(tools))
	for i := range tools {
		apiTools[i] = apiToolDefinition{Type: tools[i].Type, Function: tools[i].Function}
	}

	// Load time-limited blocked providers for this model (Phase 2: re-enter rotation after cooldown).
	var blockedSlugs []string
	if c.ConfigDir != "" {
		blocked, err := LoadBlockedProviders(c.ConfigDir, c.Model)
		if err != nil {
			log.Printf("[OPENROUTER] Failed to load provider failures: %v", err)
		} else if len(blocked) > 0 {
			blockedSlugs = blocked
			log.Printf("[OPENROUTER] Excluding %d provider(s) until cooldown expires: %v", len(blockedSlugs), blockedSlugs)
		}
	}

	// Retry logic with exponential backoff; on "reasoning_content" 400 we retry with thinking disabled, then skip the provider from the error response.
	maxRetries := 3
	backoff := 1 * time.Second
	var resp *http.Response
	var lastErr error
	var bodyBytes []byte
	disableThinking := false
	var ignoreProviderSlug string

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("[OPENROUTER] Retry %d/%d after %v...", attempt, maxRetries, backoff)
			time.Sleep(backoff)
			backoff *= 2
		}

		body := ChatRequestWithTools{
			Model:      c.Model,
			Messages:   messages,
			Tools:      apiTools,
			ToolChoice: nil,
		}
		if len(tools) > 0 {
			body.ToolChoice = "auto"
		}
		if disableThinking {
			body.ProviderParameters = map[string]interface{}{"enable_thinking": false}
			log.Printf("[OPENROUTER] Retrying with enable_thinking=false")
		}
		// Merge time-limited blocked providers with current retry's ignore (if any).
		ignoreList := make([]string, 0, len(blockedSlugs)+1)
		seen := make(map[string]bool)
		for _, s := range blockedSlugs {
			if s != "" && !seen[s] {
				ignoreList = append(ignoreList, s)
				seen[s] = true
			}
		}
		if ignoreProviderSlug != "" && !seen[ignoreProviderSlug] {
			ignoreList = append(ignoreList, ignoreProviderSlug)
			seen[ignoreProviderSlug] = true
		}
		if len(ignoreList) > 0 {
			body.Provider = &struct{ Ignore []string `json:"ignore,omitempty"` }{Ignore: ignoreList}
			if ignoreProviderSlug != "" {
				log.Printf("[OPENROUTER] Retrying with provider.ignore=%s", ignoreProviderSlug)
			}
		}
		raw, err := json.Marshal(body)
		if err != nil {
			return "", nil, err
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
		// Retry with enable_thinking=false, then provider.ignore=<provider from error> on provider validation 400.
		if resp.StatusCode == http.StatusBadRequest && attempt < maxRetries {
			bodyStr := string(bodyBytes)
			if (strings.Contains(bodyStr, "Provider returned error") &&
				(strings.Contains(bodyStr, "reasoning_content") || strings.Contains(bodyStr, "thinking"))) {
				if !disableThinking {
					disableThinking = true
					log.Printf("[OPENROUTER] Provider validation 400 (reasoning_content/thinking); retrying with enable_thinking=false")
					continue
				}
				if ignoreProviderSlug == "" {
					displayName := parseProviderNameFromErrorBody(bodyBytes)
					if displayName != "" {
						ignoreProviderSlug = providerDisplayNameToSlug(displayName)
						if ignoreProviderSlug != "" {
							// Record failure so we exclude this provider for this model until cooldown expires (Phase 2).
							blockedUntil := time.Now().Add(DefaultProviderCooldown)
							if err := RecordProviderFailure(c.ConfigDir, c.Model, ignoreProviderSlug, blockedUntil); err != nil {
								log.Printf("[OPENROUTER] Failed to record provider failure: %v", err)
							}
							log.Printf("[OPENROUTER] Still 400 after enable_thinking=false; retrying with provider.ignore=%s (from error response); cooldown until %s", ignoreProviderSlug, blockedUntil.Format(time.RFC3339))
							continue
						}
					}
					// No provider_name in response; cannot retry with provider.ignore
					log.Printf("[OPENROUTER] Still 400 after enable_thinking=false; no provider_name in error response")
				}
			}
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

