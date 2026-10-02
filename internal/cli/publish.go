// internal/cli/publish.go
package cli

import (
	"fmt"
	"io"

	"github.com/iannil/jianwu/internal/release"
	"github.com/iannil/jianwu/internal/storage"
	"github.com/spf13/cobra"
)

func newPublishCmd() *cobra.Command {
	var dryRun, forceMajor bool
	var version string
	cmd := &cobra.Command{
		Use:   "publish <slug>",
		Short: "Publish an immutable versioned release of a finalized book",
		Long: `Write books/<slug>/releases/<MAJOR.MINOR>/ with manifest, provenance,
state snapshots and chapter copies (ADR 29 publishing layer).

Hard gate: book and all chapters final, meta.json license set, target version free.
Structural changes (part or per-part chapter counts) bump the major version,
content edits bump the minor version. Warnings (unverified claims, failed
verdicts) are disclosed in the manifest, never hidden.
Publish never modifies book state; existing versions are immutable.
Use --dry-run to report the gate, next version and planned files.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPublish(cmd, args[0], release.Options{
				Version: version, ForceMajor: forceMajor, DryRun: dryRun, JianwuVersion: Version,
			})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report gate + next version without writing")
	cmd.Flags().BoolVar(&forceMajor, "major", false, "force a major version bump")
	cmd.Flags().StringVar(&version, "version", "", "explicit MAJOR.MINOR (must exceed existing versions)")
	return cmd
}

func runPublish(cmd *cobra.Command, slug string, opts release.Options) error {
	out := cmd.OutOrStdout()
	bc, err := loadBook(slug)
	if err != nil {
		return err
	}
	// Wire the EPUB artifact builder now that the export slice exists; the
	// release layer stays decoupled via the injected hook (ADR 29).
	opts.BuildEPUB = func() ([]byte, error) { return buildEPUBArtifact(bc) }
	res, err := release.Publish(storage.OS, release.Input{BookDir: bc.BookDir, Meta: bc.Meta, Outline: bc.Outline}, opts)
	if res != nil {
		printPublishReport(out, res)
	}
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}
	if res.DryRun {
		fmt.Fprintf(out, "[dry-run] next version %s, %d file(s) planned, nothing written\n", res.Version, len(res.Files))
	} else {
		fmt.Fprintf(out, "✓ Published %s -> %s (%d file(s))\n", res.Version, res.Dir, len(res.Files))
	}
	return nil
}

// printPublishReport shows blockers/warnings and the stats line. Gate messages
// are Chinese to match the serve UI; the CLI frame around them stays English.
func printPublishReport(out io.Writer, res *release.Result) {
	for _, b := range res.Report.Blockers {
		fmt.Fprintf(out, "✗ %s\n", b)
	}
	for _, w := range res.Report.Warnings {
		fmt.Fprintf(out, "! %s\n", w)
	}
	st := res.Stats
	fmt.Fprintf(out, "  %d chapter(s), %d claim(s) (%d unverified), %d citation(s), %d failed verdict(s)\n",
		st.ChaptersTotal, st.ClaimsTotal, st.ClaimsUnverified, st.CitationsTotal, st.VerdictsFailed)
}
