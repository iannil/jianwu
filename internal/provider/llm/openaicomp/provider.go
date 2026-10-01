// Package openaicomp implements the llm interfaces (Chatter, Embedder,
// Streamer) over any OpenAI-compatible chat/embeddings HTTP API. Provider
// packages such as glm, kimi and deepseek are thin wrappers that supply the
// base URL, error prefix and schema mode; anything that mirrors OpenAI's
// request/response shape can reuse this layer.
package openaicomp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/iannil/jianwu/internal/provider/llm"
)

// SchemaMode selects how ChatRequest.JSONSchema is expressed through the
// response_format field. Providers differ in what they accept.
type SchemaMode string

const (
	// SchemaStrict sends response_format {"type":"json_schema",...} so the
	// API enforces the schema. Supported by GLM and Kimi K-series.
	SchemaStrict SchemaMode = "json_schema"
	// SchemaJSONObject sends response_format {"type":"json_object"}: the API
	// only guarantees valid JSON; the schema itself stays prompt-driven.
	// DeepSeek accepts this mode but rejects json_schema.
	SchemaJSONObject SchemaMode = "json_object"
)

// Config configures a generic OpenAI-compatible provider.
type Config struct {
	Name    string // provider name used in error messages, e.g. "glm"
	APIKey  string
	BaseURL string // required; wrapper packages inject their default
	// SchemaMode selects the response_format flavor for JSONSchema requests.
	// Empty defaults to SchemaStrict.
	SchemaMode SchemaMode
	// NoEmbeddings marks APIs without an /embeddings endpoint (e.g. DeepSeek);
	// Embed then fails fast with an explanatory error instead of a 404.
	NoEmbeddings bool
}

// Provider implements llm.Chatter, llm.Embedder and llm.Streamer over an
// OpenAI-compatible API.
type Provider struct {
	cfg Config
	c   *client
}

// New constructs a generic OpenAI-compatible Provider.
func New(cfg Config) (*Provider, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("openaicomp: Name is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%s: APIKey is required", cfg.Name)
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("%s: BaseURL is required", cfg.Name)
	}
	if cfg.SchemaMode == "" {
		cfg.SchemaMode = SchemaStrict
	}
	return &Provider{cfg: cfg, c: newClient(cfg.BaseURL, cfg.APIKey)}, nil
}

// Chat calls the provider's /chat/completions endpoint.
func (p *Provider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	body := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if len(req.JSONSchema) > 0 {
		switch p.cfg.SchemaMode {
		case SchemaJSONObject:
			body["response_format"] = map[string]any{"type": "json_object"}
		default:
			body["response_format"] = map[string]any{
				"type": "json_schema",
				"json_schema": map[string]any{
					"name":   "response",
					"schema": json.RawMessage(req.JSONSchema),
				},
			}
		}
	}

	resp, err := p.c.post(ctx, "/chat/completions", body)
	if err != nil {
		return nil, llm.ClassifyError(err, 0)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, llm.ClassifyError(fmt.Errorf("%s: %s", p.cfg.Name, string(b)), resp.StatusCode)
	}

	var out chatCompletionResponse
	if err := decodeJSON(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", p.cfg.Name, err)
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("%s: empty choices in response", p.cfg.Name)
	}
	cr := &llm.ChatResponse{
		Content:      out.Choices[0].Message.Content,
		FinishReason: out.Choices[0].FinishReason,
		TokensIn:     out.Usage.PromptTokens,
		TokensOut:    out.Usage.CompletionTokens,
	}
	cr.PopulateUsage()
	return cr, nil
}

// Embed calls the provider's /embeddings endpoint.
func (p *Provider) Embed(ctx context.Context, req llm.EmbedRequest) (*llm.EmbedResponse, error) {
	if p.cfg.NoEmbeddings {
		return nil, fmt.Errorf("%s: this API offers no embeddings endpoint; use glm/gemini/ollama for embedder stages", p.cfg.Name)
	}
	body := map[string]any{
		"model": req.Model,
		"input": req.Inputs,
	}
	resp, err := p.c.post(ctx, "/embeddings", body)
	if err != nil {
		return nil, llm.ClassifyError(err, 0)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, llm.ClassifyError(fmt.Errorf("%s: %s", p.cfg.Name, string(b)), resp.StatusCode)
	}
	var out embeddingsResponse
	if err := decodeJSON(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("%s: decode embeddings: %w", p.cfg.Name, err)
	}
	return &llm.EmbedResponse{
		Embeddings: out.embeddings(),
		TokensIn:   out.Usage.PromptTokens,
	}, nil
}

// OpenAI-compatible response shapes.

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

type embeddingsResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
	} `json:"usage"`
}

func (r *embeddingsResponse) embeddings() [][]float32 {
	out := make([][]float32, len(r.Data))
	for i, d := range r.Data {
		out[i] = d.Embedding
	}
	return out
}
