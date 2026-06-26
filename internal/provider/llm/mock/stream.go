package mock

import (
	"context"

	"github.com/iannil/jianwu/internal/provider/llm"
)

// NewStream creates a Provider that streams the given tokens via Stream().
// If finalErr is non-nil, Stream sends tokens then a final chunk with Err set.
// Chat() still returns the first response (not affected by streaming setup).
func NewStream(tokens []string, finalErr error) *Provider {
	if tokens == nil {
		tokens = []string{}
	}
	return &Provider{streamTokens: tokens, streamErr: finalErr}
}

// Stream implements llm.Streamer. Returns a channel that yields the preset tokens.
func (p *Provider) Stream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	go func() {
		defer close(ch)
		for _, token := range p.streamTokens {
			select {
			case <-ctx.Done():
				ch <- llm.StreamChunk{Err: ctx.Err(), Done: true}
				return
			case ch <- llm.StreamChunk{Content: token}:
			}
		}
		if p.streamErr != nil {
			ch <- llm.StreamChunk{Err: p.streamErr, Done: true}
		} else {
			ch <- llm.StreamChunk{Done: true}
		}
	}()
	return ch, nil
}
