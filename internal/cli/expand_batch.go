package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/engine"
	"github.com/iannil/jianwu/internal/engine/expand"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/workspace"
)

// generateExpandedChapter reads a stable book snapshot; it never writes book files.
func generateExpandedChapter(parent context.Context, bc *bookCtx, cfg *config.Config, deps *ProviderDeps, partIdx, chIdx int, verbose bool, out io.Writer, tracker *engine.TokenTracker) (*expand.ExpandOutput, error) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	seconds := cfg.LLM.TimeoutSeconds
	if cfg.Models.Expand.TimeoutSeconds > 0 {
		seconds = cfg.Models.Expand.TimeoutSeconds
	}
	if seconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
		defer cancel()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ch, err := findChapter(bc.Outline, partIdx, chIdx)
	if err != nil {
		return nil, err
	}
	registry, err := buildToolRegistry(deps, cfg, bc.WSRoot)
	if err != nil {
		return nil, err
	}
	m := bc.Meta
	p := findPart(bc.Outline, partIdx)
	in := expand.ExpandInput{ArchetypeID: m.Archetype, Topic: m.Title, Audience: m.Parameters.Audience, Depth: m.Parameters.Depth,
		Goal: m.Parameters.Goal, Length: m.Parameters.Length, Language: m.Language, PartIndex: partIdx, PartTitle: p.Title, PartRole: p.Role,
		ChapterIndex: chIdx, ChapterTitle: ch.Title, Abstract: ch.Abstract, KeyConcepts: ch.KeyConcepts, WordCountTarget: ch.WordCountTarget, WebSearchEnabled: true}
	in.PreviousChapter, _ = findChapter(bc.Outline, partIdx, chIdx-1)
	in.NextChapter, _ = findChapter(bc.Outline, partIdx, chIdx+1)
	chatter := engine.NewTrackingChatter(deps.Chatter, tracker)
	if verbose {
		if _, ok := deps.Chatter.(llm.Streamer); ok {
			in.Streamer = chatter
			in.StreamOutput = out
		}
	}
	return expand.Generate(ctx, chatter, registry, in, nil)
}

func runExpandAll(cmd *cobra.Command, slug string, forceCount int) error {
	return runExpandAllWithDeps(cmd, slug, forceCount, nil, false)
}

// runExpandAllWithDeps bounds generation concurrency and serializes all persistence.
// The outline is immutable until every worker has stopped, including on cancellation.
func runExpandAllWithDeps(cmd *cobra.Command, slug string, forceCount int, deps *ProviderDeps, showTokens bool) error {
	out := cmd.OutOrStdout()
	parent := cmd.Context()
	if parent == nil {
		parent = context.Background()
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	bc, err := loadBook(slug)
	if err != nil {
		return err
	}
	ws, err := workspace.Load(bc.WSRoot)
	if err != nil {
		return err
	}
	type result struct {
		part, ch int
		output   *expand.ExpandOutput
		tracker  *engine.TokenTracker
		err      error
	}
	var results []result
	for _, p := range bc.Outline.Parts {
		for _, ch := range p.Chapters {
			if ch.Status == book.StatusScaffolded {
				results = append(results, result{part: p.Index, ch: ch.Index, tracker: &engine.TokenTracker{}})
			}
		}
	}
	if len(results) == 0 {
		fmt.Fprintf(out, "No scaffolded chapters to expand in %s\n", slug)
		return nil
	}
	deps, err = expandDeps(ws.Config, deps)
	if err != nil {
		return err
	}
	for i := range results {
		r := &results[i]
		r.err = checkExpandOverwrite(out, bc.BookDir, r.part, r.ch, forceCount)
	}
	fmt.Fprintf(out, "Expanding %d scaffolded chapter(s) in %s...\n", len(results), slug)
	var g errgroup.Group
	g.SetLimit(5)
	for i := range results {
		r := &results[i]
		if r.err != nil {
			continue
		}
		g.Go(func() error {
			r.output, r.err = generateExpandedChapter(ctx, bc, ws.Config, deps, r.part, r.ch, false, io.Discard, r.tracker)
			return nil // preserve independent successes after another chapter fails
		})
	}
	_ = g.Wait()
	var allErr error
	var total book.TokenUsage
	success, failed := 0, 0
	for i := range results {
		r := &results[i]
		usage := r.tracker.Snapshot()
		total.Add(usage)
		if r.err == nil {
			r.err = saveExpandedChapter(out, bc, ws.Config, r.part, r.ch, r.output, usage)
		}
		r.err = errors.Join(r.err, persistTokenUsage(bc.BookDir, bc.Meta, r.tracker))
		if r.err != nil {
			failed++
			allErr = errors.Join(allErr, fmt.Errorf("chapter %02d-%02d: %w", r.part, r.ch, r.err))
			fmt.Fprintf(out, "  ✗ %02d-%02d: %v\n", r.part, r.ch, r.err)
		} else {
			success++
		}
	}
	fmt.Fprintf(out, "Expand complete: %d succeeded, %d failed\n", success, failed)
	if showTokens {
		printTokenUsage(out, total)
	}
	if allErr != nil {
		return &InfoError{Err: allErr, Code: ExitCodeGeneric}
	}
	return nil
}
