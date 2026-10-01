package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/engine"
	"github.com/iannil/jianwu/internal/engine/expand"
	"github.com/iannil/jianwu/internal/engine/revise"
	"github.com/iannil/jianwu/internal/workspace"
)

func newReviseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revise <slug> <NN-MM>",
		Short: "Revise a chapter based on fact-check results",
		Long: `Read the chapter markdown and fact-check verdicts, then ask the LLM to
revise claims that failed verification. Updates both the chapter .md file
and outline.json.

Only chapters with status "expanded" or "reviewed" can be revised.
Run 'jianwu factcheck' first to generate verdicts.
After revision, run factcheck again and review the updated prose.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRevise(cmd, args)
		},
	}
	cmd.Flags().Bool("tokens", false, "show token usage after completion")
	return cmd
}

func runRevise(cmd *cobra.Command, args []string) error {
	return runReviseWithDeps(cmd, args, nil)
}

func runReviseWithDeps(cmd *cobra.Command, args []string, deps *ProviderDeps) (err error) {
	out := cmd.OutOrStdout()
	slug, addr := args[0], args[1]
	partIdx, chIdx, err := parseChapterAddr(addr)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeUsage}
	}
	bc, err := loadBook(slug)
	if err != nil {
		return err
	}
	ch, err := findChapter(bc.Outline, partIdx, chIdx)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeUsage}
	}
	if ch.Status != book.StatusExpanded && ch.Status != book.StatusReviewed {
		return &InfoError{
			Err:  fmt.Errorf("chapter %s has status %q; only %q or %q chapters can be revised", addr, ch.Status, book.StatusExpanded, book.StatusReviewed),
			Code: ExitCodeUsage,
		}
	}

	// Read current chapter file.
	chapPath := book.ChapterPath(bc.BookDir, partIdx, chIdx)
	_, body, err := book.ReadChapter(chapPath)
	if err != nil {
		return &InfoError{Err: fmt.Errorf("read chapter: %w", err), Code: ExitCodeGeneric}
	}

	// Build providers.
	if deps == nil {
		ws, err := workspace.Load(bc.WSRoot)
		if err != nil {
			return &InfoError{Err: err, Code: ExitCodeGeneric}
		}
		secrets, err := config.LoadSecrets()
		if err != nil {
			return &InfoError{Err: fmt.Errorf("load secrets: %w", err), Code: ExitCodeLLMProvider}
		}
		deps, err = buildProviderDeps(ws.Config, secrets)
		if err != nil {
			return &InfoError{Err: err, Code: ExitCodeLLMProvider}
		}

	}
	tracker := &engine.TokenTracker{}
	tracked := *deps
	tracked.Chatter = engine.NewTrackingChatter(deps.Chatter, tracker)
	deps = &tracked
	defer func() {
		err = errors.Join(err, persistTokenUsage(bc.BookDir, bc.Meta, tracker))
		if show, _ := cmd.Flags().GetBool("tokens"); show {
			printTokenUsage(out, tracker.Snapshot())
		}
	}()

	fmt.Fprintf(out, "Revising %s/%s...\n", slug, addr)

	result, err := revise.Run(cmd.Context(), deps.Chatter, revise.Input{
		ChapterTitle: ch.Title,
		Markdown:     body,
		Citations:    ch.Citations,
		Unverified:   ch.Claims,
		Verdicts:     ch.Verdicts,
	})
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	if strings.TrimSpace(result.RevisedMarkdown) == "" {
		return &InfoError{Err: fmt.Errorf("revision returned empty prose"), Code: ExitCodeGeneric}
	}

	validated, err := expand.RunValidate(cmd.Context(), deps.Chatter, result.RevisedMarkdown, expand.ResearchNotes{}, "", nil)
	if err != nil {
		return &InfoError{Err: fmt.Errorf("validate revision: %w", err), Code: ExitCodeGeneric}
	}
	if validated.RevisedMarkdown != "" {
		result.RevisedMarkdown = validated.RevisedMarkdown
	}
	if strings.TrimSpace(result.RevisedMarkdown) == "" {
		return &InfoError{Err: fmt.Errorf("validation returned empty prose"), Code: ExitCodeGeneric}
	}

	err = withBookRollback(bc, partIdx, chIdx, func() error {
		ch.Claims = nil
		ch.UnverifiedClaims = 0
		for _, c := range validated.Claims {
			ch.Claims = append(ch.Claims, book.Claim{Text: c.Text, HasCitation: c.HasCitation, CitationIDs: c.CitationIDs})
			if !c.HasCitation {
				ch.UnverifiedClaims++
			}
		}
		oldCitations := make(map[string]book.Citation)
		for _, c := range ch.Citations {
			oldCitations[c.ID] = c
		}
		ch.Citations = nil
		defs := expand.ParseFootnotes(result.RevisedMarkdown)
		ids := make([]string, 0, len(defs))
		for id := range defs {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			d := defs[id]
			c := oldCitations[id]
			if c.URL != d.URL {
				c = book.Citation{}
			}
			c.ID, c.URL, c.Title = id, d.URL, d.Title
			ch.Citations = append(ch.Citations, c)
		}
		ch.CitationsCount = len(ch.Citations)
		ch.Status = book.StatusExpanded
		ch.ReviewedAt = nil
		ch.ReviewedBy = ""
		ch.Verdicts = nil

		// Write revised chapter file with updated frontmatter.
		// Preserve existing metadata (Model, EngineVersion, Citations, etc.)
		// and only update fields that change.
		existingFM, _, readErr := book.ReadChapter(chapPath)
		var fm book.ChapterFrontmatter
		if readErr == nil && existingFM != nil {
			fm = *existingFM
		}
		fm.Title = ch.Title
		fm.PartIndex = partIdx
		fm.ChapterIndex = chIdx
		fm.Status = ch.Status
		oldFrontmatterCitations := make(map[string]book.ChapterCitation)
		for _, c := range fm.Citations {
			oldFrontmatterCitations[c.ID] = c
		}
		fm.Citations = nil
		for _, c := range ch.Citations {
			entry := oldFrontmatterCitations[c.ID]
			if entry.URL != c.URL {
				entry = book.ChapterCitation{}
			}
			entry.ID, entry.URL, entry.Title = c.ID, c.URL, c.Title
			fm.Citations = append(fm.Citations, entry)
		}
		fm.UnverifiedClaimsCount = ch.UnverifiedClaims
		fm.WordCount = roughWordCount(result.RevisedMarkdown)
		fm.GeneratedAt = time.Now().UTC()
		if _, err := book.WriteChapter(bc.BookDir, partIdx, chIdx, fm, result.RevisedMarkdown); err != nil {
			return &InfoError{Err: err, Code: ExitCodeGeneric}
		}

		// Update outline.
		ch.WordCount = fm.WordCount
		if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
			return &InfoError{Err: err, Code: ExitCodeGeneric}
		}

		bc.Meta.Status = book.BookStatusDraft
		bc.Meta.UpdatedAt = time.Now().UTC()
		if err := book.SaveMeta(filepath.Join(bc.BookDir, "meta.json"), bc.Meta); err != nil {
			return &InfoError{Err: err, Code: ExitCodeGeneric}
		}
		return nil

	})
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "✓ Revised %s/%s (%d words)\n", slug, addr, ch.WordCount)
	fmt.Fprintf(out, "Next: jianwu factcheck %s %s, then review the revised chapter.\n", slug, addr)
	return nil
}

// roughWordCount provides an approximate word count for Chinese + English text.
func roughWordCount(s string) int {
	words := strings.Fields(s)
	count := len(words)
	for _, w := range words {
		for _, r := range w {
			if r >= 0x4E00 && r <= 0x9FFF { // CJK
				count++ // count each CJK char as an additional word
			}
		}
	}
	return count
}
