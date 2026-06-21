package llm

import (
	"context"
	"errors"
	"testing"
)

// stubChatter is a minimal Chatter for testing the interface contract.
type stubChatter struct {
	resp *ChatResponse
	err  error
}

func (s *stubChatter) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return s.resp, s.err
}

func TestChatterInterface(t *testing.T) {
	var c Chatter = &stubChatter{
		resp: &ChatResponse{Content: "hello", TokensIn: 5, TokensOut: 1},
	}
	resp, err := c.Chat(context.Background(), ChatRequest{Model: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello" {
		t.Errorf("got %q, want hello", resp.Content)
	}
}

func TestClassifyErrorNetwork(t *testing.T) {
	err := ClassifyError(errors.New("connection refused"), 0)
	if !errors.Is(err, ErrNetwork) {
		t.Errorf("expected ErrNetwork, got %v", err)
	}
}

func TestClassifyErrorRateLimit(t *testing.T) {
	err := ClassifyError(errors.New("too many requests"), 429)
	if !errors.Is(err, ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got %v", err)
	}
}

func TestClassifyErrorClient(t *testing.T) {
	err := ClassifyError(errors.New("bad request"), 400)
	if !errors.Is(err, ErrLLMProvider) {
		t.Errorf("expected ErrLLMProvider, got %v", err)
	}
}

func TestClassifyErrorServer(t *testing.T) {
	err := ClassifyError(errors.New("internal"), 500)
	if !errors.Is(err, ErrServer) {
		t.Errorf("expected ErrServer, got %v", err)
	}
}
