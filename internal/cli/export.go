// internal/cli/export.go
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zhurong/jianwu/internal/book"
)

func newExportCmd() *cobra.Command {
	var target string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "export <slug>",
		Short: "Export a book to a single markdown file",
		Long: `Assemble all chapters into books/<slug>/export/<slug>.md.
Footnotes are renumbered globally across the book. Chapters without prose get a placeholder.
Exportable at any status; the output header notes whether the book is draft or final.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExport(cmd, args, target, dryRun)
		},
	}
	cmd.Flags().StringVar(&target, "target", "md", "export target (only \"md\" supported in v1.0.3)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be written without writing")
	return cmd
}

func runExport(cmd *cobra.Command, args []string, target string, dryRun bool) error {
	out := cmd.OutOrStdout()
	slug := args[0]
	if target != "md" {
		return &InfoError{Err: fmt.Errorf("unsupported export target %q; only \"md\" is supported", target), Code: ExitCodeUsage}
	}
	bc, err := loadBook(slug)
	if err != nil {
		return err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", bc.Meta.Title)
	if bc.Meta.Subtitle != "" {
		fmt.Fprintf(&b, "*%s*\n\n", bc.Meta.Subtitle)
	}
	fmt.Fprintf(&b, "> 状态：%s\n\n", bc.Meta.Status)

	counter := 1
	present, missing := 0, 0
	for pi := range bc.Outline.Parts {
		p := bc.Outline.Parts[pi]
		fmt.Fprintf(&b, "## %s\n\n", p.Title)
		if p.Intro != "" {
			fmt.Fprintf(&b, "%s\n\n", p.Intro)
		}
		for ci := range p.Chapters {
			c := p.Chapters[ci]
			fm, body, rerr := book.ReadChapter(book.ChapterPath(bc.BookDir, p.Index, c.Index))
			title := c.Title
			if rerr == nil && fm != nil && fm.Title != "" {
				title = fm.Title
			}
			fmt.Fprintf(&b, "### %s\n\n", title)
			if rerr != nil {
				b.WriteString("> （本章尚未展开）\n\n")
				missing++
				continue
			}
			renumbered, next := renumberFootnotes(body, counter)
			counter = next
			b.WriteString(renumbered)
			b.WriteString("\n\n")
			present++
		}
	}

	outPath := filepath.Join(bc.BookDir, "export", slug+".md")
	if dryRun {
		fmt.Fprintf(out, "[dry-run] would write %s (%d chapter(s) with prose, %d placeholder(s))\n", outPath, present, missing)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return &InfoError{Err: fmt.Errorf("mkdir export dir: %w", err), Code: ExitCodeGeneric}
	}
	if err := os.WriteFile(outPath, []byte(b.String()), 0o644); err != nil {
		return &InfoError{Err: fmt.Errorf("write export: %w", err), Code: ExitCodeGeneric}
	}
	fmt.Fprintf(out, "✓ Exported %s (%d chapters, %d placeholders)\n", outPath, present, missing)
	return nil
}
