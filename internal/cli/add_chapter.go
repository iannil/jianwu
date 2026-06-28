// internal/cli/add_chapter.go
package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/iannil/jianwu/internal/book"
)

func newAddChapterCmd() *cobra.Command {
	var afterAddr string
	var topic string
	var asAddr string

	cmd := &cobra.Command{
		Use:   "add-chapter <slug>",
		Short: "Add a new chapter to an existing book",
		Long: `Insert a new scaffolded chapter after an existing chapter.

The new chapter's index defaults to (after chapter's index + 1). If that
index is already taken, use --as to specify a different address.

Examples:
  jianwu add-chapter my-book --after 01-02 --topic "新的主题"
  jianwu add-chapter my-book --after 02-01 --topic "插入章节" --as 02-05`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddChapter(cmd, args[0], afterAddr, topic, asAddr)
		},
	}
	cmd.Flags().StringVar(&afterAddr, "after", "", "insert after this chapter address (e.g. 01-02)")
	cmd.Flags().StringVar(&topic, "topic", "", "title for the new chapter")
	cmd.Flags().StringVar(&asAddr, "as", "", "explicit chapter address (e.g. 01-05)")
	return cmd
}

func runAddChapter(cmd *cobra.Command, slug, afterAddr, topic, asAddr string) error {
	out := cmd.OutOrStdout()

	if afterAddr == "" {
		return &InfoError{Err: fmt.Errorf("--after is required"), Code: ExitCodeUsage}
	}
	if topic == "" {
		return &InfoError{Err: fmt.Errorf("--topic is required"), Code: ExitCodeUsage}
	}

	// Parse the "after" address.
	afterPart, afterCh, err := parseChapterAddr(afterAddr)
	if err != nil {
		return &InfoError{Err: fmt.Errorf("--after: %w", err), Code: ExitCodeUsage}
	}

	// Determine new chapter address.
	newPart := afterPart
	newCh := afterCh + 1
	if asAddr != "" {
		np, nc, err := parseChapterAddr(asAddr)
		if err != nil {
			return &InfoError{Err: fmt.Errorf("--as: %w", err), Code: ExitCodeUsage}
		}
		if np != afterPart {
			return &InfoError{Err: fmt.Errorf("--as part %d must match --after part %d", np, afterPart), Code: ExitCodeUsage}
		}
		if nc <= afterCh {
			return &InfoError{Err: fmt.Errorf("--as chapter %d must be greater than --after chapter %d", nc, afterCh), Code: ExitCodeUsage}
		}
		newPart = np
		newCh = nc
	}

	// Load book.
	bc, err := loadBook(slug)
	if err != nil {
		return err
	}

	// Find the "after" chapter in outline to locate insertion point.
	afterChapter := findChapterByExact(bc.Outline, afterPart, afterCh)
	if afterChapter == nil {
		return &InfoError{
			Err:  fmt.Errorf("chapter %s not found in outline", afterAddr),
			Code: ExitCodeUsage,
		}
	}

	// Check conflict: does a chapter with new index already exist?
	if findChapterByExact(bc.Outline, newPart, newCh) != nil {
		return &InfoError{
			Err:  fmt.Errorf("chapter %02d-%02d already exists; use --as to specify a different address", newPart, newCh),
			Code: ExitCodeUsage,
		}
	}

	// Build new chapter.
	newChapter := book.OutlineChapter{
		Index: newCh,
		Title: topic,
		// Abstract and KeyConcepts left empty — user can expand to fill them.
		Status: book.StatusScaffolded,
	}

	// Insert into outline at the correct position.
	part := findPartByIndex(bc.Outline, afterPart)
	if part == nil {
		return &InfoError{
			Err:  fmt.Errorf("part %d not found in outline", afterPart),
			Code: ExitCodeGeneric,
		}
	}

	insertIdx := -1
	for i, c := range part.Chapters {
		if c.Index == afterCh {
			insertIdx = i + 1
			break
		}
	}
	if insertIdx < 0 {
		return &InfoError{
			Err:  fmt.Errorf("internal: after chapter %s not found in part slice", afterAddr),
			Code: ExitCodeGeneric,
		}
	}

	// Insert the new chapter into the slice.
	part.Chapters = append(part.Chapters, book.OutlineChapter{})
	copy(part.Chapters[insertIdx+1:], part.Chapters[insertIdx:])
	part.Chapters[insertIdx] = newChapter

	// Save outline.
	outlinePath := filepath.Join(bc.BookDir, "outline.json")
	if err := book.SaveOutline(outlinePath, bc.Outline); err != nil {
		return &InfoError{Err: fmt.Errorf("save outline: %w", err), Code: ExitCodeGeneric}
	}

	// Write stub chapter file.
	fm := book.ChapterFrontmatter{
		Title:        topic,
		PartIndex:    newPart,
		ChapterIndex: newCh,
		Status:       book.StatusScaffolded,
		WordCount:    0,
		GeneratedAt:  time.Now().UTC(),
		Model:        "",
		EngineVersion: Version,
	}
	chapPath, err := book.WriteChapter(bc.BookDir, newPart, newCh, fm, "")
	if err != nil {
		return &InfoError{Err: fmt.Errorf("write chapter stub: %w", err), Code: ExitCodeGeneric}
	}

	fmt.Fprintf(out, "✓ Added chapter %02d-%02d: %s\n", newPart, newCh, topic)
	fmt.Fprintf(out, "  File: %s\n", chapPath)
	fmt.Fprintf(out, "  Status: %s (run `jianwu expand %s %02d-%02d` to generate content)\n",
		book.StatusScaffolded, slug, newPart, newCh)
	return nil
}

// findChapterByExact returns the chapter at (partIdx, chIdx) or nil.
func findChapterByExact(outline *book.Outline, partIdx, chIdx int) *book.OutlineChapter {
	for i := range outline.Parts {
		if outline.Parts[i].Index == partIdx {
			for j := range outline.Parts[i].Chapters {
				if outline.Parts[i].Chapters[j].Index == chIdx {
					return &outline.Parts[i].Chapters[j]
				}
			}
		}
	}
	return nil
}

// findPartByIndex returns the part with the given index, or nil.
func findPartByIndex(outline *book.Outline, partIdx int) *book.OutlinePart {
	for i := range outline.Parts {
		if outline.Parts[i].Index == partIdx {
			return &outline.Parts[i]
		}
	}
	return nil
}
