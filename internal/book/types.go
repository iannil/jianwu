package book

import "time"

// Meta is the top-level book metadata, serialized to meta.json.
// Schema mirrors DESIGN.md §4.2.
type Meta struct {
	ID         string         `json:"id"`
	Slug       string         `json:"slug"`
	Title      string         `json:"title"`
	Subtitle   string         `json:"subtitle,omitempty"`
	Archetype  string         `json:"archetype"`
	Parameters Parameters     `json:"parameters"`
	Language   string         `json:"language"`
	Status     string         `json:"status"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Engine     EngineMeta     `json:"engine"`
}

type Parameters struct {
	Audience string `json:"audience"`
	Depth    string `json:"depth"`
	Goal     string `json:"goal"`
	Length   string `json:"length"`
}

type EngineMeta struct {
	JianwuVersion          string `json:"jianwu_version"`
	ArchetypeLibraryVersion string `json:"archetype_library_version"`
	GrillTreeVersion       string `json:"grill_tree_version"`
	StyleGuideVersion      string `json:"style_guide_version"`
	SamplesVersion         string `json:"samples_version"`
}

// Outline is the book outline, serialized to outline.json.
type Outline struct {
	Parts []OutlinePart `json:"parts"`
}

type OutlinePart struct {
	Index    int              `json:"index"`
	Title    string           `json:"title"`
	Role     string           `json:"role"`
	Intro    string           `json:"intro,omitempty"`
	Chapters []OutlineChapter `json:"chapters"`
}

type OutlineChapter struct {
	Index             int        `json:"index"`
	Title             string     `json:"title"`
	Abstract          string     `json:"abstract,omitempty"`
	KeyConcepts       []string   `json:"key_concepts,omitempty"`
	LearningObjectives []string  `json:"learning_objectives,omitempty"`
	SuggestedExamples []string   `json:"suggested_examples,omitempty"`
	Claims            []Claim    `json:"claims,omitempty"`
	Status            string     `json:"status"`
	WordCountTarget   int        `json:"word_count_target,omitempty"`
	WordCount         int        `json:"word_count,omitempty"`
	CitationsCount    int        `json:"citations_count,omitempty"`
	UnverifiedClaims  int        `json:"unverified_claims,omitempty"`
	CoherenceScore    *float64   `json:"coherence_score,omitempty"`
	ExpandedWith      *ExpandedWith `json:"expanded_with,omitempty"`
	ReviewedAt        *time.Time `json:"reviewed_at,omitempty"`
	ReviewedBy        string     `json:"reviewed_by,omitempty"`
	Citations         []Citation `json:"citations,omitempty"`
}

type Claim struct {
	Text       string `json:"text"`
	HasCitation bool  `json:"has_citation"`
}

type ExpandedWith struct {
	Provider  string   `json:"provider"`
	Model     string   `json:"model"`
	ToolsUsed []string `json:"tools_used,omitempty"`
	Iterations int     `json:"iterations,omitempty"`
	Tokens    Tokens   `json:"tokens"`
}

type Tokens struct {
	In  int `json:"in"`
	Out int `json:"out"`
}

type Citation struct {
	ID              string `json:"id"`
	URL             string `json:"url"`
	Title           string `json:"title,omitempty"`
	AccessedAt      time.Time `json:"accessed_at,omitempty"`
	Snippet         string `json:"snippet,omitempty"`
	UsedInParagraph string `json:"used_in_paragraph,omitempty"`
	SearchProvider  string `json:"search_provider,omitempty"`
	ReaderProvider  string `json:"reader_provider,omitempty"`
}

// Chapter status constants.
const (
	StatusScaffolded = "scaffolded"
	StatusExpanded   = "expanded"
	StatusReviewed   = "reviewed"
	StatusFinal      = "final"
	StatusFailed     = "failed"
)

// Book status constants (mirrors Meta.Status).
const (
	BookStatusDraft = "draft"
)
