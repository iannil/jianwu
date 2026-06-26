package gemini

import (
	"context"

	"github.com/iannil/jianwu/internal/provider/llm"
	"google.golang.org/genai"
)

// Stream implements llm.Streamer via Gemini's GenerateContentStream.
func (p *Provider) Stream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	contents := make([]*genai.Content, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, &genai.Content{
			Role:  role,
			Parts: []*genai.Part{{Text: m.Content}},
		})
	}
	config := &genai.GenerateContentConfig{}
	if req.Temperature != nil {
		config.Temperature = &[]float32{float32(*req.Temperature)}[0]
	}
	if req.MaxTokens > 0 {
		config.MaxOutputTokens = int32(req.MaxTokens)
	}

	ch := make(chan llm.StreamChunk)
	go func() {
		defer close(ch)
		for resp, err := range p.client.Models.GenerateContentStream(ctx, req.Model, contents, config) {
			if err != nil {
				ch <- llm.StreamChunk{Err: llm.ClassifyError(err, 0), Done: true}
				return
			}
			if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil && len(resp.Candidates[0].Content.Parts) > 0 {
				if text := resp.Candidates[0].Content.Parts[0].Text; text != "" {
					select {
					case <-ctx.Done():
						ch <- llm.StreamChunk{Err: ctx.Err(), Done: true}
						return
					case ch <- llm.StreamChunk{Content: text}:
					}
				}
			}
		}
		ch <- llm.StreamChunk{Done: true}
	}()
	return ch, nil
}
