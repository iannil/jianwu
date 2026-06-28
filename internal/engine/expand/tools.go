package expand

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/iannil/jianwu/internal/corpus"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/reader"
	"github.com/iannil/jianwu/internal/provider/search"
	"github.com/iannil/jianwu/internal/storage"
	"github.com/iannil/jianwu/internal/workspace"
)

// ToolRegistry bundles the tools available to the expand iterations.
// Per Q13.A2: each tool has a per-chapter call limit.
type ToolRegistry struct {
	Searcher search.Searcher
	Reader   reader.Reader
	Embedder llm.Embedder

	// CorpusIndexPath is the path to the pre-computed corpus index file.
	// When set, LookupSimilarBook loads the index from here (lazily cached).
	// If empty or the file doesn't exist, LookupSimilarBook returns nil.
	CorpusIndexPath string

	mu sync.Mutex
	// Per-chapter call counters
	webSearchCalls     int
	readURLCalls       int
	lookupSimilarCalls int

	// Citation metadata registry per Q14.A3
	citations map[string]Citation // keyed by URL

	// Provider names for citation metadata (set from config; empty = use defaults)
	SearchProviderName string
	ReaderProviderName string

	// Cached corpus index (loaded lazily on first similar lookup)
	cachedIndex *corpus.CorpusIndex
	indexLoadMu sync.Mutex
}

// NewToolRegistry constructs a registry. Each Generate call gets a fresh one.
func NewToolRegistry(
	s search.Searcher,
	r reader.Reader,
	e llm.Embedder,
) *ToolRegistry {
	return &ToolRegistry{
		Searcher:  s,
		Reader:    r,
		Embedder:  e,
		citations: map[string]Citation{},
	}
}

// SetCorpusIndexPath sets the path to the corpus index file and derives the
// workspace root from it. The index is loaded lazily on first LookupSimilarBook call.
func (t *ToolRegistry) SetCorpusIndexPath(path string) {
	t.CorpusIndexPath = path
}

// loadCorpusIndex loads the corpus index from CorpusIndexPath if not yet loaded.
// Returns nil (no error) if the path is empty or file doesn't exist.
func (t *ToolRegistry) loadCorpusIndex() (*corpus.CorpusIndex, error) {
	if t.CorpusIndexPath == "" {
		return nil, nil
	}

	t.indexLoadMu.Lock()
	defer t.indexLoadMu.Unlock()

	if t.cachedIndex != nil {
		return t.cachedIndex, nil
	}

	// Check if file exists
	if _, err := storage.OS.Stat(t.CorpusIndexPath); err != nil {
		return nil, nil // not an error — just no index available
	}

	idx, err := corpus.LoadIndex(t.CorpusIndexPath)
	if err != nil {
		return nil, fmt.Errorf("load corpus index: %w", err)
	}
	t.cachedIndex = idx
	return idx, nil
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
	prov := t.ReaderProviderName
	if prov == "" {
		prov = "jina" // backward compat default
	}
	t.citations[url] = Citation{
		URL:            url,
		Title:          content.Title,
		AccessedAt:     time.Now().UTC(),
		Snippet:        truncate(content.Markdown, 200),
		ReaderProvider: prov,
	}
	t.mu.Unlock()
	return content, nil
}

// LookupSimilarBook returns the top N most similar corpus books to the given slug,
// using the pre-computed embedding index. Returns nil if no index is available
// or the slug is not found. Call limit 3 per chapter (per Q13.A2).
func (t *ToolRegistry) LookupSimilarBook(ctx context.Context, slug string, topN int) ([]string, error) {
	t.mu.Lock()
	if t.lookupSimilarCalls >= 3 {
		t.mu.Unlock()
		return nil, fmt.Errorf("lookup_similar call limit (3) reached")
	}
	t.lookupSimilarCalls++
	t.mu.Unlock()

	_ = ctx // context reserved for future use (e.g. cancel)

	idx, err := t.loadCorpusIndex()
	if err != nil {
		return nil, err
	}
	if idx == nil {
		return nil, nil // no index available
	}

	result := idx.FindSimilar(slug, topN)
	return result, nil
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
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

// CorpusIndexPathForWorkspace returns the expected corpus index file path
// for a given workspace root.
func CorpusIndexPathForWorkspace(wsRoot string) string {
	return filepath.Join(wsRoot, workspace.MarkerName, "corpus_index.json")
}
