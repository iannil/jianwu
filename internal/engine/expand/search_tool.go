package expand

import (
	"context"
	"time"

	"github.com/zhurong/jianwu/internal/provider/search"
)

// SearchAndRegister calls WebSearch and registers each result URL as a
// citation candidate (with search_provider attribution). The reader later
// enriches these with title/accessed_at/snippet when read_url is called.
func (t *ToolRegistry) SearchAndRegister(ctx context.Context, query string) ([]search.SearchResult, error) {
	results, err := t.WebSearch(ctx, query)
	if err != nil {
		return nil, err
	}
	t.mu.Lock()
	for _, r := range results {
		// Only register if not already known.
		if _, exists := t.citations[r.URL]; !exists {
			t.citations[r.URL] = Citation{
				URL:            r.URL,
				Title:          r.Title,
				Snippet:        r.Snippet,
				SearchProvider: "brave",
				AccessedAt:     time.Now().UTC(),
			}
		}
	}
	t.mu.Unlock()
	return results, nil
}
