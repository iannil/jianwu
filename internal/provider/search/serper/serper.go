package serper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zhurong/jianwu/internal/provider/search"
)

const DefaultBaseURL = "https://google.serper.dev/search"

type Config struct {
	APIKey  string
	BaseURL string
}

type Searcher struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func New(cfg Config) (*Searcher, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("serper: APIKey required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	return &Searcher{apiKey: cfg.APIKey, baseURL: cfg.BaseURL, http: &http.Client{Timeout: 15 * time.Second}}, nil
}

func (s *Searcher) Search(ctx context.Context, query string, opts search.SearchOpts) ([]search.SearchResult, error) {
	if opts.MaxResults == 0 {
		opts.MaxResults = 10
	}
	if opts.MaxResults > 50 {
		opts.MaxResults = 50
	}
	body, _ := json.Marshal(map[string]any{
		"q":   query,
		"num": opts.MaxResults,
	})
	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("serper: build request: %w", err)
	}
	req.Header.Set("X-API-KEY", s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", search.ErrSearchNetwork, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: %s", search.ErrSearchRateLimit, string(b))
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: HTTP %d: %s", search.ErrSearchProvider, resp.StatusCode, string(b))
	}
	var respBody struct {
		Organic []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("serper: decode: %w", err)
	}
	out := make([]search.SearchResult, len(respBody.Organic))
	for i, r := range respBody.Organic {
		out[i] = search.SearchResult{Title: r.Title, URL: r.Link, Snippet: r.Snippet}
	}
	return out, nil
}
