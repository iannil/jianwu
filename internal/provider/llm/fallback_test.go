package llm

import (
	"context"
	"errors"
	"testing"
)

func TestFallbackUsesPrimaryOnSuccess(t *testing.T) {
	primary := &countingChatter{resp: &ChatResponse{Content: "from-primary"}}
	fallback := &countingChatter{resp: &ChatResponse{Content: "from-fallback"}}
	fw := &FallbackWrapper{Primary: primary, Fallback: fallback}
	resp, err := fw.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "from-primary" {
		t.Errorf("got %q, want from-primary", resp.Content)
	}
	if fallback.calls != 0 {
		t.Errorf("fallback should not be called")
	}
}

func TestFallbackSwitchesOnPrimaryFailure(t *testing.T) {
	primary := &countingChatter{
		errs: []error{errors.Join(ErrServer, errors.New("500"))},
		resp: &ChatResponse{},
	}
	fallback := &countingChatter{resp: &ChatResponse{Content: "from-fallback"}}
	fw := &FallbackWrapper{Primary: primary, Fallback: fallback}
	resp, err := fw.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "from-fallback" {
		t.Errorf("got %q, want from-fallback", resp.Content)
	}
	if primary.calls != 1 {
		t.Errorf("primary calls: %d, want 1", primary.calls)
	}
	if fallback.calls != 1 {
		t.Errorf("fallback calls: %d, want 1", fallback.calls)
	}
}

func TestFallbackReturnsErrorWhenBothFail(t *testing.T) {
	primary := &countingChatter{
		errs: []error{errors.Join(ErrServer, errors.New("p 500"))},
		resp: &ChatResponse{},
	}
	fallback := &countingChatter{
		errs: []error{errors.Join(ErrServer, errors.New("f 500"))},
		resp: &ChatResponse{},
	}
	fw := &FallbackWrapper{Primary: primary, Fallback: fallback}
	_, err := fw.Chat(context.Background(), ChatRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrServer) {
		t.Errorf("expected ErrServer, got %v", err)
	}
}
