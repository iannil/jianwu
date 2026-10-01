package cli

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// buildVersion renders the version string shown by `jianwu version` and the
// root command's --version flag. Values come from version.go / -ldflags; for
// unstamped dev builds it falls back to the VCS revision and commit time that
// the go tool stamps into the binary (go1.18+, -buildvcs). Builds from source
// without VCS context (e.g. tarballs) keep "unknown". The -dirty marker
// mirrors scripts/release.sh.
func buildVersion() string {
	commit, built := Commit, BuildTime
	if info, ok := debug.ReadBuildInfo(); ok {
		var rev, vcsTime string
		dirty := false
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.time":
				vcsTime = s.Value
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
		if commit == "unknown" && rev != "" {
			commit = rev
			if dirty {
				commit += "-dirty"
			}
		}
		if built == "unknown" && vcsTime != "" {
			built = vcsTime
		}
	}
	return fmt.Sprintf("%s (commit %s, built %s)", Version, commit, built)
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
