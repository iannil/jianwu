package glm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// client is a thin HTTP wrapper over an OpenAI-compatible chat/embeddings API.
// Reusable for GLM, Qwen, Moonshot, DeepSeek, etc. — anything that mirrors OpenAI's shape.
type client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func newClient(baseURL, apiKey string) *client {
	return &client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// post sends a POST with Bearer auth and JSON body, returns the raw response.
// Caller is responsible for closing the body.
func (c *client) post(ctx context.Context, path string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	return resp, nil
}

func decodeJSON(r io.Reader, v any) error {
	d := json.NewDecoder(r)
	return d.Decode(v)
}
