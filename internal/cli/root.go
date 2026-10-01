package cli

import (
	"github.com/spf13/cobra"

	"github.com/iannil/jianwu/internal/workspace"
)

// Exit code constants. Mirrors DESIGN.md §16 decision A1.
const (
	ExitCodeSuccess           = 0
	ExitCodeGeneric           = 1
	ExitCodeUsage             = 2
	ExitCodeWorkspaceNotFound = 3
	ExitCodeLLMProvider       = 4
	ExitCodeNetwork           = 5
)

// cliWorkspaceDir is set by the --dir flag to override the default CWD.
var cliWorkspaceDir string

// findWorkspacePath returns the workspace start path by precedence:
// --dir flag > JIANWU_WORKSPACE env > global config `workspace:` key > CWD.
func findWorkspacePath() string {
	if cliWorkspaceDir != "" {
		return cliWorkspaceDir
	}
	root, _ := workspace.ResolveRoot("")
	return root
}

// GlobalFlags holds root-level flag values.
type GlobalFlags struct {
	Verbose bool
	Debug   bool
}

// NewRootCmd builds the root cobra command.
func NewRootCmd() *cobra.Command {
	gf := &GlobalFlags{}
	cmd := &cobra.Command{
		Use:   "jianwu",
		Short: "Create structured non-fiction books in your local workspace.",
		Long: `jianwu (肩吾) is an independent, local-first CLI for non-fiction books.
Design, draft, verify sources, review and export from your own workspace.`,
		Version:       buildVersion(),
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.PersistentFlags().BoolVarP(&gf.Verbose, "verbose", "L", false, "verbose output (INFO level logs)")
	cmd.PersistentFlags().BoolVar(&gf.Debug, "debug", false, "debug output (DEBUG level + LLM request/response dump)")
	cmd.PersistentFlags().StringVarP(&cliWorkspaceDir, "dir", "d", "", "workspace root directory (default: CWD)")
	cmd.SetVersionTemplate("jianwu {{.Version}}\n")
	cmd.AddCommand(newVersionCmd())

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newInfoCmd())
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newNewCmd())
	cmd.AddCommand(newScaffoldingCmd())
	cmd.AddCommand(newExpandCmd())
	cmd.AddCommand(newReviewCmd())
	cmd.AddCommand(newFinalizeCmd())
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newFactCheckCmd())
	cmd.AddCommand(newReviseCmd())
	cmd.AddCommand(newAddChapterCmd())
	cmd.AddCommand(newDeleteChapterCmd())
	cmd.AddCommand(newMoveChapterCmd())
	cmd.AddCommand(newRewriteCmd())
	cmd.AddCommand(newCorpusCmd())
	cmd.AddCommand(newServeCmd())

	return cmd
}

// GlobalFlagsFrom returns the parsed global flags for the given command.
// (Used by subcommands to access verbose/debug.)
func GlobalFlagsFrom(cmd *cobra.Command) GlobalFlags {
	v, _ := cmd.Flags().GetBool("verbose")
	d, _ := cmd.Flags().GetBool("debug")
	return GlobalFlags{Verbose: v, Debug: d}
}
