package cli

import (
	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/engine/expand"
	"strings"
)

// toChapterCitations converts expand.Citation to book.ChapterCitation (frontmatter).
func toChapterCitations(cs []expand.Citation) []book.ChapterCitation {
	out := make([]book.ChapterCitation, 0, len(cs))
	for _, c := range cs {
		out = append(out, book.ChapterCitation{
			ID:    c.ID,
			URL:   c.URL,
			Title: c.Title,
			Site:  extractSite(c.URL),
		})
	}
	return out
}

// toBookCitations converts expand.Citation to book.Citation (outline.json).
func toBookCitations(cs []expand.Citation) []book.Citation {
	out := make([]book.Citation, 0, len(cs))
	for _, c := range cs {
		out = append(out, book.Citation{
			ID:             c.ID,
			URL:            c.URL,
			Title:          c.Title,
			AccessedAt:     c.AccessedAt,
			Snippet:        c.Snippet,
			SearchProvider: c.SearchProvider,
			ReaderProvider: c.ReaderProvider,
		})
	}
	return out
}

// extractSite returns the host portion of a URL for the frontmatter "site" field.
func extractSite(rawURL string) string {
	// Best-effort: strip scheme + path.
	s := rawURL
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return s
}
