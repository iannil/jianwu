package llm

import (
	"context"
	"time"
)

// countingChatter is a test Chatter that returns errors/responses in sequence.
type countingChatter struct {
	errs  []error
	resp  *ChatResponse
	calls int
}

func (c *countingChatter) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	defer func() { c.calls++ }()
	if c.calls < len(c.errs) {
		if err := c.errs[c.calls]; err != nil {
			return nil, err
		}
	}
	return c.resp, nil
}

func (c *countingChatter) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	// For tests that don't use Embed, just return a simple response
	return &EmbedResponse{Embeddings: [][]float32{{0.1, 0.2}}}, nil
}

// fakeClock lets us skip backoff sleeps in tests.
type fakeClock struct{ t time.Duration }

func (c *fakeClock) Wait(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	c.t += d
	return nil
}
