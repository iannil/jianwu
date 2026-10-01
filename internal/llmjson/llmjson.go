// Package llmjson parses JSON payloads out of LLM responses.
//
// Some providers (notably GLM) habitually wrap structured output in
// Markdown code fences (```json ... ```) even when asked for raw JSON.
// Unmarshal strips that wrapper before decoding so callers do not each
// need to tolerate provider quirks.
package llmjson

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Unmarshal decodes an LLM response into v. The content may optionally be
// wrapped in a Markdown code fence; any leading fence marker (with optional
// language tag) and trailing fence marker are removed first.
func Unmarshal(content string, v any) error {
	trimmed := strings.TrimSpace(stripFence(content))
	if trimmed == "" {
		return fmt.Errorf("llmjson: empty content")
	}
	if err := json.Unmarshal([]byte(trimmed), v); err != nil {
		return fmt.Errorf("llmjson: %w", err)
	}
	return nil
}

// stripFence removes one outer Markdown code fence if present.
func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	// Drop the opening line (``` or ```json).
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	} else {
		return s
	}
	// Drop a trailing fence marker.
	if j := strings.LastIndex(s, "```"); j >= 0 {
		s = s[:j]
	}
	return s
}
