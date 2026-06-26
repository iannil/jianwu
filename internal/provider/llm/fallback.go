package llm

import (
	"context"
	"fmt"
)

// FallbackWrapper tries Primary first; on any non-nil error, tries Fallback.
// If both fail, returns the fallback's error (last attempt).
// Wrap Primary and Fallback in RetryWrapper if you want retry-then-fallback per Q7.
type FallbackWrapper struct {
	Primary  ChatterEmbedder
	Fallback ChatterEmbedder
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

func (fw *FallbackWrapper) Stream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	primaryStreamer, ok := fw.Primary.(Streamer)
	if !ok {
		return fw.streamFallback(ctx, req)
	}
	ch, err := primaryStreamer.Stream(ctx, req)
	if err != nil {
		return fw.streamFallback(ctx, req)
	}
	return ch, nil
}

func (fw *FallbackWrapper) streamFallback(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	fallbackStreamer, ok := fw.Fallback.(Streamer)
	if !ok {
		return nil, fmt.Errorf("neither primary nor fallback support streaming")
	}
	return fallbackStreamer.Stream(ctx, req)
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
