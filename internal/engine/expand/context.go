package expand

import (
	"fmt"

	"github.com/zhurong/jianwu/internal/archetypes"
	"github.com/zhurong/jianwu/internal/style"
)

// DraftContext holds the resolved injection material for one book's archetype.
// Loaded once per Generate call and shared across draft + validate iterations.
type DraftContext struct {
	ArchetypeText string // compact archetype rendering
	SampleText    string // verbatim samples[id], or degrade sentinel
	StyleGuide    string // full style-guide.md
}

// loadDraftContext resolves archetypeID into injection material.
// Q9: archetype-miss is a hard failure (book meta integrity); sample-miss degrades.
func loadDraftContext(archetypeID string) (DraftContext, error) {
	archs, err := archetypes.Load()
	if err != nil {
		return DraftContext{}, fmt.Errorf("load archetypes: %w", err)
	}
	arch, ok := archs[archetypeID]
	if !ok {
		return DraftContext{}, fmt.Errorf("archetype %q not found", archetypeID)
	}
	samples, err := style.LoadSamples()
	if err != nil {
		return DraftContext{}, fmt.Errorf("load samples: %w", err)
	}
	guide, err := style.LoadGuide()
	if err != nil {
		return DraftContext{}, fmt.Errorf("load style guide: %w", err)
	}
	return DraftContext{
		ArchetypeText: marshalArchetype(arch),
		SampleText:    pickSample(samples, archetypeID),
		StyleGuide:    guide,
	}, nil
}

func pickSample(samples map[string]string, id string) string {
	if s, ok := samples[id]; ok {
		return s
	}
	return "(no samples for this archetype)"
}

// marshalArchetype renders a compact text view for prompt injection.
// Mirrors the scaffolding/outline packages' approach (not a real YAML round-trip).
func marshalArchetype(a *archetypes.Archetype) string {
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
