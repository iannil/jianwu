package jina

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/zhurong/jianwu/internal/provider/reader"
)

const DefaultBaseURL = "https://r.jina.ai"

const (
	maxBodyBytes    = 10 << 20 // 10 MB cap on Jina response bodies
	maxErrBodyBytes = 4 << 10  // 4 KB cap on error bodies
)

type Config struct {
	APIKey  string // optional for free tier
	BaseURL string
}

type Reader struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func New(cfg Config) (*Reader, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	return &Reader{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (r *Reader) Read(ctx context.Context, targetURL string) (reader.Content, error) {
	// Validate target URL before issuing request
	if _, err := url.Parse(targetURL); err != nil {
		return reader.Content{}, fmt.Errorf("jina: invalid target URL: %w", err)
	}

	fullURL := r.baseURL + "/" + targetURL
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return reader.Content{}, fmt.Errorf("jina: build request: %w", err)
	}
	if r.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.apiKey)
	}
	req.Header.Set("Accept", "text/plain")
	resp, err := r.http.Do(req)
	if err != nil {
		return reader.Content{}, fmt.Errorf("jina: fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrBodyBytes))
		return reader.Content{}, fmt.Errorf("%w: HTTP %d for %s: %s", reader.ErrReader, resp.StatusCode, targetURL, string(b))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return reader.Content{}, fmt.Errorf("jina: read body: %w", err)
	}
	// Parse "Title: ..." prefix from Jina's response if present.
	markdown := string(body)
	title := ""
	if len(markdown) > 7 && markdown[:7] == "Title: " {
		// Find first newline
		for i := 7; i < len(markdown); i++ {
			if markdown[i] == '\n' {
				title = markdown[7:i]
				break
			}
		}
	}
	return reader.Content{URL: targetURL, Title: title, Markdown: markdown}, nil
}
