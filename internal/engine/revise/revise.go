package revise

import (
	"context"
	"fmt"
	"strings"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
)

// Input carries the chapter content and fact-check results for revision.
type Input struct {
	ChapterTitle string
	Markdown     string               // current chapter markdown
	Citations    []book.Citation      // all citations
	Unverified   []book.Claim         // claims that failed fact-check
	Verdicts     []book.ClaimVerdict  // fact-check verdicts with suggested_rewrites
}

// Output is the revised chapter.
type Output struct {
	RevisedMarkdown string // LLM-produced revised markdown
}

// item bundles a claim with its suggested rewrite for prompt rendering.
type item struct {
	ClaimText  string
	Suggestion string
}

// Run sends the chapter + fact-check results to LLM for revision.
func Run(ctx context.Context, chatter llm.Chatter, in Input) (*Output, error) {
	sysBytes, err := loadSystem()
	if err != nil {
		return nil, fmt.Errorf("load system template: %w", err)
	}
	userBytes, err := loadUser()
	if err != nil {
		return nil, fmt.Errorf("load user template: %w", err)
	}

	// Build unverified claims with suggestions.
	var items []item
	if len(in.Verdicts) > 0 {
		for _, v := range in.Verdicts {
			if !v.Verified {
				items = append(items, item{ClaimText: v.ClaimText, Suggestion: v.SuggestedRewrite})
			}
		}
	} else {
		for _, c := range in.Unverified {
			items = append(items, item{ClaimText: c.Text})
		}
	}

	sys, err := renderRevise("system", sysBytes, nil)
	if err != nil {
		return nil, err
	}
	user, err := renderRevise("user", userBytes, map[string]any{
		"ChapterTitle":  in.ChapterTitle,
		"Markdown":      in.Markdown,
		"ClaimCount":    len(items),
		"ClaimList":     formatClaimList(items),
		"CitationCount": len(in.Citations),
	})
	if err != nil {
		return nil, err
	}

	resp, err := chatter.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}
	if resp.Content == "" {
		return nil, fmt.Errorf("revised content is empty")
	}
	return &Output{RevisedMarkdown: resp.Content}, nil
}

func formatClaimList(items []item) string {
	if len(items) == 0 {
		return "(none)"
	}
	var b strings.Builder
	for _, it := range items {
		b.WriteString("- " + it.ClaimText)
		if it.Suggestion != "" {
			b.WriteString("\n  suggestion: " + it.Suggestion)
		}
		b.WriteString("\n")
	}
	return b.String()
}
