package cli

import "github.com/spf13/cobra"

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show workspace status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}
