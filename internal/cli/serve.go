package cli

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iannil/jianwu/internal/server"
	"github.com/iannil/jianwu/internal/workspace"
)

func newServeCmd() *cobra.Command {
	var addr, token string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the local web UI + HTTP API for this workspace",
		Long: `Start a local web server exposing jianwu's full pipeline (interview →
outline → scaffolding → expand → factcheck → revise → review → finalize →
export) through a browser UI and a JSON API under /api/v1.

The server binds localhost by default and operates on the same workspace as
the CLI (--dir). Only one writer per book: mutating requests are executed by
a single background worker, sequentially.

Agent access (ADR 30): set --token (or JIANWU_SERVER_TOKEN) to require
"Authorization: Bearer <token>" on /api/v1. Binding a non-localhost address
without a token is refused. See docs/AGENT_ACCESS.md.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if token == "" {
				token = os.Getenv("JIANWU_SERVER_TOKEN")
			}
			if err := requireTokenForNonLocal(addr, token); err != nil {
				return &InfoError{Err: err, Code: ExitCodeUsage}
			}
			startPath, source := workspace.ResolveRoot(cliWorkspaceDir)
			wsRoot, err := workspace.FindWorkspace(startPath)
			if err != nil {
				// Serve anyway: the UI offers workspace configuration + init.
				wsRoot = startPath
				if abs, aerr := os.Getwd(); aerr == nil && wsRoot == "." {
					wsRoot = abs
				}
				fmt.Fprintf(out, "warning: %v\n", err)
			}
			srv := server.NewWithSource(wsRoot, source, Version, nil)
			srv.SetToken(token)
			fmt.Fprintf(out, "jianwu %s — web UI: http://%s\n", Version, addr)
			fmt.Fprintf(out, "  workspace: %s（来源：%s）\n", wsRoot, workspace.SourceZh(source))
			fmt.Fprintf(out, "  可用 JIANWU_WORKSPACE 环境变量、全局配置 workspace: 键或网页设置切换\n")
			fmt.Fprintf(out, "  API:       http://%s/api/v1/workspace\n", addr)
			if token != "" {
				fmt.Fprintf(out, "  认证：      /api/v1 需要 Authorization: Bearer <token>（SPA 不受影响）\n")
			}
			fmt.Fprintf(out, "  Ctrl+C 退出\n")
			return http.ListenAndServe(addr, srv.Handler())
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8787", "listen address (default: localhost only)")
	cmd.Flags().StringVar(&token, "token", "", "require this Bearer token on /api/v1 (agent access; env JIANWU_SERVER_TOKEN)")
	return cmd
}

// requireTokenForNonLocal refuses to expose the API beyond localhost without
// a token (ADR 30 security model).
func requireTokenForNonLocal(addr, token string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid --addr %q: %w", addr, err)
	}
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1", "[::1]", "":
		return nil
	}
	if token == "" {
		return fmt.Errorf("refusing to bind non-localhost %s without a token: pass --token or set JIANWU_SERVER_TOKEN (ADR 30)", addr)
	}
	return nil
}
