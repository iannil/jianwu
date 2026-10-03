package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iannil/jianwu/internal/book"
)

// ChapterDoc is one chapter's assembled export content.
type ChapterDoc struct {
	PartIndex    int
	ChapterIndex int
	PartTitle    string
	PartIntro    string
	Title        string
	BodyMD       string
	Meta         book.OutlineChapter
	Missing      bool
}

// StripLeadingTitle removes a leading markdown heading that duplicates the
// chapter title. Generators emit "## <title>" as the body's first line while
// every output template (EPUB chapter page, reading-site page, md/hugo
// exports) renders the title itself — keeping both produced stacked
// duplicate titles.
func StripLeadingTitle(body, title string) string {
	if title == "" {
		return body
	}
	first, rest, hasMore := strings.Cut(body, "\n")
	if !hasMore {
		first, rest = body, ""
	}
	t := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(first), "#"))
	t = strings.TrimSpace(strings.TrimRight(t, "#")) // closed ATX form "## T ##"
	if t != strings.TrimSpace(title) {
		return body
	}
	// Swallow the blank line(s) that followed the removed heading.
	return strings.TrimLeft(rest, "\n")
}

// Collect walks the outline in order and reads every chapter file from
// <bookDir>/chapters/. See CollectDir for the generalized form.
func Collect(bookDir string, outline *book.Outline) ([]ChapterDoc, error) {
	return CollectDir(filepath.Join(bookDir, "chapters"), outline)
}

// CollectDir walks the outline in order and reads chapter files from
// <dir>/NN-MM.md. Missing chapters are marked Missing (placeholder semantics
// mirror the md export). Footnote ids are renumbered per chapter (defensive
// against LLM gaps) and access dates are normalized from structured citation
// data — invented dates in the prose are never trusted (see
// book.NormalizeFootnoteDates). The static-site layer runs this over a
// release's content/ snapshot.
func CollectDir(dir string, outline *book.Outline) ([]ChapterDoc, error) {
	var docs []ChapterDoc
	for pi := range outline.Parts {
		p := &outline.Parts[pi]
		for ci := range p.Chapters {
			c := &p.Chapters[ci]
			doc := ChapterDoc{
				PartIndex: p.Index, ChapterIndex: c.Index,
				PartTitle: p.Title, PartIntro: p.Intro,
				Title: c.Title, Meta: *c,
			}
			path := filepath.Join(dir, fmt.Sprintf("%02d-%02d.md", p.Index, c.Index))
			_, body, err := book.ReadChapter(path)
			if err != nil {
				if !os.IsNotExist(err) {
					return nil, fmt.Errorf("read chapter %02d-%02d: %w", p.Index, c.Index, err)
				}
				doc.Missing = true
			} else {
				body = StripLeadingTitle(body, c.Title)
				renumbered, _ := book.RenumberFootnotes(body, 1)
				accessed := map[string]time.Time{}
				for _, cit := range c.Citations {
					if !cit.AccessedAt.IsZero() {
						accessed[cit.URL] = cit.AccessedAt
					}
				}
				doc.BodyMD = book.NormalizeFootnoteDates(renumbered, accessed)
			}
			docs = append(docs, doc)
		}
	}
	return docs, nil
}

// FindCover returns the book's cover image bytes by convention
// (books/<slug>/cover.png or cover.jpg), or nil when absent.
func FindCover(bookDir string) ([]byte, string) {
	for name, media := range map[string]string{
		"cover.png": "image/png",
		"cover.jpg": "image/jpeg",
	} {
		data, err := os.ReadFile(filepath.Join(bookDir, name))
		if err == nil {
			return data, media
		}
	}
	return nil, ""
}
