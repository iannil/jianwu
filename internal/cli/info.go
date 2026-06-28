package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/workspace"
)

// InfoError wraps an error with a suggested exit code.
type InfoError struct {
	Err  error
	Code int
}

func (e *InfoError) Error() string { return e.Err.Error() }
func (e *InfoError) Unwrap() error { return e.Err }

func newInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show workspace status",
		RunE: func(cmd *cobra.Command, args []string) error {
			wsRoot, err := workspace.FindWorkspace(findWorkspacePath())
			if err != nil {
				return &InfoError{Err: err, Code: ExitCodeWorkspaceNotFound}
			}
			ws, err := workspace.Load(wsRoot)
			if err != nil {
				return err
			}
			secrets, secretsErr := config.LoadSecrets()
			if secretsErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not load secrets: %v\n", secretsErr)
				secrets = &config.Secrets{} // empty so printInfo doesn't nil-deref
			}
			printInfo(cmd, ws, secrets)
			return nil
		},
	}
	return cmd
}

func printInfo(cmd *cobra.Command, ws *workspace.Workspace, s *config.Secrets) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Workspace: %s\n", ws.Root)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Models:")
	fmt.Fprintf(out, "  intake:      %s/%s\n", ws.Config.Models.Intake.Provider, ws.Config.Models.Intake.Model)
	fmt.Fprintf(out, "  outline:     %s/%s\n", ws.Config.Models.Outline.Provider, ws.Config.Models.Outline.Model)
	fmt.Fprintf(out, "  scaffolding: %s/%s\n", ws.Config.Models.Scaffolding.Provider, ws.Config.Models.Scaffolding.Model)
	fmt.Fprintf(out, "  expand:      %s/%s\n", ws.Config.Models.Expand.Provider, ws.Config.Models.Expand.Model)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Search:")
	fmt.Fprintf(out, "  primary:  %s\n", ws.Config.Search.Primary)
	fmt.Fprintf(out, "  fallback: %s\n", ws.Config.Search.Fallback)
	fmt.Fprintf(out, "  reader:   %s\n", ws.Config.Search.Reader)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "API keys (configured):")
	fmt.Fprintf(out, "  gemini: %s\n", secretStatus(s.GeminiAPIKey))
	fmt.Fprintf(out, "  glm:    %s\n", secretStatus(s.GLMAPIKey))
	fmt.Fprintf(out, "  brave:  %s\n", secretStatus(s.BraveAPIKey))
	fmt.Fprintf(out, "  serper: %s\n", secretStatus(s.SerperAPIKey))
	fmt.Fprintf(out, "  jina:   %s\n", secretStatus(s.JinaAPIKey))
}

func secretStatus(v string) string {
	if v == "" {
		return "missing"
	}
	return "ok"
}
