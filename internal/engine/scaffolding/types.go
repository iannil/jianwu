package scaffolding

import (
	"fmt"

	"github.com/zhurong/jianwu/internal/archetypes"
	"github.com/zhurong/jianwu/internal/book"
	"github.com/zhurong/jianwu/internal/style"
)

// ChapterInput is the input for generating one chapter's scaffold.
type ChapterInput struct {
	ArchetypeID string
	// Part context
	PartIndex int
	PartTitle string
	PartRole  string
	// Chapter context (from outline)
	ChapterIndex int
	ChapterTitle string
	// Book parameters
	Topic    string
	Audience string
	Depth    string
	Goal     string
	Length   string
	Language string
}

// ChapterOutput is the generated scaffold for one chapter.
// Aliased to book.OutlineChapter so callers can directly assign.
type ChapterOutput = book.OutlineChapter

// promptData is the template context.
type promptData struct {
	Archetype      string
	Samples        string
	PartRole       string
	PartTitle      string
	ChapterTitle   string
	Topic          string
	Audience       string
	Depth          string
	Goal           string
	Length         string
	Language       string
}

// buildPromptData assembles prompt data from a ChapterInput.
// Returns an error if archetype or samples can't be loaded.
func buildPromptData(in ChapterInput) (promptData, error) {
	if err := in.validate(); err != nil {
		return promptData{}, err
	}
	archs, err := archetypes.Load()
	if err != nil {
		return promptData{}, fmt.Errorf("load archetypes: %w", err)
	}
	arch, ok := archs[in.ArchetypeID]
	if !ok {
		return promptData{}, fmt.Errorf("archetype %q not found", in.ArchetypeID)
	}
	samples, err := style.LoadSamples()
	if err != nil {
		return promptData{}, fmt.Errorf("load samples: %w", err)
	}
	sampleText, ok := samples[in.ArchetypeID]
	if !ok {
		sampleText = "(no samples for this archetype)"
	}
	return promptData{
		Archetype:    yamlMarshalArchetype(arch),
		Samples:      sampleText,
		PartRole:     in.PartRole,
		PartTitle:    in.PartTitle,
		ChapterTitle: in.ChapterTitle,
		Topic:        in.Topic,
		Audience:     in.Audience,
		Depth:        in.Depth,
		Goal:         in.Goal,
		Length:       in.Length,
		Language:     in.Language,
	}, nil
}

func (in ChapterInput) validate() error {
	var missing []string
	if in.ArchetypeID == "" {
		missing = append(missing, "archetype_id")
	}
	if in.ChapterTitle == "" {
		missing = append(missing, "chapter_title")
	}
	if in.PartRole == "" {
		missing = append(missing, "part_role")
	}
	if in.Topic == "" {
		missing = append(missing, "topic")
	}
	if in.Language == "" {
		missing = append(missing, "language")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", joinComma(missing))
	}
	return nil
}

func joinComma(xs []string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += ", "
		}
		out += x
	}
	return out
}

// yamlMarshalArchetype produces a compact text rendering of an archetype.
// Same approach as outline package: minimal pretty-printer, not real YAML round-trip.
func yamlMarshalArchetype(a *archetypes.Archetype) string {
	out := ""
	out += "id: " + a.ID + "\n"
	out += "name_zh: " + a.Name.Zh + "\n"
	out += "name_en: " + a.Name.En + "\n"
	out += "description: " + a.Description + "\n"
	out += "\nparts:\n"
	for _, p := range a.Parts {
		out += "  - role: " + p.Role + "\n"
		out += "    guidance: " + p.Guidance + "\n"
		if len(p.ChapterRoleHints) > 0 {
			out += "    chapter_role_hints:\n"
			for _, h := range p.ChapterRoleHints {
				out += "      - " + h + "\n"
			}
		}
	}
	return out
}
