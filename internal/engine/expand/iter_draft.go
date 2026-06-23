package expand

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
)

// RunDraft executes iteration 2: LLM writes chapter prose with [^N] footnotes.
func RunDraft(
	ctx context.Context,
	chatter llm.Chatter,
	in ExpandInput,
	dc DraftContext,
	notes ResearchNotes,
) (string, error) {
	sys, user, err := buildDraftPrompts(in, dc, notes)
	if err != nil {
		return "", err
	}
	resp, err := chatter.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return "", fmt.Errorf("draft llm chat: %w", err)
	}
	return resp.Content, nil
}

// buildDraftPrompts renders the draft system + user prompts. Pure (no I/O beyond embed),
// so it is unit-testable without an LLM (Q10).
func buildDraftPrompts(in ExpandInput, dc DraftContext, notes ResearchNotes) (string, string, error) {
	sysBytes, _ := loadTemplate("system_draft")
	sys, err := renderExpand("system_draft", sysBytes, map[string]any{
		"Language":      defaultIfEmpty(in.Language, "zh"),
		"ParagraphHint": paragraphHint(in.Length),
		"WordTarget":    wordTarget(in.Length),
		"StyleGuide":    dc.StyleGuide,
		"Samples":       dc.SampleText,
		"Archetype":     dc.ArchetypeText,
	})
	if err != nil {
		return "", "", err
	}
	notesJSON, _ := jsonMarshalNotes(notes)
	userBytes, _ := loadTemplate("user_draft")
	user, err := renderExpand("user_draft", userBytes, map[string]any{
		"Topic":         in.Topic,
		"Audience":      in.Audience,
		"Depth":         in.Depth,
		"ChapterTitle":  in.ChapterTitle,
		"Abstract":      in.Abstract,
		"KeyConcepts":   joinComma(in.KeyConcepts),
		"ResearchNotes": notesJSON,
		"Candidates":    joinCandidates(notes.Candidates),
		"PrevContext":   adjacentContext("上一章", in.PreviousChapter),
		"NextContext":   adjacentContext("下一章", in.NextChapter),
	})
	if err != nil {
		return "", "", err
	}
	return sys, user, nil
}

// adjacentContext renders one neighbor chapter for coherence, or "" when nil (Q5).
func adjacentContext(label string, ch *book.OutlineChapter) string {
	if ch == nil {
		return ""
	}
	s := label + "《" + ch.Title + "》\n摘要：" + ch.Abstract
	if len(ch.KeyConcepts) > 0 {
		s += "\n关键概念：" + joinComma(ch.KeyConcepts)
	}
	return s
}

// helpers
func defaultIfEmpty(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func paragraphHint(length string) string {
	switch length {
	case "short":
		return "50-150 字/段"
	case "long":
		return "200-400 字/段"
	default:
		return "100-200 字/段"
	}
}

func wordTarget(length string) int {
	switch length {
	case "short":
		return 1500
	case "long":
		return 4000
	default:
		return 2500
	}
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

func joinCandidates(urls []string) string {
	out := ""
	for i, u := range urls {
		if i > 0 {
			out += "\n"
		}
		out += "- " + u
	}
	return out
}

func jsonMarshalNotes(notes ResearchNotes) (string, error) {
	b, err := json.Marshal(notes)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
