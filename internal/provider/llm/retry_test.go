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

