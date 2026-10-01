package book

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// NOTE: this also matches any literal [^...] in prose (e.g. a regex char-class
// in a code span); acceptable for current chapter content.
var footnoteTokenRe = regexp.MustCompile(`\[\^([^\]]+)\]`)

// RenumberFootnotes remaps every [^id] token in body to a global sequential
// number starting at `start`, assigning new numbers in order of first
// appearance. Both the inline reference [^id] and the definition line [^id]:
// share the id and are remapped consistently. Returns the rewritten body and
// the next free number. Used by export targets that merge chapters.
func RenumberFootnotes(body string, start int) (string, int) {
	mapping := map[string]int{}
	next := start
	out := footnoteTokenRe.ReplaceAllStringFunc(body, func(tok string) string {
		id := footnoteTokenRe.FindStringSubmatch(tok)[1]
		n, ok := mapping[id]
		if !ok {
			n = next
			mapping[id] = n
			next++
		}
		return fmt.Sprintf("[^%d]", n)
	})
	return out, next
}

// footnoteDefLineRe matches a footnote definition line: [^N]: rest.
var footnoteDefLineRe = regexp.MustCompile(`^\[\^(\d+)\]:\s*(.+)$`)

// footnoteLinkRe matches [text](url) inside a footnote definition.
var footnoteLinkRe = regexp.MustCompile(`\[(.+?)\]\((https?://[^\s)]+)\)`)

// accessedDateRe matches an existing "accessed YYYY-MM-DD" tail.
var accessedDateRe = regexp.MustCompile(`accessed\s+\d{4}-\d{2}-\d{2}`)

// NormalizeFootnoteDates rewrites the "accessed DATE" tail of footnote
// definition lines with the real access timestamps recorded for the cited
// URLs. The draft/revise LLMs invent this date (P3 fix); the structured
// citation data is the source of truth. Definition lines whose URL has no
// entry in accessedByURL are left unchanged.
func NormalizeFootnoteDates(md string, accessedByURL map[string]time.Time) string {
	if len(accessedByURL) == 0 {
		return md
	}
	lines := strings.Split(md, "\n")
	for i, line := range lines {
		m := footnoteDefLineRe.FindStringSubmatch(strings.TrimLeft(line, " \t"))
		if m == nil {
			continue
		}
		link := footnoteLinkRe.FindStringSubmatch(m[2])
		if link == nil {
			continue
		}
		at, ok := accessedByURL[link[2]]
		if !ok || at.IsZero() {
			continue
		}
		date := "accessed " + at.UTC().Format("2006-01-02")
		if accessedDateRe.MatchString(line) {
			lines[i] = accessedDateRe.ReplaceAllString(line, date)
		} else {
			lines[i] = strings.TrimRight(line, " \t") + " " + date
		}
	}
	return strings.Join(lines, "\n")
}
