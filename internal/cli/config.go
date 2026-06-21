package cli

import "github.com/spf13/cobra"

func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Read or write configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil // filled in Task 14
		},
	}
}
