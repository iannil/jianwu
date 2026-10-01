package cli

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"github.com/iannil/jianwu/internal/server"
	"github.com/iannil/jianwu/internal/workspace"
)

func newServeCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the local web UI + HTTP API for this workspace",
		Long: `Start a local web server exposing jianwu's full pipeline (interview →
outline → scaffolding → expand → factcheck → revise → review → finalize →
export) through a browser UI and a JSON API under /api/v1.

The server binds localhost by default and operates on the same workspace as
the CLI (--dir). Only one writer per book: mutating requests are executed by
a single background worker, sequentially.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
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
			fmt.Fprintf(out, "jianwu %s — web UI: http://%s\n", Version, addr)
			fmt.Fprintf(out, "  workspace: %s（来源：%s）\n", wsRoot, workspace.SourceZh(source))
			fmt.Fprintf(out, "  可用 JIANWU_WORKSPACE 环境变量、全局配置 workspace: 键或网页设置切换\n")
			fmt.Fprintf(out, "  API:       http://%s/api/v1/workspace\n", addr)
			fmt.Fprintf(out, "  Ctrl+C 退出\n")
			return http.ListenAndServe(addr, srv.Handler())
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8787", "listen address (default: localhost only)")
	return cmd
}
