package embeddinggood

import (
	"context"

	"github.com/hattiebot/hattiebot/internal/core"
)

// LLMEmbedWrapper wraps an LLMClient and implements EmbeddingClient by calling Embed(ctx, text).
// embedType is ignored (LLM embedding APIs typically do not distinguish query vs document).
func NewLLMEmbedWrapper(client core.LLMClient) core.EmbeddingClient {
	return &llmEmbedWrapper{client: client}
}

type llmEmbedWrapper struct {
	client core.LLMClient
}

func (w *llmEmbedWrapper) Embed(ctx context.Context, text string, _ string) ([]float32, error) {
	return w.client.Embed(ctx, text)
}
