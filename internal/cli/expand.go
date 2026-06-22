package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newExpandCmd() *cobra.Command {
	var force bool
	var force2 bool // --force --force, for overriding reviewed/final
	cmd := &cobra.Command{
		Use:   "expand <slug> <NN-MM>",
		Short: "Expand one chapter into markdown with citations",
		Long: `Run the 3-iteration expand agent (research → draft → validate) on one chapter,
producing chapters/NN-MM.md with YAML frontmatter and [^N] footnote citations.
Updates outline.json with status, citations, word_count, unverified_claims.

Use --force to overwrite an already-expanded chapter.
Use --force twice to overwrite a reviewed or final chapter (requires confirmation).`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Stub — filled in Task 5.
			return fmt.Errorf("not implemented")
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing expanded chapter")
	cmd.Flags().BoolVar(&force2, "force2", false, "also overwrite reviewed/final chapters (use with --force)")
	return cmd
}

// parseChapterAddr parses a "NN-MM" string into (partIdx, chIdx), both 1-based.
// Accepts zero-padded ("01-01") and bare ("1-1") forms.
// Returns error if format is wrong or any index is 0.
func parseChapterAddr(s string) (int, int, error) {
	if s == "" {
		return 0, 0, fmt.Errorf("empty chapter address")
	}
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid chapter address %q: want NN-MM", s)
	}
	partIdx, err := strconv.Atoi(parts[0])
	if err != nil || partIdx < 1 {
		return 0, 0, fmt.Errorf("invalid part index %q", parts[0])
	}
	chIdx, err := strconv.Atoi(parts[1])
	if err != nil || chIdx < 1 {
		return 0, 0, fmt.Errorf("invalid chapter index %q", parts[1])
	}
	return partIdx, chIdx, nil
}
