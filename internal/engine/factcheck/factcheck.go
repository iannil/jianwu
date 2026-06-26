package factcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/iannil/jianwu/internal/provider/llm"
)

// verifyClaim asks the LLM to verify one claim against its cited source.
// Returns a ClaimVerdict with the LLM's assessment.
func verifyClaim(ctx context.Context, chatter llm.Chatter, claimText, sourceContent, citationID string) (*ClaimVerdict, error) {
	sysBytes, err := loadSystem()
	if err != nil {
		return nil, fmt.Errorf("load system template: %w", err)
	}
	userBytes, err := loadUser()
	if err != nil {
		return nil, fmt.Errorf("load user template: %w", err)
	}

	sys, err := renderFactCheck("system", sysBytes, nil)
	if err != nil {
		return nil, err
	}
	user, err := renderFactCheck("user", userBytes, map[string]any{
		"Claim":         claimText,
		"SourceContent": truncate(sourceContent, 3000),
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

	// Parse the structured verdict.
	var v ClaimVerdict
	if err := json.Unmarshal([]byte(resp.Content), &v); err != nil {
		return nil, fmt.Errorf("parse verdict: %w (content: %s)", err, truncate(resp.Content, 200))
	}
	v.CitationID = citationID
	v.ClaimText = claimText
	return &v, nil
}

func renderFactCheck(name string, raw []byte, data any) (string, error) {
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
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
