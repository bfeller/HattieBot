package llmrouter

import (
	"context"
	"testing"

	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/store"
)

type mockLLMClient struct {
	chatResp    string
	chatErr     error
	embedResp   []float32
	embedErr    error
	chatCalls   int
	embedCalls  int
}

func (m *mockLLMClient) ChatCompletion(ctx context.Context, messages []core.Message) (string, error) {
	m.chatCalls++
	return m.chatResp, m.chatErr
}

func (m *mockLLMClient) ChatCompletionWithTools(ctx context.Context, messages []core.Message, tools []core.ToolDefinition) (string, []core.ToolCall, error) {
	m.chatCalls++
	if m.chatErr != nil {
		return "", nil, m.chatErr
	}
	return m.chatResp, nil, nil
}

func (m *mockLLMClient) Embed(ctx context.Context, text string) ([]float32, error) {
	m.embedCalls++
	return m.embedResp, m.embedErr
}

func TestRouterClient_ConfigButNoAPIKeyUsesFallback(t *testing.T) {
	fallback := &mockLLMClient{chatResp: "fallback"}
	cfg := &store.LLMRoutingConfig{
		LLMProviders: map[string]store.LLMProviderEntry{
			"openrouter": {Type: "openrouter", APIKeyEnv: "TEST_OPENROUTER_KEY"},
		},
		ModelRouting: map[string]store.ModelRouteEntry{
			"default": {Provider: "openrouter", Model: "test-model"},
		},
	}
	getEnv := func(string) string { return "" }
	r := NewRouterClient(cfg, fallback, "", getEnv)
	ctx := context.Background()

	out, err := r.ChatCompletion(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "fallback" {
		t.Errorf("expected fallback response (no API key), got %q", out)
	}
	if fallback.chatCalls != 1 {
		t.Errorf("fallback should have been called once, got %d", fallback.chatCalls)
	}
}

func TestRouterClient_FallbackOnPrimaryError(t *testing.T) {
	// Use unsupported provider type so getClient returns nil and we use fallback
	fallback := &mockLLMClient{chatResp: "fallback"}
	cfg := &store.LLMRoutingConfig{
		LLMProviders: map[string]store.LLMProviderEntry{
			"ollama": {Type: "ollama", BaseURL: "http://localhost:11434"},
		},
		ModelRouting: map[string]store.ModelRouteEntry{
			"default": {Provider: "ollama", Model: "llama3"},
		},
	}
	r := NewRouterClient(cfg, fallback, "", nil)
	ctx := context.Background()

	out, err := r.ChatCompletion(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "fallback" {
		t.Errorf("expected fallback (unsupported type), got %q", out)
	}
}

func TestRouterClient_NoConfigUsesFallback(t *testing.T) {
	fallback := &mockLLMClient{chatResp: "fallback"}
	r := NewRouterClient(nil, fallback, "", nil)
	ctx := context.Background()

	out, err := r.ChatCompletion(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "fallback" {
		t.Errorf("expected fallback response, got %q", out)
	}
}
