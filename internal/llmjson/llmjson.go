// Package llmjson parses JSON payloads out of LLM responses.
//
// Some providers (notably GLM) habitually wrap structured output in
// Markdown code fences (```json ... ```), and Chinese-heavy output from
// some models (notably DeepSeek) leaves quotes inside string values
// unescaped (e.g. "把读者从"掌握"带到…"). Unmarshal strips the fence
// wrapper and, when strict decoding fails, retries once with inner
// quotes repaired, so callers do not each need to tolerate provider
// quirks.
package llmjson

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Unmarshal decodes an LLM response into v. The content may optionally be
// wrapped in a Markdown code fence; any leading fence marker (with optional
// language tag) and trailing fence marker are removed first. If strict
// decoding fails, one repair pass escapes unescaped quotes inside string
// values and retries.
func Unmarshal(content string, v any) error {
	trimmed := strings.TrimSpace(stripFence(content))
	if trimmed == "" {
		return fmt.Errorf("llmjson: empty content")
	}
	err := json.Unmarshal([]byte(trimmed), v)
	if err == nil {
		return nil
	}
	if repaired := repairQuotes(trimmed); repaired != trimmed {
		if retryErr := json.Unmarshal([]byte(repaired), v); retryErr == nil {
			return nil
		}
	}
	return fmt.Errorf("llmjson: %w", err)
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

// repairQuotes escapes double quotes inside JSON string values that the
// model left unescaped. It walks the text tracking string state; a quote
// encountered inside a string is treated as the closing quote only when the
// next non-space rune is a structural character (:, , } ] or end of input).
// Otherwise it is prose punctuation and gets escaped. ASCII structural
// characters are distinct from CJK punctuation, so quotes quoted around
// Chinese words are repaired reliably.
func repairQuotes(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 16)
	inString := false
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\\' && inString && i+1 < len(runes) {
			// Keep existing escapes verbatim.
			b.WriteRune(r)
			b.WriteRune(runes[i+1])
			i++
			continue
		}
		if r != '"' {
			b.WriteRune(r)
			continue
		}
		if !inString {
			inString = true
			b.WriteRune(r)
			continue
		}
		if isStringClose(runes[i+1:]) {
			inString = false
			b.WriteRune(r)
			continue
		}
		b.WriteString(`\"`)
	}
	return b.String()
}

// isStringClose reports whether a quote followed by rest should be treated
// as the string terminator: the next non-space rune must be a structural
// character or the input must end.
func isStringClose(rest []rune) bool {
	for _, r := range rest {
		switch r {
		case ' ', '\t', '\n', '\r':
			continue
		case ':', ',', '}', ']':
			return true
		default:
			return false
		}
	}
	return true
}
