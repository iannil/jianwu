package expand

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zhurong/jianwu/internal/provider/llm"
	"github.com/zhurong/jianwu/internal/provider/reader"
	"github.com/zhurong/jianwu/internal/provider/search"
)

// ToolRegistry bundles the tools available to the expand iterations.
// Per Q13.A2: each tool has a per-chapter call limit.
type ToolRegistry struct {
	Searcher search.Searcher
	Reader   reader.Reader
	Embedder llm.Embedder
	Outline  func(partIdx, chIdx int) (string, error) // returns adjacent chapter prose

	mu sync.Mutex
	// Per-chapter call counters
	webSearchCalls int
	readURLCalls   int

	// Citation metadata registry per Q14.A3
	citations map[string]Citation // keyed by URL
}

// NewToolRegistry constructs a registry. Each Generate call gets a fresh one.
func NewToolRegistry(
	s search.Searcher,
	r reader.Reader,
	e llm.Embedder,
	outlineFn func(partIdx, chIdx int) (string, error),
) *ToolRegistry {
	return &ToolRegistry{
		Searcher:  s,
		Reader:    r,
		Embedder:  e,
		Outline:   outlineFn,
		citations: map[string]Citation{},
	}
}

// WebSearch calls the search provider with hard cap 5 (per Q13.A2).
func (t *ToolRegistry) WebSearch(ctx context.Context, query string) ([]search.SearchResult, error) {
	t.mu.Lock()
	if t.webSearchCalls >= 5 {
		t.mu.Unlock()
		return nil, fmt.Errorf("web_search call limit (5) reached")
	}
	t.webSearchCalls++
	t.mu.Unlock()

	return t.Searcher.Search(ctx, query, search.SearchOpts{MaxResults: 5})
}

// ReadURL calls the reader provider with hard cap 10.
func (t *ToolRegistry) ReadURL(ctx context.Context, url string) (reader.Content, error) {
	t.mu.Lock()
	if t.readURLCalls >= 10 {
		t.mu.Unlock()
		return reader.Content{}, fmt.Errorf("read_url call limit (10) reached")
	}
	t.readURLCalls++
	t.mu.Unlock()

	content, err := t.Reader.Read(ctx, url)
	if err != nil {
		return reader.Content{}, err
	}
	// Record citation metadata per Q14.A3.
	t.mu.Lock()
	t.citations[url] = Citation{
		URL:            url,
		Title:          content.Title,
		AccessedAt:     time.Now().UTC(),
		Snippet:        truncate(content.Markdown, 200),
		ReaderProvider: "jina", // hardcode for v1; could come from registry
	}
	t.mu.Unlock()
	return content, nil
}

// ReadAdjacentChapter retrieves prose from an adjacent chapter for coherence.
func (t *ToolRegistry) ReadAdjacentChapter(partIdx, chIdx int) (string, error) {
	if t.Outline == nil {
		return "", fmt.Errorf("outline function not provided")
	}
	return t.Outline(partIdx, chIdx)
}

// Citations returns a snapshot of recorded citations.
func (t *ToolRegistry) Citations() []Citation {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Citation, 0, len(t.citations))
	for _, c := range t.citations {
		out = append(out, c)
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
