package scaffolding

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/zhurong/jianwu/internal/book"
	"github.com/zhurong/jianwu/internal/provider/llm"
)

// GenerateChapter produces a scaffold for one chapter via LLM call.
func GenerateChapter(ctx context.Context, chatter llm.Chatter, in ChapterInput) (*ChapterOutput, error) {
	data, err := buildPromptData(in)
	if err != nil {
		return nil, err
	}
	sysBytes, err := loadSystem()
	if err != nil {
		return nil, fmt.Errorf("load system template: %w", err)
	}
	userBytes, err := loadUser()
	if err != nil {
		return nil, fmt.Errorf("load user template: %w", err)
	}
	sys, err := renderTemplate("system", sysBytes, data)
	if err != nil {
		return nil, err
	}
	user, err := renderTemplate("user", userBytes, data)
	if err != nil {
		return nil, err
	}
	schema, err := JSONSchema()
	if err != nil {
		return nil, fmt.Errorf("generate schema: %w", err)
	}

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		},
		JSONSchema: schema,
	}
	resp, err := chatter.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}

	var parsed chapterSchema
	if err := json.Unmarshal([]byte(resp.Content), &parsed); err != nil {
		return nil, fmt.Errorf("parse chapter JSON: %w (content was: %s)", err, truncate(resp.Content, 500))
	}
	return &ChapterOutput{
		Abstract:           parsed.Abstract,
		KeyConcepts:        parsed.KeyConcepts,
		LearningObjectives: parsed.LearningObjectives,
		SuggestedExamples:  parsed.SuggestedExamples,
		Status:             book.StatusScaffolded,
	}, nil
}

func renderTemplate(name string, raw []byte, data any) (string, error) {
	tmpl, err := template.New(name).Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("parse %s template: %w", name, err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute %s template: %w", name, err)
	}
	return buf.String(), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
