// Package release implements the ADR 29 publishing layer: an immutable,
// versioned release directory per published book, guarded by a hard gate.
package release

import (
	"fmt"

	"github.com/iannil/jianwu/internal/book"
)

// GateReport lists publish blockers (hard gate) and warnings (disclosed but
// non-blocking). Warnings surface quality state that readers will also see in
// the EPUB source-verification pages; publishing never hides them.
type GateReport struct {
	Blockers []string `json:"blockers,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// OK reports whether the hard gate passes.
func (r GateReport) OK() bool { return len(r.Blockers) == 0 }

// Stats summarizes claim/citation verification state across a book. Recorded
// in the manifest and used for gate warnings.
type Stats struct {
	ChaptersTotal    int `json:"chapters_total"`
	ClaimsTotal      int `json:"claims_total"`
	ClaimsUnverified int `json:"claims_unverified"`
	CitationsTotal   int `json:"citations_total"`
	VerdictsFailed   int `json:"verdicts_failed"`
}

// CheckGate validates the ADR 29 publish preconditions against meta/outline
// and computes book-level stats. Hard gate: book + all chapters final, stable
// id, license set. The status machine already re-arms this gate after any
// revise/expand (they reset Meta.Status to draft), so no extra state here.
func CheckGate(meta *book.Meta, outline *book.Outline) (GateReport, Stats) {
	var rep GateReport
	var st Stats
	if meta.Status != book.BookStatusFinal {
		rep.Blockers = append(rep.Blockers,
			fmt.Sprintf("图书状态为 %q：需全部章节 review 后 finalize", meta.Status))
	}
	if meta.ID == "" {
		rep.Blockers = append(rep.Blockers,
			"meta.json 缺少 id：无法生成稳定书标识（urn:uuid）")
	}
	if meta.License == "" {
		rep.Blockers = append(rep.Blockers,
			"meta.json 缺少 license 字段：发布硬门（ADR 29），设置后重试")
	}

	var notFinal, missingBy int
	for pi := range outline.Parts {
		for ci := range outline.Parts[pi].Chapters {
			c := &outline.Parts[pi].Chapters[ci]
			st.ChaptersTotal++
			if c.Status != book.StatusFinal {
				notFinal++
			}
			if c.ReviewedAt != nil && c.ReviewedBy == "" {
				missingBy++
			}
			st.ClaimsTotal += len(c.Claims)
			for _, cl := range c.Claims {
				if !claimVerified(cl, c.Verdicts) {
					st.ClaimsUnverified++
				}
			}
			st.CitationsTotal += len(c.Citations)
			for _, v := range c.Verdicts {
				if !v.Verified {
					st.VerdictsFailed++
				}
			}
		}
	}
	if notFinal > 0 {
		rep.Blockers = append(rep.Blockers,
			fmt.Sprintf("%d 个章节状态非 final：内容修改后需重新 review/finalize", notFinal))
	}
	if st.ClaimsUnverified > 0 {
		rep.Warnings = append(rep.Warnings,
			fmt.Sprintf("%d 条论断未核验（无引用或无核验结论）；将以披露形式进入来源核验页", st.ClaimsUnverified))
	}
	if st.VerdictsFailed > 0 {
		rep.Warnings = append(rep.Warnings,
			fmt.Sprintf("%d 条核验结论未通过", st.VerdictsFailed))
	}
	if missingBy > 0 {
		rep.Warnings = append(rep.Warnings,
			fmt.Sprintf("%d 个章节缺少 reviewed_by 署名", missingBy))
	}
	return rep, st
}

// claimVerified reports whether any verdict marks the claim as supported.
// A claim without citation IDs is never verified. Verdicts are matched by
// claim text first, then by citation ID membership (factcheck records one
// verdict per claim-source pair, so one supporting verdict suffices).
func claimVerified(c book.Claim, verdicts []book.ClaimVerdict) bool {
	if len(c.CitationIDs) == 0 {
		return false
	}
	for _, v := range verdicts {
		if !v.Verified {
			continue
		}
		if v.ClaimText == c.Text {
			return true
		}
		for _, id := range c.CitationIDs {
			if v.CitationID == id {
				return true
			}
		}
	}
	return false
}
