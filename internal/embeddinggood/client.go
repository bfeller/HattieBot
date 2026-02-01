package embeddinggood

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client calls an EmbeddingGood-compatible HTTP API.
// See https://github.com/bfeller/EmbeddingGood: POST /embed, header x-api-key,
// body { "input": string|string[], "type": "query"|"document", "dimension": 128|256|512|768 },
// response { "embeddings": number[][], "dimension": number }.
type Client struct {
	BaseURL   string
	APIKey    string
	Dimension int
	HTTP      *http.Client
}

// NewClient creates a client for the given base URL (e.g. http://embeddinggood:8000), API key, and dimension (128, 256, 512, or 768).
func NewClient(baseURL, apiKey string, dimension int) *Client {
	if dimension <= 0 {
		dimension = 768
	}
	return &Client{
		BaseURL:   strings.TrimSuffix(baseURL, "/"),
		APIKey:    apiKey,
		Dimension: dimension,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
	}
}

// EmbedRequest is the request body for POST /embed.
type EmbedRequest struct {
	Input     interface{} `json:"input"`     // string or []string
	Type      string      `json:"type"`      // "query" or "document"
	Dimension int         `json:"dimension"` // 128, 256, 512, 768
}

// EmbedResponse is the response from POST /embed.
type EmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"` // JSON numbers decode as float64
	Dimension  int         `json:"dimension"`
}

// Embed calls POST {BaseURL}/embed and returns the first embedding vector.
// embedType should be "document" for memorize and "query" for recall_memories.
func (c *Client) Embed(ctx context.Context, text string, embedType string) ([]float32, error) {
	if c.BaseURL == "" {
		return nil, fmt.Errorf("embeddinggood: base URL not set")
	}
	if c.APIKey == "" {
		return nil, fmt.Errorf("embeddinggood: API key not set")
	}
	if embedType == "" {
		embedType = "document"
	}
	dim := c.Dimension
	if dim <= 0 {
		dim = 768
	}
	body := EmbedRequest{
		Input:     text,
		Type:      embedType,
		Dimension: dim,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	url := c.BaseURL + "/embed"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embeddinggood: HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var out EmbedResponse
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		return nil, fmt.Errorf("embeddinggood: decode response: %w", err)
	}
	if len(out.Embeddings) == 0 {
		return nil, fmt.Errorf("embeddinggood: no embeddings in response")
	}
	vec64 := out.Embeddings[0]
	vec := make([]float32, len(vec64))
	for i, v := range vec64 {
		vec[i] = float32(v)
	}
	return vec, nil
}
