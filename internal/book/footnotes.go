package book

import (
	"fmt"
	"regexp"
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
