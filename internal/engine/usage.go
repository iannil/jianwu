package engine

import (
	"context"
	"sync"

	"github.com/iannil/jianwu/internal/provider/llm"
)

// TokenTracker accumulates token usage across multiple LLM calls.
// Thread-safe for concurrent use.
type TokenTracker struct {
	mu             sync.Mutex
	promptTokens   int
	completionToks int
	totalTokens    int
	callCount      int
	cachedCount    int
}

// Add records a single LLM response's token usage.
func (t *TokenTracker) Add(resp *llm.ChatResponse) {
	if resp == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.promptTokens += resp.Usage.PromptTokens
	t.completionToks += resp.Usage.CompletionTokens
	t.totalTokens += resp.Usage.TotalTokens
	t.callCount++
	if resp.Usage.Cached {
		t.cachedCount++
	}
}

// Snapshot returns a copy of the current totals.
func (t *TokenTracker) Snapshot() TokenUsage {
	t.mu.Lock()
	defer t.mu.Unlock()
	return TokenUsage{
		PromptTokens:     t.promptTokens,
		CompletionTokens: t.completionToks,
		TotalTokens:      t.totalTokens,
		CallCount:        t.callCount,
		CachedCount:      t.cachedCount,
	}
}

// Reset clears the tracker.
func (t *TokenTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.promptTokens = 0
	t.completionToks = 0
	t.totalTokens = 0
	t.callCount = 0
	t.cachedCount = 0
}

// TokenUsage is a snapshot of accumulated token consumption.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	CallCount        int `json:"call_count"`
	CachedCount      int `json:"cached_count"`
}

// TrackingChatter wraps an llm.Chatter and records each response's token usage.
type TrackingChatter struct {
	inner   llm.Chatter
	tracker *TokenTracker
}

// NewTrackingChatter creates a chatter that records usage into the given tracker.
func NewTrackingChatter(inner llm.Chatter, tracker *TokenTracker) *TrackingChatter {
	return &TrackingChatter{inner: inner, tracker: tracker}
}

func (t *TrackingChatter) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	resp, err := t.inner.Chat(ctx, req)
	if t.tracker != nil {
		t.tracker.Add(resp)
	}
	return resp, err
}
