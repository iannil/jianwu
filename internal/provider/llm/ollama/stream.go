package ollama

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/iannil/jianwu/internal/provider/llm"
)

// Stream implements llm.Streamer via Ollama's SSE streaming.
func (p *Provider) Stream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	body := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}

	resp, err := p.do(ctx, p.streamClient, "/api/chat", body)
	if err != nil {
		return nil, llm.ClassifyError(err, 0)
	}

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, llm.ClassifyError(fmt.Errorf("ollama: %s", string(b)), resp.StatusCode)
	}

	ch := make(chan llm.StreamChunk)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			var chunk chatResponse
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				continue
			}
			if chunk.Message.Content != "" {
				select {
				case <-ctx.Done():
					ch <- llm.StreamChunk{Err: ctx.Err(), Done: true}
					return
				case ch <- llm.StreamChunk{Content: chunk.Message.Content}:
				}
			}
			if chunk.Done {
				ch <- llm.StreamChunk{Done: true}
				return
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- llm.StreamChunk{Err: llm.ClassifyError(err, 0), Done: true}
			return
		}
		ch <- llm.StreamChunk{Done: true}
	}()
	return ch, nil
}
