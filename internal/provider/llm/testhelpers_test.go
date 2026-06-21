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

// fakeClock lets us skip backoff sleeps in tests.
type fakeClock struct{ t time.Duration }

func (c *fakeClock) Sleep(d time.Duration) { c.t += d }
