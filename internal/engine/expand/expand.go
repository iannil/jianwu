package expand

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/iannil/jianwu/internal/provider/llm"
)

// Generate runs all 3 iterations for one chapter.
// tools may be nil (skips web search); webSearchEnabled controls iter 1 grounding.
func Generate(
	ctx context.Context,
	chatter llm.Chatter,
	tools *ToolRegistry,
	in ExpandInput,
) (*ExpandOutput, error) {
	// Resolve injection material once; archetype-miss fails fast (Q1, Q9).
	dc, err := loadDraftContext(in.ArchetypeID)
	if err != nil {
		return nil, fmt.Errorf("load draft context: %w", err)
	}

	// Iter 1: research
	slog.Debug("expand: iter 1 research", "chapter", in.ChapterTitle)
	notes, err := RunResearch(ctx, chatter, tools, in)
	if err != nil {
		return nil, fmt.Errorf("iter 1 research: %w", err)
	}
	slog.Debug("expand: iter 1 research complete", "chapter", in.ChapterTitle)

	// Iter 2: draft
	slog.Debug("expand: iter 2 draft", "chapter", in.ChapterTitle)
	draft, err := RunDraft(ctx, chatter, in, dc, notes)
	if err != nil {
		return nil, fmt.Errorf("iter 2 draft: %w", err)
	}
	slog.Debug("expand: iter 2 draft complete", "chapter", in.ChapterTitle,
		"length", len(draft))

	// Iter 3: validate
	slog.Debug("expand: iter 3 validate", "chapter", in.ChapterTitle)
	validated, err := RunValidate(ctx, chatter, draft, notes, dc.StyleGuide)
	if err != nil {
		return nil, fmt.Errorf("iter 3 validate: %w", err)
	}
	slog.Debug("expand: iter 3 validate complete", "chapter", in.ChapterTitle)

	// Build output: parse footnotes from final markdown, merge with tool registry metadata.
	finalMD := validated.RevisedMarkdown
	if finalMD == "" {
		finalMD = draft // fallback if LLM returned empty
	}
	if finalMD == "" {
		return nil, fmt.Errorf("draft output is empty after iter 2 and 3")
	}
	defs := ParseFootnotes(finalMD)
	citations := mergeCitations(defs, tools)

	// Count unverified claims.
	var unverified []Claim
	for _, c := range validated.Claims {
		if !c.HasCitation {
			unverified = append(unverified, c)
		}
	}

	return &ExpandOutput{
		Markdown:         finalMD,
		Citations:        citations,
		UnverifiedClaims: unverified,
		WordCount:        CountWords(finalMD),
		Research:         notes,
	}, nil
}

// mergeCitations combines parsed footnote definitions with tool registry metadata.
// Footnote ID becomes Citation.ID; URL matches registry entry for metadata.
func mergeCitations(defs map[string]FootnoteDef, tools *ToolRegistry) []Citation {
	if tools == nil {
		// No tools; build from defs only.
		out := make([]Citation, 0, len(defs))
		for id, d := range defs {
			out = append(out, Citation{ID: id, URL: d.URL, Title: d.Title})
		}
		return out
	}
	registryCites := tools.Citations()
	byURL := make(map[string]Citation, len(registryCites))
	for _, c := range registryCites {
		byURL[c.URL] = c
	}
	out := make([]Citation, 0, len(defs))
	for id, d := range defs {
		c := Citation{ID: id, URL: d.URL, Title: d.Title}
		if reg, ok := byURL[d.URL]; ok {
			c.AccessedAt = reg.AccessedAt
			c.Snippet = reg.Snippet
			c.SearchProvider = reg.SearchProvider
			c.ReaderProvider = reg.ReaderProvider
		}
		out = append(out, c)
	}
	return out
}
