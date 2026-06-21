package cli

import "github.com/spf13/cobra"

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a jianwu workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil // filled in Task 12
		},
	}
}
