package cli

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/engine"
	"github.com/iannil/jianwu/internal/engine/scaffolding"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/workspace"
)

func newScaffoldingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scaffolding <slug>",
		Short: "Retry failed chapter scaffolds",
		Long: `Re-run scaffold generation for chapters whose outline status is "failed"
(e.g. after a chapter failed during ` + "`jianwu new`" + `).

Chapters in any other status are never touched. Without --retry-failed the
command only lists the failed chapters.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScaffolding(cmd, args)
		},
	}
	cmd.Flags().Bool("retry-failed", false, "retry chapters with status \"failed\"")
	cmd.Flags().Bool("tokens", false, "show token usage after completion")
	return cmd
}

func runScaffolding(cmd *cobra.Command, args []string) error {
	return runScaffoldingWithChatter(cmd, args, nil)
}

func runScaffoldingWithChatter(cmd *cobra.Command, args []string, chatter llm.Chatter) (err error) {
	out := cmd.OutOrStdout()
	slug := args[0]
	bc, err := loadBook(slug)
	if err != nil {
		return err
	}

	var failed []string
	for _, p := range bc.Outline.Parts {
		for _, c := range p.Chapters {
			if c.Status == book.StatusFailed {
				failed = append(failed, fmt.Sprintf("%02d-%02d %s", p.Index, c.Index, c.Title))
			}
		}
	}
	if len(failed) == 0 {
		fmt.Fprintf(out, "No failed scaffolds in %q\n", slug)
		return nil
	}

	retry, _ := cmd.Flags().GetBool("retry-failed")
	if !retry {
		fmt.Fprintf(out, "%d failed scaffold(s) in %q:\n", len(failed), slug)
		for _, f := range failed {
			fmt.Fprintf(out, "  %s\n", f)
		}
		fmt.Fprintf(out, "Run `jianwu scaffolding %s --retry-failed` to retry them.\n", slug)
		return nil
	}

	ws, err := workspace.Load(bc.WSRoot)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}
	if chatter == nil {
		secrets, err := config.LoadSecrets()
		if err != nil {
			return &InfoError{Err: fmt.Errorf("load secrets: %w", err), Code: ExitCodeLLMProvider}
		}
		chatter, err = buildChatter(ws.Config, secrets, "scaffolding")
		if err != nil {
			return &InfoError{Err: err, Code: ExitCodeLLMProvider}
		}
	}
	tracker := &engine.TokenTracker{}
	tracked := engine.NewTrackingChatter(chatter, tracker)
	defer func() {
		err = errors.Join(err, persistTokenUsage(bc.BookDir, bc.Meta, tracker))
		if show, _ := cmd.Flags().GetBool("tokens"); show {
			printTokenUsage(out, tracker.Snapshot())
		}
	}()

	scaffCtx, cancel := stageCtx(ws.Config, "scaffolding")
	defer cancel()
	results := scaffolding.RetryFailed(scaffCtx, tracked, bc.Outline, bc.Meta.Archetype, scaffolding.ChapterParams{
		Topic:    bc.Meta.Title,
		Audience: bc.Meta.Parameters.Audience,
		Depth:    bc.Meta.Parameters.Depth,
		Goal:     bc.Meta.Parameters.Goal,
		Length:   bc.Meta.Parameters.Length,
		Language: bc.Meta.Language,
	}, scaffolding.Options{
		Progress: func(p scaffolding.ScaffoldProgress) {
			fmt.Fprintf(out, "  %s %s\n", p.Status, p.Title)
		},
	})

	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	okCount, failCount := 0, 0
	for key, res := range results {
		if res.Err != nil {
			failCount++
			fmt.Fprintf(out, "✗ %s: %v\n", key, res.Err)
		} else {
			okCount++
		}
	}
	fmt.Fprintf(out, "✓ Scaffold retry complete: %d recovered, %d still failed\n", okCount, failCount)
	if failCount > 0 {
		return &InfoError{
			Err:  fmt.Errorf("%d chapter scaffold(s) still failing; run again to retry", failCount),
			Code: ExitCodeGeneric,
		}
	}
	return nil
}
