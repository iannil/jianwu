package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iannil/jianwu/internal/workspace"
)

func newInitCmd() *cobra.Command {
	var bare bool
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a jianwu workspace",
		Long: `Create a jianwu workspace at the given path (defaults to current directory).

A workspace is a directory containing a .jianwu/ marker. By default, init also
creates books/, exports/, and archive/ subdirectories. Use --bare to skip those
when initializing inside an existing project.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}
			opts := workspace.InitOpts{Bare: bare}
			if err := workspace.Init(path, opts); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized jianwu workspace at %s\n", path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&bare, "bare", false, "skip books/exports/archive creation")
	return cmd
}
