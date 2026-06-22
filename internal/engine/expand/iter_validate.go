package expand

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zhurong/jianwu/internal/provider/llm"
)

// RunValidate executes iteration 3: LLM self-checks and revises the draft.
// Returns revised markdown + claims with has_citation flags.
func RunValidate(
	ctx context.Context,
	chatter llm.Chatter,
	draft string,
	notes ResearchNotes,
) (ValidationResult, error) {
	sysBytes, _ := loadTemplate("system_validate")
	userBytes, _ := loadTemplate("user_validate")
	sys, err := renderExpand("system_validate", sysBytes, map[string]any{})
	if err != nil {
		return ValidationResult{}, err
	}
	notesJSON, _ := jsonMarshalNotes(notes)
	user, err := renderExpand("user_validate", userBytes, map[string]any{
		"Draft":         draft,
		"ResearchNotes": notesJSON,
	})
	if err != nil {
		return ValidationResult{}, err
	}

	schema, _ := JSONSchemaValidation()
	resp, err := chatter.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		},
		JSONSchema: schema,
	})
	if err != nil {
		return ValidationResult{}, fmt.Errorf("validate llm chat: %w", err)
	}
	var result ValidationResult
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		return ValidationResult{}, fmt.Errorf("parse validation result: %w (content: %s)", err, truncate(resp.Content, 500))
	}
	return result, nil
}
