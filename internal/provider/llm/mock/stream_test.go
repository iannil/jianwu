package mock

import (
	"context"
	"errors"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
)

func TestStreamYieldsTokens(t *testing.T) {
	tokens := []string{"Hello", ", ", "world", "!"}
	p := NewStream(tokens, nil)
	ch, err := p.Stream(context.Background(), llm.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var got string
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("unexpected error: %v", chunk.Err)
		}
		if chunk.Done {
			break
		}
		got += chunk.Content
	}
	if got != "Hello, world!" {
		t.Errorf("got %q, want %q", got, "Hello, world!")
	}
}

func TestStreamEmptyTokens(t *testing.T) {
	p := NewStream(nil, nil)
	ch, err := p.Stream(context.Background(), llm.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	chunk := <-ch
	if !chunk.Done {
		t.Error("expected immediate Done")
	}
}

func TestStreamError(t *testing.T) {
	wantErr := errors.New("mock stream error")
	p := NewStream([]string{"hello"}, wantErr)
	ch, err := p.Stream(context.Background(), llm.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var gotErr error
	var gotContent string
	for chunk := range ch {
		if chunk.Content != "" {
			gotContent += chunk.Content
		}
		if chunk.Err != nil {
			gotErr = chunk.Err
		}
	}
	if gotContent != "hello" {
		t.Errorf("content = %q, want %q", gotContent, "hello")
	}
	if !errors.Is(gotErr, wantErr) {
		t.Errorf("error = %v, want %v", gotErr, wantErr)
	}
}

func TestStreamCancellation(t *testing.T) {
	tokens := []string{"a", "b", "c", "d", "e"}
	p := NewStream(tokens, nil)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := p.Stream(ctx, llm.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	// Read one token, then cancel.
	chunk := <-ch
	if chunk.Content != "a" {
		t.Errorf("first token = %q, want %q", chunk.Content, "a")
	}
	cancel()
	// Remaining tokens should be consumed or skipped; final chunk should have ctx error.
	var sawCancel bool
	for chunk := range ch {
		if chunk.Err != nil {
			sawCancel = true
		}
	}
	if !sawCancel {
		t.Error("expected cancellation error")
	}
}
