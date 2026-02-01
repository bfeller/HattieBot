package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/registry"
)

func init() {
	registry.RegisterClient("default", func(apiKey, model string) (core.LLMClient, error) {
		return NewClient(apiKey, model), nil
	})
}

const BaseURL = "https://openrouter.ai/api/v1"

// parseContent parses API content that may be string, null, or array of parts (e.g. [{"type":"text","text":"..."}]).
func parseContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Try array of parts with type+text
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, p := range parts {
			if p.Type == "text" {
				b.WriteString(p.Text)
			}
		}
		return b.String()
	}
	return parseContentArrayGeneric(raw)
}

// parseContentArrayGeneric extracts text from an array of objects that may have "text" key (e.g. OpenRouter/Kimi).
func parseContentArrayGeneric(raw json.RawMessage) string {
	var parts []map[string]interface{}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return ""
	}
	var b strings.Builder
	for _, p := range parts {
		if t, ok := p["text"].(string); ok {
			b.WriteString(t)
		}
	}
	return b.String()
}

// Message represents a chat message (OpenRouter/OpenAI format).
type Message = core.Message

// ChatRequest is the request body for chat completions.
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// ChatResponse is the response from chat completions.
type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content json.RawMessage `json:"content"`
			Role    string          `json:"role"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Client calls OpenRouter API.
type Client struct {
	APIKey string
	Model  string
	HTTP   *http.Client
}

// NewClient creates a client with the given API key and model.
func NewClient(apiKey, model string) *Client {
	return &Client{
		APIKey: apiKey,
		Model:  model,
		HTTP:   http.DefaultClient,
	}
}

// ChatCompletion sends messages to OpenRouter and returns the assistant reply content.
func (c *Client) ChatCompletion(ctx context.Context, messages []Message) (string, error) {
	if c.APIKey == "" {
		return "", fmt.Errorf("openrouter: API key not set")
	}
	if c.Model == "" {
		return "", fmt.Errorf("openrouter: model not set")
	}
	body := ChatRequest{Model: c.Model, Messages: messages}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	// Exponential backoff retry for network/rate limits
	var resp *http.Response
	var errDo error
	maxRetries := 3
	backoff := 1 * time.Second

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			time.Sleep(backoff)
			backoff *= 2
		}
		resp, errDo = c.HTTP.Do(req)
		if errDo != nil {
			// Network error, maybe retry?
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			resp.Body.Close()
			continue
		}
		break
	}
	if errDo != nil {
		return "", errDo
	}
	if resp == nil {
		return "", fmt.Errorf("openrouter: request failed after retries")
	}

	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openrouter: HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}
	var out ChatResponse
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		return "", fmt.Errorf("openrouter: decode: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("openrouter: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("openrouter: no choices in response")
	}
	rawContent := out.Choices[0].Message.Content
	content := parseContent(rawContent)
	if content == "" && len(rawContent) > 0 && rawContent[0] == '[' {
		content = parseContentArrayGeneric(rawContent)
	}
	return content, nil
}

// RewriteAsSystemPurpose sends the user's description to the LLM and returns the cleaned system-purpose text.
func (c *Client) RewriteAsSystemPurpose(ctx context.Context, userDescription string) (string, error) {
	systemPrompt := "Rewrite the following as a concise, professional system-purpose statement for an AI assistant. Preserve name and core purpose. Output only the rewritten text, no preamble."
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userDescription},
	}
	return c.ChatCompletion(ctx, messages)

}

// EmbeddingRequest is the request body for embeddings.
type EmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// EmbeddingResponse is the response from embeddings.
type EmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Embed generates embeddings for the given text using text-embedding-3-small.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("openrouter: API key not set")
	}
	
	model := "text-embedding-3-small"
	body := EmbeddingRequest{
		Model: model,
		Input: text,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://openrouter.ai/api/v1/embeddings", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("HTTP-Referer", "https://hattiebot.local") 
	req.Header.Set("X-Title", "HattieBot")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter embeddings: HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}
	
	var out EmbeddingResponse
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		return nil, fmt.Errorf("decode embeddings: %w", err)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("api error: %s", out.Error.Message)
	}
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("no embedding data")
	}
	return out.Data[0].Embedding, nil
}
