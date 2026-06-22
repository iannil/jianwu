package expand

import (
	"regexp"
	"strings"
)

// footnoteDefRe matches [^N]: ... lines (footnote definitions at the bottom).
var footnoteDefRe = regexp.MustCompile(`(?m)^\[\^(\d+)\]:\s*(.+)$`)

// linkRe matches [text](url) inside a footnote definition.
var linkRe = regexp.MustCompile(`\[(.+?)\]\((https?://[^\s)]+)\)`)

// FootnoteDef represents a parsed footnote definition.
type FootnoteDef struct {
	ID    string
	Title string
	URL   string
	Raw   string // full text after [^N]:
}

// ParseFootnotes extracts footnote definitions from markdown.
// Returns map[ID]FootnoteDef where ID is the numeric string ("1", "2", ...).
func ParseFootnotes(markdown string) map[string]FootnoteDef {
	out := map[string]FootnoteDef{}
	matches := footnoteDefRe.FindAllStringSubmatch(markdown, -1)
	for _, m := range matches {
		id := m[1]
		raw := m[2]
		def := FootnoteDef{ID: id, Raw: raw}
		// Try to extract [Title](URL).
		if link := linkRe.FindStringSubmatch(raw); link != nil {
			def.Title = link[1]
			def.URL = link[2]
		} else {
			// No link format; treat raw as URL if it looks like one.
			trimmed := strings.TrimSpace(raw)
			if strings.HasPrefix(trimmed, "http") {
				def.URL = trimmed
			} else {
				def.Title = trimmed
			}
		}
		out[id] = def
	}
	return out
}

// CountWords estimates Chinese + English word count.
// Chinese: count runes; English: count words separated by whitespace.
func CountWords(markdown string) int {
	// Strip footnote definitions.
	body := footnoteDefRe.ReplaceAllString(markdown, "")
	// Strip markdown syntax: #, *, _, etc.
	body = strings.ReplaceAll(body, "#", " ")
	body = strings.ReplaceAll(body, "*", " ")
	body = strings.ReplaceAll(body, "_", " ")
	body = strings.ReplaceAll(body, "[", " ")
	body = strings.ReplaceAll(body, "]", " ")
	body = strings.ReplaceAll(body, "(", " ")
	body = strings.ReplaceAll(body, ")", " ")

	var count int
	var inChinese bool
	var currentWord []rune
	for _, r := range body {
		if r >= 0x4E00 && r <= 0x9FFF { // CJK Unified Ideograph
			if len(currentWord) > 0 {
				count++
				currentWord = currentWord[:0]
			}
			count++
			inChinese = true
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			currentWord = append(currentWord, r)
			inChinese = false
		} else {
			if len(currentWord) > 0 {
				count++
				currentWord = currentWord[:0]
			}
			if !inChinese {
				// punctuation/whitespace; do nothing
			}
		}
	}
	if len(currentWord) > 0 {
		count++
	}
	return count
}
