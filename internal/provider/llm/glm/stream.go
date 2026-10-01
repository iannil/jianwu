package glm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/iannil/jianwu/internal/provider/llm"
)

// Stream implements llm.Streamer via GLM's OpenAI-compatible SSE streaming.
func (p *Provider) Stream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	body := map[string]any{
		"model":          req.Model,
		"messages":       req.Messages,
		"stream":         true,
		"stream_options": map[string]bool{"include_usage": true},
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}

	resp, err := p.c.postStream(ctx, "/chat/completions", body)
	if err != nil {
		return nil, llm.ClassifyError(err, 0)
	}

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, llm.ClassifyError(fmt.Errorf("glm: %s", string(b)), resp.StatusCode)
	}

	ch := make(chan llm.StreamChunk)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				select {
				case ch <- llm.StreamChunk{Done: true}:
				case <-ctx.Done():
					return
				}
				return
			}
			var chunk streamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue // skip malformed lines
			}
			if chunk.Usage != nil {
				select {
				case ch <- llm.StreamChunk{Usage: chunk.Usage}:
				case <-ctx.Done():
					return
				}
			}
			if len(chunk.Choices) > 0 {
				content := chunk.Choices[0].Delta.Content
				if content != "" {
					select {
					case <-ctx.Done():
						select {
						case ch <- llm.StreamChunk{Err: ctx.Err(), Done: true}:
						case <-ctx.Done():
							return
						}
						return
					case ch <- llm.StreamChunk{Content: content}:
					}
				}
				// Continue through the trailing usage-only event after finish_reason.
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case ch <- llm.StreamChunk{Err: llm.ClassifyError(err, 0), Done: true}:
			case <-ctx.Done():
				return
			}
			return
		}
		select {
		case ch <- llm.StreamChunk{Done: true}:
		case <-ctx.Done():
			return
		}
	}()
	return ch, nil
}

// streamChunk is the SSE delta format from OpenAI-compatible APIs.
type streamChunk struct {
	Usage   *llm.Usage `json:"usage"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}
