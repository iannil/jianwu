package engine

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
)

type usageChatter struct {
	response *llm.ChatResponse
	err      error
	chunks   []llm.StreamChunk
}

func (c usageChatter) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return c.response, c.err
}
func (c usageChatter) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, len(c.chunks))
	for _, v := range c.chunks {
		ch <- v
	}
	close(ch)
	return ch, c.err
}
func TestTrackingFailureAndMissing(t *testing.T) {
	tracker := &TokenTracker{}
	failure := errors.New("failed")
	for _, c := range []usageChatter{{response: &llm.ChatResponse{Usage: llm.Usage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10}}, err: failure}, {err: failure}, {response: &llm.ChatResponse{}}} {
		_, _ = NewTrackingChatter(c, tracker).Chat(context.Background(), llm.ChatRequest{})
	}
	u := tracker.Snapshot()
	if u.TotalTokens != 10 || u.CallCount != 3 || u.MissingUsageCalls != 2 {
		t.Fatalf("usage = %+v", u)
	}
}
func TestTrackingConcurrent(t *testing.T) {
	tracker := &TokenTracker{}
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); tracker.Add(&llm.ChatResponse{Usage: llm.Usage{TotalTokens: 2}}) }()
	}
	wg.Wait()
	if u := tracker.Snapshot(); u.TotalTokens != 80 || u.CallCount != 40 {
		t.Fatalf("usage = %+v", u)
	}
}
func TestTrackingStream(t *testing.T) {
	for _, tc := range []struct {
		name          string
		chunks        []llm.StreamChunk
		want, missing int
	}{
		{name: "cumulative reports", chunks: []llm.StreamChunk{{Usage: &llm.Usage{TotalTokens: 3}}, {Usage: &llm.Usage{TotalTokens: 8}}, {Done: true}}, want: 8},
		{name: "missing", chunks: []llm.StreamChunk{{Content: "x"}, {Done: true}}, missing: 1},
		{name: "failure preserves report", chunks: []llm.StreamChunk{{Usage: &llm.Usage{TotalTokens: 5}}, {Done: true, Err: errors.New("fail")}}, want: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tracker := &TokenTracker{}
			ch, err := NewTrackingChatter(usageChatter{chunks: tc.chunks}, tracker).Stream(context.Background(), llm.ChatRequest{})
			if err != nil {
				t.Fatal(err)
			}
			for range ch {
			}
			u := tracker.Snapshot()
			if u.CallCount != 1 || u.TotalTokens != tc.want || u.MissingUsageCalls != tc.missing {
				t.Fatalf("usage = %+v", u)
			}
		})
	}
}

func (c usageChatter) Embed(context.Context, llm.EmbedRequest) (*llm.EmbedResponse, error) {
	return nil, nil
}
func TestTrackingRetryFallbackAttempts(t *testing.T) {
	failed := usageChatter{response: &llm.ChatResponse{Usage: llm.Usage{TotalTokens: 4}}, err: llm.ErrServer}
	retry := &llm.RetryWrapper{Inner: failed, Config: llm.RetryConfig{MaxAttempts: 2}}
	fallback := &llm.FallbackWrapper{Primary: retry, Fallback: usageChatter{response: &llm.ChatResponse{Usage: llm.Usage{TotalTokens: 7}}}}
	tracker := &TokenTracker{}
	if _, err := NewTrackingChatter(fallback, tracker).Chat(context.Background(), llm.ChatRequest{}); err != nil {
		t.Fatal(err)
	}
	if u := tracker.Snapshot(); u.TotalTokens != 15 || u.CallCount != 3 {
		t.Fatalf("discarded retry usage lost: %+v", u)
	}
}
