// internal/cli/status.go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zhurong/jianwu/internal/book"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <slug>",
		Short: "Show a book's chapter-by-chapter progress",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, args)
		},
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	slug := args[0]
	bc, err := loadBook(slug)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "%s (%s)\n", bc.Meta.Title, slug)
	fmt.Fprintf(out, "Book status: %s\n\n", bc.Meta.Status)

	counts := map[string]int{}
	total := 0
	for pi := range bc.Outline.Parts {
		p := bc.Outline.Parts[pi]
		fmt.Fprintf(out, "Part %d: %s\n", p.Index, p.Title)
		for ci := range p.Chapters {
			c := p.Chapters[ci]
			counts[c.Status]++
			total++
			line := fmt.Sprintf("  %02d-%02d  %-24s [%s]", p.Index, c.Index, c.Title, c.Status)
			if c.Status == book.StatusExpanded || c.Status == book.StatusReviewed || c.Status == book.StatusFinal {
				line += fmt.Sprintf("  %d words", c.WordCount)
				if c.UnverifiedClaims > 0 {
					line += fmt.Sprintf(", %d unverified", c.UnverifiedClaims)
				}
			}
			fmt.Fprintln(out, line)
		}
	}

	fmt.Fprintf(out, "\nSummary: scaffolded %d / expanded %d / reviewed %d / final %d / failed %d (total %d)\n",
		counts[book.StatusScaffolded], counts[book.StatusExpanded],
		counts[book.StatusReviewed], counts[book.StatusFinal], counts[book.StatusFailed], total)

	switch {
	case counts[book.StatusFailed] > 0:
		fmt.Fprintf(out, "Next: re-run expand on %d failed chapter(s) — `jianwu expand %s <NN-MM>`\n", counts[book.StatusFailed], slug)
	case counts[book.StatusExpanded] > 0:
		fmt.Fprintf(out, "Next: review %d expanded chapter(s) — `jianwu review %s <NN-MM>`\n", counts[book.StatusExpanded], slug)
	case total > 0 && counts[book.StatusReviewed] == total:
		fmt.Fprintf(out, "Next: all chapters reviewed — `jianwu finalize %s`\n", slug)
	case bc.Meta.Status == book.BookStatusFinal:
		fmt.Fprintf(out, "Next: book is final — `jianwu export %s`\n", slug)
	}
	return nil
}
