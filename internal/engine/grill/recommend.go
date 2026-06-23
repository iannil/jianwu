package grill

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/iannil/jianwu/internal/provider/llm"
)

// Recommend asks the LLM for a recommendation on one dimension.
// Returns the LLM's text response (first line is the recommended value, rest is reasoning).
func Recommend(ctx context.Context, chatter llm.Chatter, dim Dimension, answers map[string]string) (string, error) {
	sysBytes, err := loadSystem()
	if err != nil {
		return "", fmt.Errorf("load system template: %w", err)
	}
	userBytes, err := loadUser()
	if err != nil {
		return "", fmt.Errorf("load user template: %w", err)
	}
	data := recommendData{
		DimID:        dim.ID,
		DimName:      dim.Name,
		DimQuestion:  dim.Question,
		DimOptions:   strings.Join(dim.Options, ", "),
		DimDefault:   dim.DefaultValue,
		PriorAnswers: formatAnswers(answers),
		Topic:        answers["topic"],
	}
	sys, err := renderGrillTemplate("system", sysBytes, data)
	if err != nil {
		return "", err
	}
	user, err := renderGrillTemplate("user", userBytes, data)
	if err != nil {
		return "", err
	}
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		},
	}
	resp, err := chatter.Chat(ctx, req)
	if err != nil {
		return "", fmt.Errorf("llm chat: %w", err)
	}
	return resp.Content, nil
}

type recommendData struct {
	DimID        string
	DimName      string
	DimQuestion  string
	DimOptions   string
	DimDefault   string
	PriorAnswers string
	Topic        string
}

func formatAnswers(answers map[string]string) string {
	if len(answers) == 0 {
		return "(无)"
	}
	keys := orderedKeys(answers)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k + ": " + answers[k] + "\n")
	}
	return b.String()
}

// orderedKeys returns a stable key order. topic first if present, then alphabetical.
func orderedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func renderGrillTemplate(name string, raw []byte, data any) (string, error) {
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
