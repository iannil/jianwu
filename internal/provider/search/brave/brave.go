package brave

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/zhurong/jianwu/internal/provider/search"
)

const DefaultBaseURL = "https://api.search.brave.com/res/v1/web/search"

type Config struct {
	APIKey  string
	BaseURL string // defaults to DefaultBaseURL
}

type Searcher struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func New(cfg Config) (*Searcher, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("brave: APIKey required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	return &Searcher{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		http:    &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (s *Searcher) Search(ctx context.Context, query string, opts search.SearchOpts) ([]search.SearchResult, error) {
	if opts.MaxResults == 0 {
		opts.MaxResults = 10
	}
	params := url.Values{}
	params.Set("q", query)
	params.Set("count", strconv.Itoa(opts.MaxResults))
	if opts.Language != "" {
		params.Set("search_lang", opts.Language)
	}
	switch opts.TimeRange {
	case search.TimeDay:
		params.Set("freshness", "pd")
	case search.TimeWeek:
		params.Set("freshness", "pw")
	case search.TimeMonth:
		params.Set("freshness", "pm")
	case search.TimeYear:
		params.Set("freshness", "py")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("brave: build request: %w", err)
	}
	req.Header.Set("X-Subscription-Token", s.apiKey)
	req.Header.Set("Accept", "application/json")

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

	var body struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
				Age         string `json:"age"` // Brave returns ISO 8601 duration or timestamp
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("brave: decode: %w", err)
	}
	out := make([]search.SearchResult, len(body.Web.Results))
	for i, r := range body.Web.Results {
		out[i] = search.SearchResult{Title: r.Title, URL: r.URL, Snippet: r.Description}
	}
	return out, nil
}
