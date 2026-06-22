package outline

import (
	"fmt"

	"github.com/zhurong/jianwu/internal/book"
	"github.com/zhurong/jianwu/internal/corpus"
)

// Input carries everything the outline generator needs.
type Input struct {
	// ArchetypeID picks the structural template (e.g. "ontology-epistemology-practice").
	ArchetypeID string
	// Topic is the book's core question or subject, e.g. "时间的实在".
	Topic string
	// Parameters from grill: audience, depth, goal, length.
	Audience string // "scholar" | "educated-general" | "advanced-practitioner" | ...
	Depth    string // "intro" | "intermediate" | "advanced"
	Goal     string // "understanding" | "operational" | "decision"
	Length   string // "short" | "medium" | "long"
	// Language: "zh" | "en" | "bilingual".
	Language string
}

// Output is the generated outline. Aliased here so callers don't need book package.
type Output = book.Outline

// promptData is the template context for system.md.tmpl and user.md.tmpl.
type promptData struct {
	Archetype      string // raw YAML text of the chosen archetype
	Samples        string // few-shot paragraph samples for the chosen archetype
	CorpusOutlines string // formatted outline excerpts of reference books with same archetype
	Topic          string
	Audience       string
	Depth          string
	Goal           string
	Length         string
	Language       string
}

// referenceOutlines formats matching corpus books as a compact reference text.
// One book per section: title + parts + chapter titles + abstracts.
func referenceOutlines(books []*corpus.Book) string {
	if len(books) == 0 {
		return "(no reference books available)"
	}
	var b []string
	for _, bk := range books {
		b = append(b, formatCorpusBook(bk))
	}
	return joinSections(b)
}

// formatCorpusBook formats a single corpus book as a reference outline.
func formatCorpusBook(b *corpus.Book) string {
	var s string
	s += fmt.Sprintf("## %s\n", b.Title.Zh)
	s += fmt.Sprintf("slug: %s, archetype: %s\n\n", b.Slug, b.Archetype)
	for _, p := range b.Parts {
		s += fmt.Sprintf("### %s (role: %s)\n", p.Title.Zh, p.Role)
		if p.Intro != "" {
			s += fmt.Sprintf("%s\n\n", p.Intro)
		}
		for _, c := range p.Chapters {
			s += fmt.Sprintf("- %s: %s\n", c.Title.Zh, c.Abstract)
		}
		s += "\n"
	}
	return s
}

// joinSections joins multiple sections with a separator.
func joinSections(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "\n---\n\n"
		}
		out += p
	}
	return out
}
