package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/engine"
	"github.com/iannil/jianwu/internal/engine/expand"
	"github.com/iannil/jianwu/internal/workspace"
)

func newExpandCmd() *cobra.Command {
	var forceCount int
	var expandAll, showTokens bool
	cmd := &cobra.Command{Use: "expand <slug> <NN-MM>", Short: "Expand one (or all) chapters into markdown with citations",
		Long: `Run research → draft → validate for a chapter, then save its prose, claims and citations.
Use --all for all scaffolded chapters (up to 5 concurrent generations).
Failed chapters do not stop others; any failure returns a nonzero exit code.
Use --force to overwrite expanded chapters, twice for reviewed/final chapters.
Token usage is always recorded; --tokens displays this run's reported usage.`,
		Args: cobra.RangeArgs(1, 2), RunE: func(cmd *cobra.Command, args []string) error {
			if expandAll {
				if len(args) != 1 {
					return &InfoError{Err: fmt.Errorf("--all requires only <slug>"), Code: ExitCodeUsage}
				}
				return runExpandAllWithDeps(cmd, args[0], forceCount, nil, showTokens)
			}
			if len(args) != 2 {
				return &InfoError{Err: fmt.Errorf("requires <slug> <NN-MM> or --all"), Code: ExitCodeUsage}
			}
			return runExpand(cmd, args, forceCount, nil, showTokens)
		}}
	cmd.Flags().CountVarP(&forceCount, "force", "f", "overwrite chapter (twice to override reviewed/final)")
	cmd.Flags().BoolVar(&expandAll, "all", false, "expand all scaffolded chapters")
	cmd.Flags().BoolVar(&showTokens, "tokens", false, "show reported token usage after completion")
	return cmd
}

func expandDeps(cfg *config.Config, deps *ProviderDeps) (*ProviderDeps, error) {
	if deps != nil {
		return deps, nil
	}
	secrets, err := config.LoadSecrets()
	if err != nil {
		return nil, &InfoError{Err: fmt.Errorf("load secrets: %w", err), Code: ExitCodeLLMProvider}
	}
	deps, err = buildProviderDeps(cfg, secrets)
	if err != nil {
		return nil, &InfoError{Err: err, Code: ExitCodeLLMProvider}
	}
	return deps, nil
}

// runExpand generates one chapter and records usage even when generation fails.
func runExpand(cmd *cobra.Command, args []string, forceCount int, deps *ProviderDeps, showTokens bool) (runErr error) {
	partIdx, chIdx, err := parseChapterAddr(args[1])
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeUsage}
	}
	bc, err := loadBook(args[0])
	if err != nil {
		return err
	}
	ws, err := workspace.Load(bc.WSRoot)
	if err != nil {
		return err
	}
	if _, err := findChapter(bc.Outline, partIdx, chIdx); err != nil {
		return &InfoError{Err: err, Code: ExitCodeUsage}
	}
	if err := checkExpandOverwrite(cmd.OutOrStdout(), bc.BookDir, partIdx, chIdx, forceCount); err != nil {
		return err
	}
	deps, err = expandDeps(ws.Config, deps)
	if err != nil {
		return err
	}
	tracker := &engine.TokenTracker{}
	defer func() {
		runErr = errors.Join(runErr, persistTokenUsage(bc.BookDir, bc.Meta, tracker))
		if showTokens {
			printTokenUsage(cmd.OutOrStdout(), tracker.Snapshot())
		}
	}()
	fmt.Fprintf(cmd.OutOrStdout(), "Expanding %s/%s...\n", args[0], args[1])
	result, err := generateExpandedChapter(cmd.Context(), bc, ws.Config, deps, partIdx, chIdx, GlobalFlagsFrom(cmd).Verbose, cmd.OutOrStdout(), tracker)
	if err != nil {
		return wrapLLMError(err)
	}
	return saveExpandedChapter(cmd.OutOrStdout(), bc, ws.Config, partIdx, chIdx, result, tracker.Snapshot())
}

func checkExpandOverwrite(out io.Writer, bookDir string, partIdx, chIdx, forceCount int) error {
	path := book.ChapterPath(bookDir, partIdx, chIdx)
	if _, err := book.DefaultStorage.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat chapter: %w", err)
	}
	fm, _, err := book.ReadChapter(path)
	if err != nil {
		return fmt.Errorf("read existing chapter: %w", err)
	}
	needed := 1
	if fm.Status == book.StatusReviewed || fm.Status == book.StatusFinal {
		needed = 2
	}
	if forceCount < needed {
		return &InfoError{Err: fmt.Errorf("chapter %02d-%02d has status %q; use --force %d time(s) to overwrite", partIdx, chIdx, fm.Status, needed), Code: ExitCodeGeneric}
	}
	fmt.Fprintf(out, "warning: overwriting %s (was: %s, %d words)\n", path, fm.Status, fm.WordCount)
	return nil
}

// saveExpandedChapter runs only in the coordinator, after generation workers finish.
func saveExpandedChapterFiles(out io.Writer, bc *bookCtx, cfg *config.Config, partIdx, chIdx int, result *expand.ExpandOutput, usage book.TokenUsage) error {
	ch, err := findChapter(bc.Outline, partIdx, chIdx)
	if err != nil {
		return err
	}
	model, _ := stageModel(cfg, "expand")
	fm := book.ChapterFrontmatter{Title: ch.Title, PartIndex: partIdx, ChapterIndex: chIdx, Status: book.StatusExpanded,
		WordCount: result.WordCount, GeneratedAt: time.Now().UTC(), Model: model.Model, EngineVersion: Version,
		UnverifiedClaimsCount: len(result.UnverifiedClaims), Citations: toChapterCitations(result.Citations)}
	path, err := book.WriteChapter(bc.BookDir, partIdx, chIdx, fm, result.Markdown)
	if err != nil {
		return err
	}
	// A replacement invalidates prior human review and source verdicts.
	updated := *ch
	updated.Status = book.StatusExpanded
	updated.WordCount = result.WordCount
	updated.CitationsCount = len(result.Citations)
	updated.UnverifiedClaims = len(result.UnverifiedClaims)
	updated.Citations = toBookCitations(result.Citations)
	updated.Claims = make([]book.Claim, len(result.Claims))
	for i, c := range result.Claims {
		updated.Claims[i] = book.Claim{Text: c.Text, HasCitation: c.HasCitation, CitationIDs: c.CitationIDs}
	}
	updated.Verdicts = nil
	updated.ReviewedAt = nil
	updated.ReviewedBy = ""
	updated.ExpandedWith = &book.ExpandedWith{Provider: model.Provider, Model: model.Model, Iterations: 3, Tokens: book.Tokens{In: usage.PromptTokens, Out: usage.CompletionTokens}}
	previous := *ch
	*ch = updated
	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		*ch = previous
		return err
	}
	bc.Meta.Status = book.BookStatusDraft
	bc.Meta.UpdatedAt = time.Now().UTC()
	if err := book.SaveMeta(filepath.Join(bc.BookDir, "meta.json"), bc.Meta); err != nil {
		return err
	}
	fmt.Fprintf(out, "✓ Wrote %s\n  Words: %d, Citations: %d, Unverified claims: %d\n", path, result.WordCount, len(result.Citations), len(result.UnverifiedClaims))
	return nil
}
