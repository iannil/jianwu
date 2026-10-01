package llmfactory

import (
	"context"
	"fmt"

	"github.com/iannil/jianwu/internal/provider/llm"
)

// modelProvider binds a configured default at the leaf, so fallback providers
// retain their own models. Requests with an explicit model are passed unchanged.
type modelProvider struct {
	inner llm.ChatterEmbedder
	model string
}

func (p *modelProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if req.Model == "" {
		req.Model = p.model
	}
	return p.inner.Chat(ctx, req)
}

func (p *modelProvider) Embed(ctx context.Context, req llm.EmbedRequest) (*llm.EmbedResponse, error) {
	if req.Model == "" {
		req.Model = p.model
	}
	return p.inner.Embed(ctx, req)
}

func (p *modelProvider) Stream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	streamer, ok := p.inner.(llm.Streamer)
	if !ok {
		return nil, fmt.Errorf("provider does not support streaming")
	}
	if req.Model == "" {
		req.Model = p.model
	}
	return streamer.Stream(ctx, req)
}
