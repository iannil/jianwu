package llm

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryWrapperSucceedsOnFirstTry(t *testing.T) {
	inner := &countingChatter{resp: &ChatResponse{Content: "ok"}}
	rw := &RetryWrapper{
		Inner:  inner,
		Config: RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond},
		clock:  &fakeClock{},
	}
	resp, err := rw.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "ok" {
		t.Errorf("got %q", resp.Content)
	}
	if inner.calls != 1 {
		t.Errorf("got %d calls, want 1", inner.calls)
	}
}

func TestRetryWrapperRetriesOnNetworkError(t *testing.T) {
	inner := &countingChatter{
		errs: []error{
			errors.Join(ErrNetwork, errors.New("conn refused")),
			errors.Join(ErrNetwork, errors.New("conn refused")),
			nil, // third succeeds
		},
		resp: &ChatResponse{Content: "finally"},
	}
	rw := &RetryWrapper{
		Inner:  inner,
		Config: RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond},
		clock:  &fakeClock{},
	}
	resp, err := rw.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if resp.Content != "finally" {
		t.Errorf("got %q", resp.Content)
	}
	if inner.calls != 3 {
		t.Errorf("got %d calls, want 3", inner.calls)
	}
}

func TestRetryWrapperDoesNotRetryOn4xx(t *testing.T) {
	inner := &countingChatter{
		errs: []error{errors.Join(ErrLLMProvider, errors.New("bad request"))},
		resp: &ChatResponse{},
	}
	rw := &RetryWrapper{
		Inner:  inner,
		Config: RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond},
		clock:  &fakeClock{},
	}
	_, err := rw.Chat(context.Background(), ChatRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if inner.calls != 1 {
		t.Errorf("got %d calls, want 1 (no retry on 4xx)", inner.calls)
	}
}

func TestRetryWrapperGivesUpAfterMaxAttempts(t *testing.T) {
	inner := &countingChatter{
		errs: []error{
			errors.Join(ErrServer, errors.New("500")),
			errors.Join(ErrServer, errors.New("500")),
			errors.Join(ErrServer, errors.New("500")),
		},
		resp: &ChatResponse{},
	}
	rw := &RetryWrapper{
		Inner:  inner,
		Config: RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond},
		clock:  &fakeClock{},
	}
	_, err := rw.Chat(context.Background(), ChatRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrServer) {
		t.Errorf("expected ErrServer, got %v", err)
	}
	if inner.calls != 3 {
		t.Errorf("got %d calls, want 3", inner.calls)
	}
}

func TestRetryWrapperHonorsCancelledContext(t *testing.T) {
	inner := &countingChatter{
		errs: []error{
			errors.Join(ErrNetwork, errors.New("timeout")),
			errors.Join(ErrNetwork, errors.New("timeout")),
		},
		resp: &ChatResponse{},
	}
	rw := &RetryWrapper{
		Inner:  inner,
		Config: RetryConfig{MaxAttempts: 5, BaseDelay: time.Second},
		clock:  &fakeClock{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately to test ctx checking before sleep
	cancel()
	_, err := rw.Chat(ctx, ChatRequest{})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if inner.calls != 1 {
		t.Errorf("got %d calls, want 1 (first attempt should run)", inner.calls)
	}
}

func TestRetryWrapperEmbedRetriesOnNetworkError(t *testing.T) {
	// countingEmbedder mirrors countingChatter but for Embed
	type countingEmbedder struct {
		errs  []error
		resp  *EmbedResponse
		calls int
	}
	inner := &countingEmbedder{
		errs: []error{
			errors.Join(ErrNetwork, errors.New("conn refused")),
			errors.Join(ErrNetwork, errors.New("conn refused")),
			nil, // third succeeds
		},
		resp: &EmbedResponse{Embeddings: [][]float32{{0.1, 0.2}}},
	}
	// Implement Embed
	embedFunc := func(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
		defer func() { inner.calls++ }()
		if inner.calls < len(inner.errs) {
			if err := inner.errs[inner.calls]; err != nil {
				return nil, err
			}
		}
		return inner.resp, nil
	}
	// Create a wrapper that calls embedFunc via the embedder interface
	rw := &RetryWrapper{
		Inner:  &mockChatterEmbedder{chat: nil, embed: embedFunc},
		Config: RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond},
		clock:  &fakeClock{},
	}
	resp, err := rw.Embed(context.Background(), EmbedRequest{Inputs: []string{"test"}})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if len(resp.Embeddings) != 1 {
		t.Errorf("got %d embeddings, want 1", len(resp.Embeddings))
	}
	if inner.calls != 3 {
		t.Errorf("got %d calls, want 3", inner.calls)
	}
}

// mockChatterEmbedder is a test helper that implements ChatterEmbedder with custom funcs
type mockChatterEmbedder struct {
	chat  func(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	embed func(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)
}

func (m *mockChatterEmbedder) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if m.chat == nil {
		return nil, errors.New("chat not implemented")
	}
	return m.chat(ctx, req)
}

func (m *mockChatterEmbedder) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	if m.embed == nil {
		return nil, errors.New("embed not implemented")
	}
	return m.embed(ctx, req)
}
