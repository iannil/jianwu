package release

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/iannil/jianwu/internal/book"
)

// Manifest is the release manifest (manifest.json), schema 1. It carries the
// content hashes that make a release verifiable and immutable.
type Manifest struct {
	Schema        int             `json:"schema"`
	BookID        string          `json:"book_id"`
	Slug          string          `json:"slug"`
	Version       string          `json:"version"`
	CreatedAt     time.Time       `json:"created_at"`
	JianwuVersion string          `json:"jianwu_version"`
	Content       ManifestContent `json:"content"`
	EPUB          *ArtifactInfo   `json:"epub,omitempty"`
	Cover         *ArtifactInfo   `json:"cover,omitempty"`
}

// ManifestContent hashes and counts the snapshot files inside a release.
type ManifestContent struct {
	OutlineSHA256    string         `json:"outline_sha256"`
	MetaSHA256       string         `json:"meta_sha256"`
	Chapters         []ChapterEntry `json:"chapters"`
	ChaptersTotal    int            `json:"chapters_total"`
	ClaimsTotal      int            `json:"claims_total"`
	ClaimsUnverified int            `json:"claims_unverified"`
	CitationsTotal   int            `json:"citations_total"`
	VerdictsFailed   int            `json:"verdicts_failed"`
}

// ChapterEntry records one chapter copy inside content/.
type ChapterEntry struct {
	Part      int    `json:"part"`
	Chapter   int    `json:"chapter"`
	Title     string `json:"title"`
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	WordCount int    `json:"word_count"`
	Status    string `json:"status"`
}

// ArtifactInfo records a built artifact (e.g. the EPUB) inside the release.
type ArtifactInfo struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

// Provenance records how a release was produced (provenance.json, schema 1):
// engine and resource versions, book parameters, per-chapter models, reported
// token usage, AI disclosure and the deduplicated source list that a
// commercial-use license audit runs against (ADR 29).
type Provenance struct {
	Schema       int             `json:"schema"`
	Engine       EngineInfo      `json:"engine"`
	Parameters   ParameterInfo   `json:"parameters"`
	Models       []ChapterModel  `json:"models"`
	TokenUsage   book.TokenUsage `json:"token_usage"`
	AIDisclosure AIDisclosure    `json:"ai_disclosure"`
	Sources      []SourceInfo    `json:"sources"`
}

// EngineInfo records jianwu and embedded resource versions.
type EngineInfo struct {
	JianwuVersion           string `json:"jianwu_version"`
	Archetype               string `json:"archetype"`
	ArchetypeLibraryVersion string `json:"archetype_library_version,omitempty"`
	GrillTreeVersion        string `json:"grill_tree_version,omitempty"`
	StyleGuideVersion       string `json:"style_guide_version,omitempty"`
	SamplesVersion          string `json:"samples_version,omitempty"`
}

// ParameterInfo records the grill parameters that shaped the book.
type ParameterInfo struct {
	Audience string `json:"audience,omitempty"`
	Depth    string `json:"depth,omitempty"`
	Goal     string `json:"goal,omitempty"`
	Length   string `json:"length,omitempty"`
	Language string `json:"language,omitempty"`
}

// ChapterModel records which provider/model expanded one chapter and its
// reported token usage.
type ChapterModel struct {
	Part       int    `json:"part"`
	Chapter    int    `json:"chapter"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	Iterations int    `json:"iterations,omitempty"`
	TokensIn   int    `json:"tokens_in"`
	TokensOut  int    `json:"tokens_out"`
}

// AIDisclosure states generation assistance and the human review record.
type AIDisclosure struct {
	GeneratedBy string          `json:"generated_by"`
	HumanReview []ChapterReview `json:"human_review"`
}

// ChapterReview records one chapter's human approval timestamp.
type ChapterReview struct {
	Part       int        `json:"part"`
	Chapter    int        `json:"chapter"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
}

// SourceInfo is one deduplicated cited source. Citation has no license field
// in v1; this URL list is what a pre-commercial license audit runs against.
type SourceInfo struct {
	ID             string    `json:"id"`
	URL            string    `json:"url"`
	Title          string    `json:"title,omitempty"`
	AccessedAt     time.Time `json:"accessed_at,omitempty"`
	SearchProvider string    `json:"search_provider,omitempty"`
	ReaderProvider string    `json:"reader_provider,omitempty"`
}

// buildProvenance assembles the provenance record from book state.
func buildProvenance(in Input, opts Options) Provenance {
	p := Provenance{
		Schema: 1,
		Engine: EngineInfo{
			JianwuVersion:           opts.JianwuVersion,
			Archetype:               in.Meta.Archetype,
			ArchetypeLibraryVersion: in.Meta.Engine.ArchetypeLibraryVersion,
			GrillTreeVersion:        in.Meta.Engine.GrillTreeVersion,
			StyleGuideVersion:       in.Meta.Engine.StyleGuideVersion,
			SamplesVersion:          in.Meta.Engine.SamplesVersion,
		},
		Parameters: ParameterInfo{
			Audience: in.Meta.Parameters.Audience,
			Depth:    in.Meta.Parameters.Depth,
			Goal:     in.Meta.Parameters.Goal,
			Length:   in.Meta.Parameters.Length,
			Language: in.Meta.Language,
		},
		TokenUsage: in.Meta.TokenUsage,
		AIDisclosure: AIDisclosure{
			GeneratedBy: "jianwu " + opts.JianwuVersion + " 辅助生成",
		},
	}
	seenURL := map[string]bool{}
	for pi := range in.Outline.Parts {
		part := &in.Outline.Parts[pi]
		for ci := range part.Chapters {
			c := &part.Chapters[ci]
			if c.ExpandedWith != nil {
				p.Models = append(p.Models, ChapterModel{
					Part: part.Index, Chapter: c.Index,
					Provider: c.ExpandedWith.Provider, Model: c.ExpandedWith.Model,
					Iterations: c.ExpandedWith.Iterations,
					TokensIn:   c.ExpandedWith.Tokens.In, TokensOut: c.ExpandedWith.Tokens.Out,
				})
			}
			if c.ReviewedAt != nil {
				p.AIDisclosure.HumanReview = append(p.AIDisclosure.HumanReview, ChapterReview{
					Part: part.Index, Chapter: c.Index, ReviewedAt: c.ReviewedAt,
				})
			}
			for _, cit := range c.Citations {
				if seenURL[cit.URL] {
					continue
				}
				seenURL[cit.URL] = true
				p.Sources = append(p.Sources, SourceInfo{
					ID: cit.ID, URL: cit.URL, Title: cit.Title, AccessedAt: cit.AccessedAt,
					SearchProvider: cit.SearchProvider, ReaderProvider: cit.ReaderProvider,
				})
			}
		}
	}
	return p
}

// sha256Hex returns the hex sha256 of data.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
