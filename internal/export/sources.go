package export

import (
	"strings"

	"github.com/iannil/jianwu/internal/book"
)

// ClaimStatus classifies one claim for the reader-facing source page.
// This mirrors the disclosure principle of the release gate: unverified or
// failed claims are shown, never hidden (ADR 29).
type ClaimStatus string

// Claim verification states shown to readers.
const (
	StatusSupported  ClaimStatus = "supported"
	StatusFailed     ClaimStatus = "failed"
	StatusUnverified ClaimStatus = "unverified"
	StatusUncited    ClaimStatus = "uncited"
)

// statusLabels are the reader-facing status strings.
var statusLabels = map[ClaimStatus]struct{ label, class string }{
	StatusSupported:  {"✓ 来源支持", "st-supported"},
	StatusFailed:     {"✗ 未通过", "st-failed"},
	StatusUnverified: {"未核验", "st-unverified"},
	StatusUncited:    {"无引用", "st-uncited"},
}

// claimStatus matches verdicts to a claim the same way the release gate
// does: by claim text first, then by citation ID membership (factcheck
// records one verdict per claim-source pair).
func claimStatus(c book.Claim, verdicts []book.ClaimVerdict) ClaimStatus {
	if len(c.CitationIDs) == 0 {
		return StatusUncited
	}
	if len(verdicts) == 0 {
		return StatusUnverified
	}
	matched := false
	for _, v := range verdicts {
		match := v.ClaimText == c.Text
		if !match {
			for _, id := range c.CitationIDs {
				if v.CitationID == id {
					match = true
					break
				}
			}
		}
		if !match {
			continue
		}
		matched = true
		if v.Verified {
			return StatusSupported
		}
	}
	if !matched {
		return StatusUnverified
	}
	return StatusFailed
}

// SourcesXHTML renders the per-chapter "来源与核验" section from the
// structured citations/claims/verdicts. Always emitted — the disclosure is
// the product differentiator, not an optional appendix.
func SourcesXHTML(c book.OutlineChapter) string {
	var b strings.Builder
	b.WriteString(`<section class="sources" epub:type="endnotes">` + "\n")
	b.WriteString("<h2>来源与核验</h2>\n")

	if len(c.Citations) == 0 && len(c.Claims) == 0 {
		b.WriteString("<p>本章无引用论断。</p>\n</section>\n")
		return b.String()
	}

	if len(c.Citations) > 0 {
		b.WriteString("<h3>来源</h3>\n<ul>\n")
		for _, cit := range c.Citations {
			b.WriteString(`<li>[` + Esc(cit.ID) + "] ")
			if cit.URL != "" {
				b.WriteString(`<a href="` + Esc(cit.URL) + "\">" + Esc(cit.Title) + "</a>")
			} else {
				b.WriteString(Esc(cit.Title))
			}
			if !cit.AccessedAt.IsZero() {
				b.WriteString(` <span class="src-accessed">（访问于 ` + Esc(cit.AccessedAt.Format("2006-01-02")) + "）</span>")
			}
			b.WriteString("</li>\n")
		}
		b.WriteString("</ul>\n")
	}

	if len(c.Claims) > 0 {
		b.WriteString("<h3>论断核验</h3>\n<table>\n<tr><th>论断</th><th>状态</th><th>说明</th></tr>\n")
		for _, cl := range c.Claims {
			st := claimStatus(cl, c.Verdicts)
			meta := statusLabels[st]
			b.WriteString("<tr><td>" + Esc(cl.Text) + `</td><td class="` + meta.class + "\">" + meta.label + "</td><td>")
			if reasoning := matchedReasoning(cl, c.Verdicts); reasoning != "" {
				b.WriteString("<details><summary>核验说明</summary><p>" + Esc(reasoning) + "</p></details>")
			} else {
				b.WriteString("—")
			}
			b.WriteString("</td></tr>\n")
		}
		b.WriteString("</table>\n")
	}

	b.WriteString("</section>\n")
	return b.String()
}

// matchedReasoning returns the first matching verdict's explanation, if any.
func matchedReasoning(c book.Claim, verdicts []book.ClaimVerdict) string {
	for _, v := range verdicts {
		if v.ClaimText == c.Text {
			return v.Reasoning
		}
		for _, id := range c.CitationIDs {
			if v.CitationID == id {
				return v.Reasoning
			}
		}
	}
	return ""
}
