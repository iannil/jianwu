package factcheck

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/reader"
)

// Input carries what the factcheck engine needs.
type Input struct {
	ChapterTitle string
	Claims       []book.Claim
	Citations    []book.Citation
	// ClaimWhitelist is retained for compatibility and never bypasses source verification.
	ClaimWhitelist map[string]bool
}

// ClaimVerdict is the result of verifying one claim against its cited source.
type ClaimVerdict struct {
	ClaimText        string `json:"claim_text"`
	Verified         bool   `json:"verified"`
	Reasoning        string `json:"reasoning"`
	SuggestedRewrite string `json:"suggested_rewrite,omitempty"`
	CitationID       string `json:"citation_id,omitempty"`
}

// Output is the aggregated fact-check result.
type Output struct {
	Verdicts     []ClaimVerdict
	SourceErrors []string // URLs that failed to read
}

// Run verifies each explicit claim/source association. Legacy unlinked claims
// remain visibly unverified; citation array order never establishes an association.
func Run(ctx context.Context, chatter llm.Chatter, rd reader.Reader, in Input) (*Output, error) {
	out := &Output{}
	byID := make(map[string]book.Citation)
	duplicate := make(map[string]bool)
	for _, c := range in.Citations {
		if _, ok := byID[c.ID]; ok {
			duplicate[c.ID] = true
		}
		byID[c.ID] = c
	}
	for _, claim := range in.Claims {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(claim.CitationIDs) == 0 {
			out.Verdicts = append(out.Verdicts, ClaimVerdict{ClaimText: claim.Text, Reasoning: "unverified: no explicit citation IDs; re-expand legacy chapters"})
			continue
		}
		seen := make(map[string]bool)
		for _, id := range claim.CitationIDs {
			if seen[id] {
				continue
			}
			seen[id] = true
			v := ClaimVerdict{ClaimText: claim.Text, CitationID: id}
			c, ok := byID[id]
			u, parseErr := url.Parse(c.URL)
			switch {
			case !ok || id == "":
				v.Reasoning = "unverified: citation ID is missing from chapter sources"
			case duplicate[id]:
				v.Reasoning = "unverified: duplicate citation ID"
			case parseErr != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http"):
				v.Reasoning = "unverified: source URL is missing or invalid"
			case rd == nil:
				v.Reasoning = "unverified: source reader unavailable"
			default:
				content, err := rd.Read(ctx, c.URL)
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				if err != nil || strings.TrimSpace(content.Markdown) == "" {
					out.SourceErrors = append(out.SourceErrors, c.URL)
					v.Reasoning = "unverified: source could not be read"
				} else if issue := reader.ContentIssue(content.Markdown); issue != "" {
					// Non-empty but unusable (login wall, nav-only shell):
					// distinguish from a genuine content mismatch.
					out.SourceErrors = append(out.SourceErrors, c.URL)
					v.Reasoning = fmt.Sprintf("unverified: source unusable (%s)", issue)
				} else if chatter == nil {
					v.Reasoning = "unverified: verifier unavailable"
				} else {
					verified, err := verifyClaim(ctx, chatter, claim.Text, content.Markdown, id)
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					if err != nil {
						v.Reasoning = fmt.Sprintf("fact-check LLM error: %v", err)
					} else {
						v = *verified
					}
				}
			}
			out.Verdicts = append(out.Verdicts, v)
		}
	}
	return out, nil
}
