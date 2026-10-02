// internal/cli/skill.go
package cli

import (
	"fmt"

	"github.com/iannil/jianwu/internal/skill"
	"github.com/spf13/cobra"
)

func newSkillCmd() *cobra.Command {
	var dir string
	var stdout bool
	cmd := &cobra.Command{
		Use:   "skill [--dir <agent-skills-root>] [--stdout]",
		Short: "Install the jianwu agent skill (SKILL.md)",
		Long: `Write the embedded agent skill to <dir>/jianwu/SKILL.md (default
~/.agents/skills), teaching AI agents the jianwu workflow, the four access
modes (MCP / API / RSS / CLI) and the human-gate rules. Use --stdout to
print the skill for redirection into any agent's skill directory.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if stdout {
				_, _ = cmd.OutOrStdout().Write(skill.Content())
				return nil
			}
			path, err := skill.Install(dir)
			if err != nil {
				return &InfoError{Err: err, Code: ExitCodeGeneric}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Installed agent skill: %s\n", path)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "agent skills root (default ~/.agents/skills)")
	cmd.Flags().BoolVar(&stdout, "stdout", false, "print the skill instead of installing")
	return cmd
}
