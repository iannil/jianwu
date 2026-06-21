package archetypes

// Archetype represents a structural prototype for a book.
// Schema mirrors internal/archetypes/*.yaml.
type Archetype struct {
	SchemaVersion int           `yaml:"schema_version"`
	ID            string        `yaml:"id"`
	Name          LocalizedName `yaml:"name"`
	Description   string        `yaml:"description"`
	WhenToUse     WhenToUse     `yaml:"when_to_use"`
	Parts         []Part        `yaml:"parts"`
	Examples      []Example     `yaml:"examples"`
	Metadata      Metadata      `yaml:"metadata"`
}

type LocalizedName struct {
	Zh string `yaml:"zh"`
	En string `yaml:"en"`
}

type WhenToUse struct {
	Goals             []string `yaml:"goals"`
	TopicTypes        []string `yaml:"topic_types"`
	AudienceFit       []string `yaml:"audience_fit"`
	NotRecommendedFor []string `yaml:"not_recommended_for"`
}

type Part struct {
	Role             string            `yaml:"role"`
	TitleTemplate    LocalizedTemplate `yaml:"title_template"`
	Guidance         string            `yaml:"guidance"`
	TypicalChapters  []int             `yaml:"typical_chapters"`
	ChapterRoleHints []string          `yaml:"chapter_role_hints"`
	Conditional      *bool             `yaml:"conditional,omitempty"`
	SkipWhen         string            `yaml:"skip_when,omitempty"`
}

type LocalizedTemplate struct {
	Zh string `yaml:"zh"`
	En string `yaml:"en"`
}

type Example struct {
	Slug      string  `yaml:"slug"`
	Source    string  `yaml:"source"`
	SourceURL string  `yaml:"source_url"`
	FitScore  float64 `yaml:"fit_score"`
	Note      string  `yaml:"note,omitempty"`
}

type Metadata struct {
	ExtractedFrom string `yaml:"extracted_from"`
	ExtractedAt   string `yaml:"extracted_at"`
	Author        string `yaml:"author"`
	Notes         string `yaml:"notes,omitempty"`
}
