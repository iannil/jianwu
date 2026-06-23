package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/workspace"
)

func newNewCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Start a new book (interactive grill → outline → scaffolding)",
		Long: `Walk through the grill questionnaire interactively, then auto-generate
outline + scaffolding. If an incomplete grill session exists, prompts to resume.

Use --force to overwrite an existing book with the same slug.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			wsRoot, err := workspace.FindWorkspace(".")
			if err != nil {
				return &InfoError{Err: err, Code: ExitCodeWorkspaceNotFound}
			}
			ws, err := workspace.Load(wsRoot)
			if err != nil {
				return err
			}
			secrets, _ := config.LoadSecrets()
			prompt := NewTerminalPrompt(nil, cmd.OutOrStdout())

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "jianwu new — starting grill flow\n")
			fmt.Fprintf(out, "Workspace: %s\n", wsRoot)

			outline, err := runNewFlow(wsRoot, ws.Config, secrets, prompt, force)
			if err != nil {
				return err
			}
			// Summary
			fmt.Fprintf(out, "\n✓ Book created\n")
			fmt.Fprintf(out, "  Parts: %d\n", len(outline.Parts))
			totalCh := 0
			scaffolded := 0
			failed := 0
			for _, p := range outline.Parts {
				for _, c := range p.Chapters {
					totalCh++
					switch c.Status {
					case "scaffolded":
						scaffolded++
					case "failed":
						failed++
					}
				}
			}
			fmt.Fprintf(out, "  Chapters: %d (scaffolded: %d, failed: %d)\n", totalCh, scaffolded, failed)
			if failed > 0 {
				fmt.Fprintf(out, "\nRun `jianwu status <slug>` to see failed chapters.\n")
				fmt.Fprintf(out, "Run `jianwu scaffolding <slug> --retry-failed` to retry them.\n")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing book with same slug")
	return cmd
}
