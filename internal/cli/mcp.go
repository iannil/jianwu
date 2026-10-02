// internal/cli/mcp.go
package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/iannil/jianwu/internal/server"
	"github.com/iannil/jianwu/internal/workspace"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run jianwu as an MCP stdio server for AI agents",
		Long: `Run the jianwu MCP server over stdio (ADR 30). Tools cover the full
pipeline: create_book, list_books, book_status, read_chapter, expand_chapter,
expand_all, factcheck, revise, review_chapter, finalize, export_book,
publish, list_releases, generate_site and job_status. Long operations run as
background jobs — poll job_status with the returned job_id. The same
single-writer-per-book constraint as the CLI and serve applies.

Human gates are unchanged: review requires explicit operator attribution and
the publish hard gate (final + license) is not relaxed for agents. Register
with your agent runtime, e.g. "jianwu --dir <workspace> mcp".

stdout belongs to the protocol; diagnostics go to stderr.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			startPath, _ := workspace.ResolveRoot(cliWorkspaceDir)
			wsRoot, err := workspace.FindWorkspace(startPath)
			if err != nil {
				return &InfoError{Err: err, Code: ExitCodeWorkspaceNotFound}
			}
			return server.RunMCPServer(context.Background(), wsRoot, Version)
		},
	}
	return cmd
}
