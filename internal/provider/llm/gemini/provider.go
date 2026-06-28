package gemini

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iannil/jianwu/internal/provider/llm"
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
	out.PopulateUsage()
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
		if len(resp.Embeddings) > 0 && len(resp.Embeddings[0].Values) > 0 {
			out.Embeddings = append(out.Embeddings, resp.Embeddings[0].Values)
		}
		// Gemini doesn't return per-call token count in EmbedContent response.
	}
	return out, nil
}

// schemaFromRaw populates a genai.Schema from a JSON Schema byte slice.
func schemaFromRaw(raw []byte, s *genai.Schema) error {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("parse JSON Schema: %w", err)
	}
	return populateSchema(m, s)
}

// populateSchema recursively translates JSON Schema to genai.Schema.
func populateSchema(m map[string]any, s *genai.Schema) error {
	if t, ok := m["type"].(string); ok {
		switch t {
		case "object":
			s.Type = "OBJECT"
		case "string":
			s.Type = "STRING"
		case "number":
			s.Type = "NUMBER"
		case "integer":
			s.Type = "INTEGER"
		case "boolean":
			s.Type = "BOOLEAN"
		case "array":
			s.Type = "ARRAY"
		}
	}
	if desc, ok := m["description"].(string); ok {
		s.Description = desc
	}
	if req, ok := m["required"].([]any); ok {
		for _, r := range req {
			if str, ok := r.(string); ok {
				s.Required = append(s.Required, str)
			}
		}
	}
	if props, ok := m["properties"].(map[string]any); ok {
		s.Properties = make(map[string]*genai.Schema, len(props))
		for k, v := range props {
			if vm, ok := v.(map[string]any); ok {
				child := &genai.Schema{}
				if err := populateSchema(vm, child); err != nil {
					return err
				}
				s.Properties[k] = child
			}
		}
	}
	if items, ok := m["items"].(map[string]any); ok {
		child := &genai.Schema{}
		if err := populateSchema(items, child); err != nil {
			return err
		}
		s.Items = child
	}
	return nil
}
