package reader

import (
	"strings"
	"unicode/utf8"
)

// loginWallMarkers are substrings that identify a login/pay/bot-check interstitial
// rather than real page content. They are only checked near the top of the page,
// where such walls render.
var loginWallMarkers = []string{
	"请您登录", "登录后查看", "登录并查看", "扫码登录", "人机验证", "访问验证",
	"Sign in to continue", "Log in to continue", "Sign up to continue",
	"Subscribe to continue", "subscribe to read",
	"Just a moment...", "Enable JavaScript and cookies to continue",
	"verifying you are human",
}

const (
	// markerWindowRunes is how far into the page login-wall markers are matched.
	markerWindowRunes = 1200
	// loginWalledMaxRunes caps the page length for a marker hit to count as a
	// wall (long articles may legitimately mention login flows).
	loginWalledMaxRunes = 5000
	// minUsableRunes is the minimum markdown length to count as content.
	minUsableRunes = 500
)

// ContentIssue reports whether markdown looks like usable page content.
// It returns "" when the content is usable, or a short human-readable reason
// ("login-walled", "no content (too short)") usable in logs and verdicts.
func ContentIssue(markdown string) string {
	md := strings.TrimSpace(markdown)
	if md == "" {
		return "no content (empty)"
	}
	n := utf8.RuneCountInString(md)
	prefix := md
	if r := []rune(md); len(r) > markerWindowRunes {
		prefix = string(r[:markerWindowRunes])
	}
	if n <= loginWalledMaxRunes {
		for _, m := range loginWallMarkers {
			if strings.Contains(prefix, m) {
				return "login-walled"
			}
		}
	}
	if n < minUsableRunes {
		return "no content (too short)"
	}
	return ""
}
