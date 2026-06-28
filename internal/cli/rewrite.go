// internal/cli/rewrite.go
package cli

import (
	"github.com/spf13/cobra"
)

func newRewriteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rewrite <slug> <NN-MM>",
		Short: "Re-expand a chapter from scratch",
		Long: `Completely re-run the expand pipeline (research → draft → validate)
on an already-expanded chapter. Equivalent to 'expand --force --force'.

All citations, claims, and word counts are regenerated.
The chapter's status is set back to "expanded" after rewrite.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRewrite(cmd, args)
		},
	}
}

func runRewrite(cmd *cobra.Command, args []string) error {
	// rewrite == expand with --force --force (override any status).
	const forceCount = 2
	return runExpand(cmd, args, forceCount, nil, false)
}
