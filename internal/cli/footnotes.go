// internal/cli/footnotes.go
package cli

import (
	"fmt"
	"regexp"
)

var footnoteTokenRe = regexp.MustCompile(`\[\^([^\]]+)\]`)

// renumberFootnotes remaps every [^id] token in body to a global sequential number
// starting at `start`, assigning new numbers in order of first appearance. Both the
// inline reference [^id] and the definition line [^id]: share the id and are remapped
// consistently. Returns the rewritten body and the next free number.
func renumberFootnotes(body string, start int) (string, int) {
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
