package book

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

var (
	nonASCII            = regexp.MustCompile(`[^a-zA-Z0-9\s-]`)
	whitespace          = regexp.MustCompile(`\s+`)
	leadingTrailingDash = regexp.MustCompile(`^-+|-+$`)
)

// Slugify converts a title to a URL-safe slug.
// ASCII titles: lowercased, whitespace and punctuation to dashes.
// Non-ASCII titles (Chinese, etc.): deterministic hash fallback,
// so the same title always produces the same slug.
func Slugify(title string) string {
	t := strings.TrimSpace(strings.ToLower(title))
	if t == "" {
		return ""
	}
	if isASCII(t) {
		t = nonASCII.ReplaceAllString(t, "")
		t = whitespace.ReplaceAllString(t, "-")
		t = leadingTrailingDash.ReplaceAllString(t, "")
		if t == "" {
			return ""
		}
		return t
	}
	// Non-ASCII: hash fallback (short, deterministic)
	t = strings.ToLower(strings.TrimSpace(title))
	t = norm.NFC.String(t)
	sum := sha256.Sum256([]byte(t))
	return "b-" + hex.EncodeToString(sum[:])[:16]
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}
