// internal/cli/delete_chapter.go
package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/storage"
	"github.com/spf13/cobra"
)

func newDeleteChapterCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete-chapter <slug> <NN-MM>",
		Short: "Delete a chapter from a book",
		Long: `Remove a chapter from the outline and delete its .md file.
The chapter's index will not be reused by other chapters (no renumbering).
ClaimWhitelist entries from this chapter are preserved (no-op).

Examples:
  jianwu delete-chapter my-book 01-03
  jianwu delete-chapter my-book 02-01`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeleteChapter(cmd, args)
		},
	}
}

func runDeleteChapter(cmd *cobra.Command, args []string) error {
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

	// Find the part.
	part := findPartByIndex(bc.Outline, partIdx)
	if part == nil {
		return &InfoError{
			Err:  fmt.Errorf("part %d not found in outline", partIdx),
			Code: ExitCodeUsage,
		}
	}

	// Find and remove the chapter from the slice.
	found := false
	for i, c := range part.Chapters {
		if c.Index == chIdx {
			part.Chapters = append(part.Chapters[:i], part.Chapters[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return &InfoError{
			Err:  fmt.Errorf("chapter %s not found in outline", addr),
			Code: ExitCodeUsage,
		}
	}

	// Delete the .md chapter file (ignore error if file doesn't exist).
	chapPath := book.ChapterPath(bc.BookDir, partIdx, chIdx)
	if err := storage.OS.RemoveAll(chapPath); err != nil && !strings.Contains(err.Error(), "no such file") {
		fmt.Fprintf(out, "warning: could not delete chapter file %s: %v\n", chapPath, err)
	}

	// Save outline.
	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		return &InfoError{Err: fmt.Errorf("save outline: %w", err), Code: ExitCodeGeneric}
	}

	fmt.Fprintf(out, "✓ Deleted chapter %s from %s\n", addr, slug)
	return nil
}
