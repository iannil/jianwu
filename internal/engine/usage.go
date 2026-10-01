package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
)

// TokenUsage is the book-owned usage snapshot.
type TokenUsage = book.TokenUsage

// TokenTracker accumulates usage safely across concurrent calls.
type TokenTracker struct {
	mu    sync.Mutex
	usage TokenUsage
}

// Add records a call, including a failed call with no reported usage.
func (t *TokenTracker) Add(resp *llm.ChatResponse) {
	var u *llm.Usage
	if resp != nil {
		u = &resp.Usage
	}
	t.addUsage(u)
}
func (t *TokenTracker) addUsage(u *llm.Usage) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.usage.CallCount++
	if u == nil || (u.PromptTokens == 0 && u.CompletionTokens == 0 && u.TotalTokens == 0 && !u.Cached) {
		t.usage.MissingUsageCalls++
		return
	}
	t.usage.PromptTokens += u.PromptTokens
	t.usage.CompletionTokens += u.CompletionTokens
	t.usage.TotalTokens += u.TotalTokens
	if u.TotalTokens == 0 {
		t.usage.TotalTokens += u.PromptTokens + u.CompletionTokens
	}
	if u.Cached {
		t.usage.CachedCount++
	}
}

// Snapshot returns a copy of the recorded totals.
func (t *TokenTracker) Snapshot() TokenUsage { t.mu.Lock(); defer t.mu.Unlock(); return t.usage }

// Reset clears the recorded totals.
func (t *TokenTracker) Reset() { t.mu.Lock(); defer t.mu.Unlock(); t.usage = TokenUsage{} }

// TrackingChatter records responses, including reported usage returned alongside errors.
type TrackingChatter struct {
	inner     llm.Chatter
	tracker   *TokenTracker
	delegated bool
}

// NewTrackingChatter wraps a chatter with usage accounting.
func NewTrackingChatter(inner llm.Chatter, tracker *TokenTracker) *TrackingChatter {
	// Place counters inside retry/fallback wrappers so discarded attempts count too.
	switch v := inner.(type) {
	case *llm.RetryWrapper:
		copy := *v
		copy.Inner = trackingProvider{TrackingChatter: NewTrackingChatter(v.Inner, tracker), Embedder: v.Inner}
		return &TrackingChatter{inner: &copy, tracker: tracker, delegated: true}
	case *llm.FallbackWrapper:
		copy := *v
		copy.Primary = trackingProvider{TrackingChatter: NewTrackingChatter(v.Primary, tracker), Embedder: v.Primary}
		copy.Fallback = trackingProvider{TrackingChatter: NewTrackingChatter(v.Fallback, tracker), Embedder: v.Fallback}
		return &TrackingChatter{inner: &copy, tracker: tracker, delegated: true}
	}
	return &TrackingChatter{inner: inner, tracker: tracker}
}

type trackingProvider struct {
	*TrackingChatter
	llm.Embedder
}

// Chat implements llm.Chatter.
func (t *TrackingChatter) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	resp, err := t.inner.Chat(ctx, req)
	if !t.delegated {
		t.tracker.Add(resp)
	}
	return resp, err
}

// Stream records the latest cumulative usage once, before forwarding the terminal chunk.
func (t *TrackingChatter) Stream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	inner, ok := t.inner.(llm.Streamer)
	if !ok {
		return nil, fmt.Errorf("provider does not support streaming")
	}
	if t.delegated {
		return inner.Stream(ctx, req)
	}
	input, err := inner.Stream(ctx, req)
	if err != nil {
		t.tracker.Add(nil)
		return nil, err
	}
	output := make(chan llm.StreamChunk)
	go func() {
		defer close(output)
		var usage *llm.Usage
		recorded := false
		record := func() {
			if !recorded {
				t.tracker.addUsage(usage)
				recorded = true
			}
		}
		defer record()
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-input:
				if !ok {
					return
				}
				if chunk.Usage != nil {
					copy := *chunk.Usage
					usage = &copy
				}
				if chunk.Done || chunk.Err != nil {
					record()
				}
				select {
				case output <- chunk:
				case <-ctx.Done():
					return
				}
				if chunk.Done || chunk.Err != nil {
					return
				}
			}
		}
	}()
	return output, nil
}
