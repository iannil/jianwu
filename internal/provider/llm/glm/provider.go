package glm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/zhurong/jianwu/internal/provider/llm"
)

// DefaultBaseURL is the GLM (智谱 BigModel) endpoint.
const DefaultBaseURL = "https://open.bigmodel.cn/api/paas/v4"

// Config configures a GLM provider.
type Config struct {
	APIKey  string
	BaseURL string // defaults to DefaultBaseURL if empty
}

// Provider implements llm.Chatter and llm.Embedder via GLM's OpenAI-compatible API.
type Provider struct {
	c *client
}

// New constructs a GLM Provider.
func New(cfg Config) (*Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("glm: APIKey is required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	return &Provider{c: newClient(cfg.BaseURL, cfg.APIKey)}, nil
}

// Chat calls GLM's /chat/completions endpoint.
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
		body["response_format"] = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "response",
				"schema": json.RawMessage(req.JSONSchema),
			},
		}
	}

	resp, err := p.c.post(ctx, "/chat/completions", body)
	if err != nil {
		return nil, llm.ClassifyError(err, 0)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, llm.ClassifyError(fmt.Errorf("glm: %s", string(b)), resp.StatusCode)
	}

	var out chatCompletionResponse
	if err := decodeJSON(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("glm: decode response: %w", err)
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("glm: empty choices in response")
	}
	return &llm.ChatResponse{
		Content:      out.Choices[0].Message.Content,
		FinishReason: out.Choices[0].FinishReason,
		TokensIn:     out.Usage.PromptTokens,
		TokensOut:    out.Usage.CompletionTokens,
	}, nil
}

// Embed calls GLM's /embeddings endpoint.
func (p *Provider) Embed(ctx context.Context, req llm.EmbedRequest) (*llm.EmbedResponse, error) {
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
		return nil, llm.ClassifyError(fmt.Errorf("glm: %s", string(b)), resp.StatusCode)
	}
	var out embeddingsResponse
	if err := decodeJSON(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("glm: decode embeddings: %w", err)
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
