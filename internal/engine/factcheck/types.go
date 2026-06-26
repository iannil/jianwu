package factcheck

import (
	"context"
	"fmt"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/reader"
)

// Input carries what the factcheck engine needs.
type Input struct {
	ChapterTitle string
	Claims       []book.Claim
	Citations    []book.Citation
	// ClaimWhitelist contains claim texts that have been verified in other chapters.
	// Claims found in this set are auto-verified without an LLM call.
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

// Run executes fact-check: for each claim with HasCitation=true, find its
// matching citation (by index), read the source URL, and ask LLM to verify.
// v1 limitation: claims match citations by position (claim[i] ↔ citation[i]).
func Run(
	ctx context.Context,
	chatter llm.Chatter,
	rd reader.Reader,
	in Input,
) (*Output, error) {
	out := &Output{}

	for i, claim := range in.Claims {
		if !claim.HasCitation {
			continue
		}

		// Check whitelist: if this claim was already verified in another chapter, skip LLM call.
		if in.ClaimWhitelist != nil && in.ClaimWhitelist[claim.Text] {
			out.Verdicts = append(out.Verdicts, ClaimVerdict{
				ClaimText:  claim.Text,
				Verified:   true,
				Reasoning:  "previously verified in another chapter (whitelist)",
				CitationID: fmt.Sprintf("%d", i+1),
			})
			continue
		}

		if i >= len(in.Citations) {
			continue // no matching citation
		}
		c := in.Citations[i]
		if c.URL == "" {
			continue
		}

		// Read the source content (best-effort).
		content, err := rd.Read(ctx, c.URL)
		if err != nil {
			out.SourceErrors = append(out.SourceErrors, c.URL)
			continue
		}

		// Ask LLM to verify the claim against the source.
		verdict, err := verifyClaim(ctx, chatter, claim.Text, content.Markdown, c.ID)
		if err != nil {
			out.Verdicts = append(out.Verdicts, ClaimVerdict{
				ClaimText:  claim.Text,
				Verified:   false,
				Reasoning:  fmt.Sprintf("fact-check LLM error: %v", err),
				CitationID: c.ID,
			})
			continue
		}
		out.Verdicts = append(out.Verdicts, *verdict)
	}

	return out, nil
}
