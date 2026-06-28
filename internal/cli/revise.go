package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/engine/revise"
	"github.com/iannil/jianwu/internal/workspace"
	"github.com/spf13/cobra"
)

func newReviseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revise <slug> <NN-MM>",
		Short: "Revise a chapter based on fact-check results",
		Long: `Read the chapter markdown and fact-check verdicts, then ask the LLM to
revise claims that failed verification. Updates both the chapter .md file
and outline.json.

Only chapters with status "expanded" or "reviewed" can be revised.
Run 'jianwu factcheck' first to generate verdicts.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRevise(cmd, args)
		},
	}
}

func runRevise(cmd *cobra.Command, args []string) error {
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
	ws, err := workspace.Load(bc.WSRoot)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}
	secrets, err := config.LoadSecrets()
	if err != nil {
		return &InfoError{Err: fmt.Errorf("load secrets: %w", err), Code: ExitCodeLLMProvider}
	}
	deps, err := buildProviderDeps(ws.Config, secrets)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeLLMProvider}
	}

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
	fm.Status = ch.Status // preserve current status
	fm.WordCount = roughWordCount(result.RevisedMarkdown)
	fm.GeneratedAt = time.Now().UTC()
	if _, err := book.WriteChapter(bc.BookDir, partIdx, chIdx, fm, result.RevisedMarkdown); err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	// Update outline.
	ch.WordCount = fm.WordCount
	ch.UnverifiedClaims = 0 // reset after revision
	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	fmt.Fprintf(out, "✓ Revised %s/%s (%d words)\n", slug, addr, fm.WordCount)
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
