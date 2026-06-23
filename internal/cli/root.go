package cli

import (
	"fmt"

	"github.com/spf13/cobra"
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
		Short: "Structure AI's training knowledge into human-readable books.",
		Long: `jianwu (简物) - Library + CLI for turning AI's training knowledge
into human-readable, well-structured books.`,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.PersistentFlags().BoolVarP(&gf.Verbose, "verbose", "v", false, "verbose output (INFO level logs)")
	cmd.PersistentFlags().BoolVar(&gf.Debug, "debug", false, "debug output (DEBUG level + LLM request/response dump)")
	cmd.PersistentFlags().Bool("version", false, "print version and exit")

	// Override Run to handle --version
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if v, _ := c.Flags().GetBool("version"); v {
			fmt.Fprintf(c.OutOrStdout(), "jianwu %s\n", Version)
			return nil
		}
		return c.Help()
	}

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newInfoCmd())
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newNewCmd())
	cmd.AddCommand(newExpandCmd())
	cmd.AddCommand(newReviewCmd())
	cmd.AddCommand(newFinalizeCmd())
	cmd.AddCommand(newExportCmd())

	return cmd
}

// GlobalFlagsFrom returns the parsed global flags for the given command.
// (Used by subcommands to access verbose/debug.)
func GlobalFlagsFrom(cmd *cobra.Command) GlobalFlags {
	v, _ := cmd.Flags().GetBool("verbose")
	d, _ := cmd.Flags().GetBool("debug")
	return GlobalFlags{Verbose: v, Debug: d}
}
