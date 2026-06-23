// internal/cli/finalize.go
package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/iannil/jianwu/internal/book"
	"github.com/spf13/cobra"
)

func newFinalizeCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "finalize <slug>",
		Short: "Finalize a book once all chapters are reviewed",
		Long: `Transition every chapter reviewed -> final and set the book status to final.
Requires ALL chapters to be in status "reviewed"; otherwise lists the blockers and aborts.
Use --dry-run to preview without writing.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFinalize(cmd, args, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate and report without writing")
	return cmd
}

func runFinalize(cmd *cobra.Command, args []string, dryRun bool) error {
	out := cmd.OutOrStdout()
	slug := args[0]
	bc, err := loadBook(slug)
	if err != nil {
		return err
	}
	if bc.Meta.Status == book.BookStatusFinal {
		return &InfoError{Err: fmt.Errorf("book %q is already final", slug), Code: ExitCodeGeneric}
	}

	var total int
	var blockers []string
	for pi := range bc.Outline.Parts {
		for ci := range bc.Outline.Parts[pi].Chapters {
			total++
			c := bc.Outline.Parts[pi].Chapters[ci]
			if c.Status != book.StatusReviewed {
				blockers = append(blockers, fmt.Sprintf("  %02d-%02d %q (status: %s)",
					bc.Outline.Parts[pi].Index, c.Index, c.Title, c.Status))
			}
		}
	}
	if total == 0 {
		return &InfoError{Err: fmt.Errorf("book %q has no chapters to finalize", slug), Code: ExitCodeGeneric}
	}
	if len(blockers) > 0 {
		return &InfoError{
			Err:  fmt.Errorf("cannot finalize %q: %d chapter(s) not reviewed:\n%s", slug, len(blockers), strings.Join(blockers, "\n")),
			Code: ExitCodeGeneric,
		}
	}

	if dryRun {
		fmt.Fprintf(out, "[dry-run] would finalize %d chapter(s) (reviewed -> final) and set book %q to final\n", total, slug)
		return nil
	}

	for pi := range bc.Outline.Parts {
		for ci := range bc.Outline.Parts[pi].Chapters {
			bc.Outline.Parts[pi].Chapters[ci].Status = book.StatusFinal
			if err := mirrorChapterStatus(bc.BookDir, bc.Outline.Parts[pi].Index, bc.Outline.Parts[pi].Chapters[ci].Index, book.StatusFinal); err != nil {
				return &InfoError{Err: err, Code: ExitCodeGeneric}
			}
		}
	}
	bc.Meta.Status = book.BookStatusFinal
	bc.Meta.UpdatedAt = time.Now().UTC()
	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}
	if err := book.SaveMeta(filepath.Join(bc.BookDir, "meta.json"), bc.Meta); err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	fmt.Fprintf(out, "✓ Finalized %q (%d chapters)\n", slug, total)
	return nil
}
