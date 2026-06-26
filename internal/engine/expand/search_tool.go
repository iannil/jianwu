package expand

import (
	"context"
	"time"

	"github.com/iannil/jianwu/internal/provider/search"
)

// SearchAndRegister calls WebSearch and registers each result URL as a
// citation candidate (with search_provider attribution). The reader later
// enriches these with title/accessed_at/snippet when read_url is called.
func (t *ToolRegistry) SearchAndRegister(ctx context.Context, query string) ([]search.SearchResult, error) {
	results, err := t.WebSearch(ctx, query)
	if err != nil {
		return nil, err
	}
	prov := t.SearchProviderName
	if prov == "" {
		prov = "brave" // backward compat default
	}
	t.mu.Lock()
	for _, r := range results {
		// Only register if not already known.
		if _, exists := t.citations[r.URL]; !exists {
			t.citations[r.URL] = Citation{
				URL:            r.URL,
				Title:          r.Title,
				Snippet:        r.Snippet,
				SearchProvider: prov,
				AccessedAt:     time.Now().UTC(),
			}
		}
	}
	t.mu.Unlock()
	return results, nil
}
