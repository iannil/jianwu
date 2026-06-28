// internal/cli/move_chapter.go
package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/storage"
	"github.com/spf13/cobra"
)

func newMoveChapterCmd() *cobra.Command {
	var afterAddr string

	cmd := &cobra.Command{
		Use:   "move-chapter <slug> <NN-MM> <target-part>",
		Short: "Move a chapter to a different part",
		Long: `Move a chapter from its current part to another part.

By default the chapter keeps its chapter index (only the part changes).
Use --after to specify an exact insertion position in the target part.

Examples:
  jianwu move-chapter my-book 01-03 2
  jianwu move-chapter my-book 01-03 2 --after 02-01`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMoveChapter(cmd, args[0], args[1], args[2], afterAddr)
		},
	}
	cmd.Flags().StringVar(&afterAddr, "after", "", "insert after this chapter in target part (e.g. 02-01)")
	return cmd
}

func runMoveChapter(cmd *cobra.Command, slug, addr, targetPartStr, afterAddr string) error {
	out := cmd.OutOrStdout()

	partIdx, chIdx, err := parseChapterAddr(addr)
	if err != nil {
		return &InfoError{Err: fmt.Errorf("chapter address: %w", err), Code: ExitCodeUsage}
	}

	targetPart := 0
	if _, err := fmt.Sscanf(targetPartStr, "%d", &targetPart); err != nil || targetPart < 1 {
		return &InfoError{Err: fmt.Errorf("invalid target part %q", targetPartStr), Code: ExitCodeUsage}
	}

	bc, err := loadBook(slug)
	if err != nil {
		return err
	}

	// Find source chapter and remove from source part.
	sourcePart := findPartByIndex(bc.Outline, partIdx)
	if sourcePart == nil {
		return &InfoError{Err: fmt.Errorf("source part %d not found", partIdx), Code: ExitCodeUsage}
	}

	var chapter *book.OutlineChapter
	found := -1
	for i := range sourcePart.Chapters {
		if sourcePart.Chapters[i].Index == chIdx {
			chapter = &sourcePart.Chapters[i]
			found = i
			break
		}
	}
	if found < 0 {
		return &InfoError{Err: fmt.Errorf("chapter %s not found", addr), Code: ExitCodeUsage}
	}

	// Deep copy the chapter before removing from source.
	chCopy := *chapter
	sourcePart.Chapters = append(sourcePart.Chapters[:found], sourcePart.Chapters[found+1:]...)

	// Find (or create) target part.
	targetP := findPartByIndex(bc.Outline, targetPart)
	if targetP == nil {
		return &InfoError{Err: fmt.Errorf("target part %d not found", targetPart), Code: ExitCodeUsage}
	}

	// Determine new chapter index and insertion position.
	newChIdx := chCopy.Index // default: keep index
	insertPos := len(targetP.Chapters) // default: append to end
	if afterAddr != "" {
		ap, ac, err := parseChapterAddr(afterAddr)
		if err != nil {
			return &InfoError{Err: fmt.Errorf("--after: %w", err), Code: ExitCodeUsage}
		}
		if ap != targetPart {
			return &InfoError{Err: fmt.Errorf("--after part %d must match target part %d", ap, targetPart), Code: ExitCodeUsage}
		}
		newChIdx = ac + 1
		// Find position of after chapter in target.
		insertPos = -1
		for i, c := range targetP.Chapters {
			if c.Index == ac {
				insertPos = i + 1
				break
			}
		}
		if insertPos < 0 {
			return &InfoError{Err: fmt.Errorf("--after chapter %s not found in target part", afterAddr), Code: ExitCodeUsage}
		}
	}

	// Check conflict in target part.
	for _, c := range targetP.Chapters {
		if c.Index == newChIdx {
			return &InfoError{
				Err:  fmt.Errorf("target part %d already has chapter %02d-%02d; use --after to specify a different index", targetPart, targetPart, newChIdx),
				Code: ExitCodeUsage,
			}
		}
	}

	// Update chapter metadata.
	chCopy.Index = newChIdx

	// Insert into target part.
	targetP.Chapters = append(targetP.Chapters, book.OutlineChapter{})
	copy(targetP.Chapters[insertPos+1:], targetP.Chapters[insertPos:])
	targetP.Chapters[insertPos] = chCopy

	// Move chapter file: read old, write new, delete old.
	oldPath := book.ChapterPath(bc.BookDir, partIdx, chIdx)
	newPath := book.ChapterPath(bc.BookDir, targetPart, newChIdx)

	fm, body, rerr := book.ReadChapter(oldPath)
	if rerr == nil {
		// Update frontmatter with new part/chapter index.
		fm.PartIndex = targetPart
		fm.ChapterIndex = newChIdx
		fm.GeneratedAt = time.Now().UTC()
		if _, err := book.WriteChapter(bc.BookDir, targetPart, newChIdx, *fm, body); err != nil {
			return &InfoError{Err: fmt.Errorf("write moved chapter: %w", err), Code: ExitCodeGeneric}
		}
		// Remove old file.
		if err := storage.OS.RemoveAll(oldPath); err != nil {
			fmt.Fprintf(out, "warning: could not delete old chapter file %s: %v\n", oldPath, err)
		}
	} else {
		// No existing file — just write a stub.
		stub := book.ChapterFrontmatter{
			Title:         chCopy.Title,
			PartIndex:     targetPart,
			ChapterIndex:  newChIdx,
			Status:        chCopy.Status,
			GeneratedAt:   time.Now().UTC(),
			EngineVersion: Version,
		}
		if _, err := book.WriteChapter(bc.BookDir, targetPart, newChIdx, stub, ""); err != nil {
			return &InfoError{Err: fmt.Errorf("write stub: %w", err), Code: ExitCodeGeneric}
		}
	}

	// Save outline.
	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		return &InfoError{Err: fmt.Errorf("save outline: %w", err), Code: ExitCodeGeneric}
	}

	fmt.Fprintf(out, "✓ Moved %s → part %d (as %02d-%02d)\n", addr, targetPart, targetPart, newChIdx)
	fmt.Fprintf(out, "  File: %s\n", newPath)
	return nil
}
