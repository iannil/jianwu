package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// buildVersion renders the version string shown by `jianwu version` and the
// root command's --version flag. Values come from version.go / -ldflags.
func buildVersion() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, BuildTime)
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and build information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "jianwu %s\n", buildVersion())
			return err
		},
	}
}
