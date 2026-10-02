// internal/cli/site.go
package cli

import (
	"fmt"
	"path/filepath"

	"github.com/iannil/jianwu/internal/site"
	"github.com/iannil/jianwu/internal/workspace"
	"github.com/spf13/cobra"
)

func newSiteCmd() *cobra.Command {
	var out, base string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "site [--out <dir>] [--dry-run]",
		Short: "Generate the static reading site from published releases",
		Long: `Rebuild <workspace>/site (or --out dir) from every published release:
catalog page, book pages, per-chapter reading pages with the source
verification sections, EPUB downloads and an OPDS acquisition feed.

The site reads releases only — working drafts never appear. The directory is
fully derived state: it is wiped and regenerated on every run. Deterministic
for a given shelf state. Deploy the output to any static host.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSite(cmd, out, base, dryRun)
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "output directory (default <workspace>/site)")
	cmd.Flags().StringVar(&base, "base", "", "base URL for absolute RSS/OPDS links (e.g. https://books.example.com)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "list the shelf without writing")
	return cmd
}

func runSite(cmd *cobra.Command, out, base string, dryRun bool) error {
	o := cmd.OutOrStdout()
	wsRoot, err := workspace.FindWorkspace(findWorkspacePath())
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeWorkspaceNotFound}
	}
	if out == "" {
		out = filepath.Join(wsRoot, "site")
	}
	if dryRun {
		books, skipped, err := site.Scan(wsRoot)
		if err != nil {
			return &InfoError{Err: err, Code: ExitCodeGeneric}
		}
		for i := range books {
			b := &books[i]
			fmt.Fprintf(o, "[dry-run] %s v%s (%s) — %d chapter(s)\n",
				b.Slug, b.Manifest.Version, b.Manifest.CreatedAt.Format("2006-01-02"), b.Manifest.Content.ChaptersTotal)
		}
		for _, s := range skipped {
			fmt.Fprintf(o, "! skipped %s\n", s)
		}
		fmt.Fprintf(o, "[dry-run] would write %s (%d book(s)), nothing written\n", out, len(books))
		return nil
	}
	res, err := site.GenerateAt(wsRoot, out, base)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}
	for _, s := range res.Skipped {
		fmt.Fprintf(o, "! skipped %s\n", s)
	}
	fmt.Fprintf(o, "✓ Generated %s (%d book(s), %d page(s), %d epub artifact(s))\n", res.Dir, res.Books, res.Pages, res.EPUBs)
	return nil
}
