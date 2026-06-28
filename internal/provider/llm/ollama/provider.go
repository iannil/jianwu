package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/iannil/jianwu/internal/provider/llm"
)

// DefaultBaseURL is the default Ollama server endpoint.
const DefaultBaseURL = "http://localhost:11434"

// Config configures an Ollama provider.
type Config struct {
	BaseURL string // defaults to DefaultBaseURL if empty; no API key needed for local
}

// Provider implements llm.Chatter, llm.Embedder, and llm.Streamer via Ollama's API.
type Provider struct {
	baseURL      string
	http         *http.Client
	streamClient *http.Client
}

// New constructs an Ollama Provider.
func New(cfg Config) (*Provider, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	return &Provider{
		baseURL:      cfg.BaseURL,
		http:         &http.Client{Timeout: 60 * time.Second},
		streamClient: &http.Client{}, // no timeout — ctx controls streaming cancellation
	}, nil
}

// Chat calls Ollama's /api/chat endpoint with stream=false.
func (p *Provider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	body := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   false,
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}

	resp, err := p.do(ctx, p.http, "/api/chat", body)
	if err != nil {
		return nil, llm.ClassifyError(err, 0)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, llm.ClassifyError(fmt.Errorf("ollama: %s", string(b)), resp.StatusCode)
	}

	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ollama: decode response: %w", err)
	}
	if out.Message.Content == "" && out.Done {
		return nil, fmt.Errorf("ollama: empty response")
	}
	cr := &llm.ChatResponse{
		Content:   out.Message.Content,
		TokensIn:  out.PromptEvalCount,
		TokensOut: out.EvalCount,
	}
	cr.PopulateUsage()
	return cr, nil
}

// Embed calls Ollama's /api/embed endpoint.
func (p *Provider) Embed(ctx context.Context, req llm.EmbedRequest) (*llm.EmbedResponse, error) {
	body := map[string]any{
		"model": req.Model,
		"input": req.Inputs,
	}
	resp, err := p.do(ctx, p.http, "/api/embed", body)
	if err != nil {
		return nil, llm.ClassifyError(err, 0)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, llm.ClassifyError(fmt.Errorf("ollama: %s", string(b)), resp.StatusCode)
	}

	var out embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ollama: decode embed: %w", err)
	}
	return &llm.EmbedResponse{
		Embeddings: out.Embeddings,
		TokensIn:   out.PromptEvalCount,
	}, nil
}

// do sends a POST request with JSON body. Caller must close the response body.
func (p *Provider) do(ctx context.Context, cl *http.Client, path string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := cl.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	return resp, nil
}

// Ollama API response shapes.

type chatResponse struct {
	Model           string      `json:"model"`
	Message         message     `json:"message"`
	Done            bool        `json:"done"`
	PromptEvalCount int         `json:"prompt_eval_count"`
	EvalCount       int         `json:"eval_count"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type embedResponse struct {
	Model           string      `json:"model"`
	Embeddings      [][]float32 `json:"embeddings"`
	PromptEvalCount int         `json:"prompt_eval_count"`
}
