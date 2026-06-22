package expand

import (
	"time"

	"github.com/zhurong/jianwu/internal/book"
)

// ExpandInput is the input for expanding one chapter.
type ExpandInput struct {
	ArchetypeID string
	// Book context
	Topic    string
	Audience string
	Depth    string
	Goal     string
	Length   string
	Language string
	// Chapter context
	PartIndex    int
	PartTitle    string
	PartRole     string
	ChapterIndex int
	ChapterTitle string
	Abstract     string   // from scaffolding
	KeyConcepts  []string // from scaffolding
	// Adjacent chapters (for coherence)
	PreviousChapter *book.OutlineChapter
	NextChapter     *book.OutlineChapter
	// Config
	WebSearchEnabled bool // false skips research iteration
}

// ExpandOutput is the expanded chapter result.
type ExpandOutput struct {
	Markdown         string // final prose with [^N] footnotes
	Citations        []Citation
	UnverifiedClaims []Claim
	WordCount        int
	Research         ResearchNotes
	Draft            string // pre-validation draft (for debugging)
}

// Citation is one footnote reference, structured.
// Per Q14.A1 — double-write: also exists as [^N] in Markdown.
type Citation struct {
	ID              string // "1", "2", ...
	URL             string
	Title           string
	AccessedAt      time.Time
	Snippet         string
	UsedInParagraph string // paragraph identifier (e.g. "p3")
	SearchProvider  string // "brave", "serper"
	ReaderProvider  string // "jina"
}

// Claim is a factual statement the LLM self-reported.
// has_citation=false counts toward UnverifiedClaims.
type Claim struct {
	Text        string `json:"text"`
	HasCitation bool   `json:"has_citation"`
}

// ResearchPlan is what iter 1 produces: queries to search.
type ResearchPlan struct {
	Queries []string `json:"queries"`
}

// ResearchNotes is what iter 1 produces after tool calls: digested findings.
type ResearchNotes struct {
	Findings   []Finding `json:"findings"`
	Candidates []string  `json:"candidates"` // URLs to potentially cite
}

// Finding is one piece of digested research.
type Finding struct {
	Query   string `json:"query"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	Note    string `json:"note"` // LLM-extracted insight
}

// ValidationResult is iter 3's structured output.
type ValidationResult struct {
	RevisedMarkdown string  `json:"revised_markdown"`
	Claims          []Claim `json:"claims"`
}
