package corpus

// Book is a reference book outline stored in the builtin corpus.
// Schema mirrors internal/corpus/builtin/*.json.
type Book struct {
	Slug      string         `json:"slug"`
	Title     LocalizedTitle `json:"title"`
	Subtitle  string         `json:"subtitle,omitempty"`
	Archetype string         `json:"archetype"`
	Audience  string         `json:"audience"`
	Depth     string         `json:"depth"`
	Goal      string         `json:"goal"`
	Length    string         `json:"length"`
	Language  []string       `json:"language"`
	Source    Source         `json:"source"`
	Abstract  string         `json:"abstract"`
	Parts     []Part         `json:"parts"`
}

type LocalizedTitle struct {
	Zh string `json:"zh"`
	En string `json:"en"`
}

type Source struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	AccessedAt string `json:"accessed_at"`
}

type Part struct {
	Index    int            `json:"index"`
	Title    LocalizedTitle `json:"title"`
	Role     string         `json:"role"`
	Intro    string         `json:"intro,omitempty"`
	Chapters []Chapter      `json:"chapters"`
}

type Chapter struct {
	Index    int            `json:"index"`
	Title    LocalizedTitle `json:"title"`
	Abstract string         `json:"abstract,omitempty"`
}
