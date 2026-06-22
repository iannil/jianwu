package expand

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zhurong/jianwu/internal/provider/llm"
)

// RunDraft executes iteration 2: LLM writes chapter prose with [^N] footnotes.
// Free-form markdown output (no JSON Schema).
func RunDraft(
	ctx context.Context,
	chatter llm.Chatter,
	in ExpandInput,
	notes ResearchNotes,
) (string, error) {
	sysBytes, _ := loadTemplate("system_draft")
	userBytes, _ := loadTemplate("user_draft")
	sys, err := renderExpand("system_draft", sysBytes, map[string]any{
		"Language":      defaultIfEmpty(in.Language, "zh"),
		"ParagraphHint": paragraphHint(in.Length),
		"WordTarget":    wordTarget(in.Length),
		"Samples":       "(samples loaded at orchestrator level)",
		"Archetype":     "(archetype loaded at orchestrator level)",
	})
	if err != nil {
		return "", err
	}
	notesJSON, _ := jsonMarshalNotes(notes)
	user, err := renderExpand("user_draft", userBytes, map[string]any{
		"Topic":         in.Topic,
		"Audience":      in.Audience,
		"Depth":         in.Depth,
		"ChapterTitle":  in.ChapterTitle,
		"Abstract":      in.Abstract,
		"KeyConcepts":   joinComma(in.KeyConcepts),
		"ResearchNotes": notesJSON,
		"Candidates":    joinCandidates(notes.Candidates),
	})
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
