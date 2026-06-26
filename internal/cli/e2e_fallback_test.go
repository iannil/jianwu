package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
)

// TestE2EExpandFallbackTakeover verifies that when primary chatter fails
// and a fallback is configured, the fallback is called and the operation succeeds.
func TestE2EExpandFallbackTakeover(t *testing.T) {
	// Build a ProviderDeps where Chatter is a FallbackWrapper:
	// - Primary always returns network error
	// - Fallback returns a valid response (mock success)
	primary := mock.NewError(errors.New("mock primary failure: network error"))
	fallback := mock.New(llm.ChatResponse{Content: "fallback response ok"})
	fw := &llm.FallbackWrapper{
		Primary:  llm.NewRetryWrapper(primary),
		Fallback: llm.NewRetryWrapper(fallback),
	}

	deps := &ProviderDeps{Chatter: fw}

	// Verify the chatter is the FallbackWrapper (not a raw mock).
	_, ok := deps.Chatter.(*llm.FallbackWrapper)
	if !ok {
		t.Fatalf("expected *llm.FallbackWrapper, got %T", deps.Chatter)
	}

	// Actually call Chat — primary should fail, fallback should take over.
	resp, err := deps.Chatter.Chat(context.Background(), llm.ChatRequest{})
	if err != nil {
		t.Fatalf("Chat should succeed via fallback, got: %v", err)
	}
	if resp.Content != "fallback response ok" {
		t.Errorf("expected fallback content, got %q", resp.Content)
	}
}
