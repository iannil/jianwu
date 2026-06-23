// internal/cli/review.go
package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/zhurong/jianwu/internal/book"
)

func newReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review <slug> <NN-MM>",
		Short: "Mark one expanded chapter as reviewed (human-approved)",
		Long: `After reading an expanded chapter, mark it reviewed.
Only chapters in status "expanded" can be reviewed (strict state machine).
Updates outline.json and mirrors the status into the chapter .md frontmatter.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReview(cmd, args)
		},
	}
}

func runReview(cmd *cobra.Command, args []string) error {
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
	if ch.Status != book.StatusExpanded {
		return &InfoError{
			Err:  fmt.Errorf("chapter %s has status %q; only %q chapters can be reviewed", addr, ch.Status, book.StatusExpanded),
			Code: ExitCodeUsage,
		}
	}

	now := time.Now().UTC()
	ch.Status = book.StatusReviewed
	ch.ReviewedAt = &now
	ch.ReviewedBy = osUsername()

	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}
	if err := mirrorChapterStatus(bc.BookDir, partIdx, chIdx, book.StatusReviewed); err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	fmt.Fprintf(out, "✓ Reviewed %s/%s by %s\n", slug, addr, ch.ReviewedBy)
	return nil
}
