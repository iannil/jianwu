package llm

import (
	"context"
	"fmt"
)

// FallbackWrapper tries Primary first; on any non-nil error, tries Fallback.
// If both fail, returns the fallback's error (last attempt).
// Wrap Primary and Fallback in RetryWrapper if you want retry-then-fallback per Q7.
type FallbackWrapper struct {
	Primary  chatterEmbedder
	Fallback chatterEmbedder
}

func (fw *FallbackWrapper) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	resp, err := fw.Primary.Chat(ctx, req)
	if err == nil {
		return resp, nil
	}
	// Primary failed. Try fallback.
	primaryErr := err
	resp2, err2 := fw.Fallback.Chat(ctx, req)
	if err2 == nil {
		return resp2, nil
	}
	return nil, fmt.Errorf("primary: %v; fallback: %w", primaryErr, err2)
}

func (fw *FallbackWrapper) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	resp, err := fw.Primary.Embed(ctx, req)
	if err == nil {
		return resp, nil
	}
	// Primary failed. Try fallback.
	primaryErr := err
	resp2, err2 := fw.Fallback.Embed(ctx, req)
	if err2 == nil {
		return resp2, nil
	}
	return nil, fmt.Errorf("primary: %v; fallback: %w", primaryErr, err2)
}
