package cli

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/engine"
	"github.com/iannil/jianwu/internal/engine/factcheck"
	"github.com/iannil/jianwu/internal/workspace"
)

func newFactCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "factcheck <slug> <NN-MM>",
		Short: "Auto-verify claims against cited sources",
		Long: `For each claim in an expanded chapter, read the cited source URL and
ask the LLM to verify whether the source actually supports the claim.

Only chapters with status "expanded" or "reviewed" can be fact-checked.
Results are stored in outline.json (verdicts field on each chapter).

Run the command again to re-check current sources.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFactCheck(cmd, args)
		},
	}
	cmd.Flags().Bool("tokens", false, "show token usage after completion")
	return cmd
}

func runFactCheck(cmd *cobra.Command, args []string) error {
	return runFactCheckWithDeps(cmd, args, nil)
}

func runFactCheckWithDeps(cmd *cobra.Command, args []string, deps *ProviderDeps) (err error) {
	out := cmd.OutOrStdout()
	slug, addr := args[0], args[1]
	partIdx, chIdx, err := parseChapterAddr(addr)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeUsage}
	}
	bc, err := loadBook(slug)
	if err != nil {
		return err
	}
	ch, err := findChapter(bc.Outline, partIdx, chIdx)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeUsage}
	}
	if ch.Status != book.StatusExpanded && ch.Status != book.StatusReviewed {
		return &InfoError{
			Err:  fmt.Errorf("chapter %s has status %q; only %q or %q chapters can be fact-checked", addr, ch.Status, book.StatusExpanded, book.StatusReviewed),
			Code: ExitCodeUsage,
		}
	}
	if len(ch.Claims) == 0 {
		fmt.Fprintf(out, "No claims to verify for %s/%s\n", slug, addr)
		return nil
	}

	if deps == nil {
		ws, err := workspace.Load(bc.WSRoot)
		if err != nil {
			return &InfoError{Err: err, Code: ExitCodeGeneric}
		}
		secrets, err := config.LoadSecrets()
		if err != nil {
			return &InfoError{Err: fmt.Errorf("load secrets: %w", err), Code: ExitCodeLLMProvider}
		}
		deps, err = buildProviderDeps(ws.Config, secrets)
		if err != nil {
			return &InfoError{Err: err, Code: ExitCodeLLMProvider}
		}

	}
	tracker := &engine.TokenTracker{}
	tracked := *deps
	tracked.Chatter = engine.NewTrackingChatter(deps.Chatter, tracker)
	deps = &tracked
	defer func() {
		err = errors.Join(err, persistTokenUsage(bc.BookDir, bc.Meta, tracker))
		if show, _ := cmd.Flags().GetBool("tokens"); show {
			printTokenUsage(out, tracker.Snapshot())
		}
	}()

	fmt.Fprintf(out, "Fact-checking %s/%s (%d claims)...\n", slug, addr, len(ch.Claims))

	result, err := factcheck.Run(cmd.Context(), deps.Chatter, deps.Reader, factcheck.Input{
		ChapterTitle: ch.Title,
		Claims:       ch.Claims,
		Citations:    ch.Citations,
	})
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	// Update outline with verdicts.
	ch.Verdicts = make([]book.ClaimVerdict, len(result.Verdicts))
	verifiedCount := 0
	for i, v := range result.Verdicts {
		ch.Verdicts[i] = book.ClaimVerdict{
			ClaimText:        v.ClaimText,
			Verified:         v.Verified,
			Reasoning:        v.Reasoning,
			SuggestedRewrite: v.SuggestedRewrite,
			CitationID:       v.CitationID,
		}
		if v.Verified {
			verifiedCount++
		}
	}
	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	// Summary.
	fmt.Fprintf(out, "✓ Fact-check complete: %d/%d verified\n", verifiedCount, len(result.Verdicts))
	for _, v := range result.Verdicts {
		status := "✓"
		if !v.Verified {
			status = "✗"
		}
		fmt.Fprintf(out, "  %s %s\n", status, truncateText(v.ClaimText, 80))
		fmt.Fprintf(out, "    → %s\n", v.Reasoning)
		if v.SuggestedRewrite != "" {
			fmt.Fprintf(out, "    → Suggestion: %s\n", v.SuggestedRewrite)
		}
	}
	for _, u := range result.SourceErrors {
		fmt.Fprintf(out, "  ⚠ Failed to read source: %s\n", u)
	}
	return nil
}

func truncateText(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
