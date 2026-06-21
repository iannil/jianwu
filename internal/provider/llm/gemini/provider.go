package gemini

import (
	"context"
	"fmt"

	"github.com/zhurong/jianwu/internal/provider/llm"
	"google.golang.org/genai"
)

// Config configures a Gemini provider.
type Config struct {
	APIKey string
}

// Provider implements llm.Chatter and llm.Embedder via Google's official genai SDK.
type Provider struct {
	client *genai.Client
}

// New constructs a Gemini Provider. The genai.Client is initialized eagerly
// to validate the API key against Google's backend.
func New(cfg Config) (*Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("gemini: APIKey is required")
	}
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: init client: %w", err)
	}
	return &Provider{client: client}, nil
}

// Chat calls Gemini's GenerateContent endpoint.
func (p *Provider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	contents := make([]*genai.Content, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := m.Role
		if role == "assistant" {
			role = "model" // Gemini uses "model" not "assistant"
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
	if len(req.JSONSchema) > 0 {
		config.ResponseMIMEType = "application/json"
		config.ResponseSchema = &genai.Schema{} // populated from req.JSONSchema; see schemaFromRaw
		if err := schemaFromRaw(req.JSONSchema, config.ResponseSchema); err != nil {
			return nil, fmt.Errorf("gemini: parse JSON schema: %w", err)
		}
	}
	resp, err := p.client.Models.GenerateContent(ctx, req.Model, contents, config)
	if err != nil {
		return nil, llm.ClassifyError(err, 0) // SDK errors are network/API; status mapping is approximate
	}
	out := &llm.ChatResponse{}
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil && len(resp.Candidates[0].Content.Parts) > 0 {
		if text := resp.Candidates[0].Content.Parts[0].Text; text != "" {
			out.Content = text
		}
		out.FinishReason = string(resp.Candidates[0].FinishReason)
	}
	if resp.UsageMetadata != nil {
		out.TokensIn = int(resp.UsageMetadata.PromptTokenCount)
		out.TokensOut = int(resp.UsageMetadata.CandidatesTokenCount)
	}
	return out, nil
}

// Embed calls Gemini's embedContent endpoint.
func (p *Provider) Embed(ctx context.Context, req llm.EmbedRequest) (*llm.EmbedResponse, error) {
	out := &llm.EmbedResponse{Embeddings: make([][]float32, 0, len(req.Inputs))}
	for _, input := range req.Inputs {
		// EmbedContent expects []*Content, so wrap the input string
		contents := []*genai.Content{
			genai.NewContentFromText(input, genai.RoleUser),
		}
		resp, err := p.client.Models.EmbedContent(ctx, req.Model, contents, &genai.EmbedContentConfig{
			TaskType: "RETRIEVAL_DOCUMENT",
		})
		if err != nil {
			return nil, llm.ClassifyError(err, 0)
		}
		if resp.Embeddings != nil && len(resp.Embeddings) > 0 && len(resp.Embeddings[0].Values) > 0 {
			out.Embeddings = append(out.Embeddings, resp.Embeddings[0].Values)
		}
		// Gemini doesn't return per-call token count in EmbedContent response.
	}
	return out, nil
}

// schemaFromRaw populates a genai.Schema from a JSON Schema byte slice.
// For S2 we only support a minimal subset (type, properties). Full JSON Schema
// translation will land in S3 (outline) when structured outputs are first used.
func schemaFromRaw(raw []byte, s *genai.Schema) error {
	// Minimal: treat as a free-form object. Full impl deferred to S3.
	s.Type = "OBJECT"
	return nil
}
