package server

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/engine"
	"github.com/iannil/jianwu/internal/engine/scaffolding"
)

// handleScaffoldRetry starts a job that re-runs scaffold generation for the
// book's chapters whose outline status is "failed" (the recovery path for a
// `new-book` job that ended with failed scaffolds).
func (s *Server) handleScaffoldRetry(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	bc, err := s.loadBook(r.PathValue("slug"))
	if err != nil {
		failErr(w, err)
		return
	}
	hasFailed := false
	for _, p := range bc.Outline.Parts {
		for _, c := range p.Chapters {
			if c.Status == book.StatusFailed {
				hasFailed = true
			}
		}
	}
	if !hasFailed {
		fail(w, http.StatusConflict, "没有 status=failed 的章节，无需重试")
		return
	}
	if _, err := s.resolveDeps("scaffolding", needChatterOnly); err != nil {
		failErr(w, err)
		return
	}
	jobID := s.jobs.Start("scaffold-retry", r.PathValue("slug"), func(ctx context.Context, j *Job) error {
		return s.runScaffoldRetry(ctx, j, r.PathValue("slug"))
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID})
}

// runScaffoldRetry mirrors the CLI scaffolding --retry-failed core: retry
// failed scaffolds, persist outline, merge LLM usage into meta.
func (s *Server) runScaffoldRetry(ctx context.Context, j *Job, slug string) error {
	bc, err := s.loadBook(slug)
	if err != nil {
		return err
	}
	deps, err := s.resolveDeps("scaffolding", needChatterOnly)
	if err != nil {
		return err
	}
	cfg, err := s.loadWorkspace()
	if err != nil {
		return err
	}
	tracker := &engine.TokenTracker{}
	chatter := engine.NewTrackingChatter(deps.Chatter, tracker)

	scaffCtx, cancel := stageTimeoutCtx(ctx, cfg.Config, "scaffolding")
	defer cancel()
	j.SetProgress(10, "重试失败章节框架")
	results := scaffolding.RetryFailed(scaffCtx, chatter, bc.Outline, bc.Meta.Archetype, scaffolding.ChapterParams{
		Topic:    bc.Meta.Title,
		Audience: bc.Meta.Parameters.Audience,
		Depth:    bc.Meta.Parameters.Depth,
		Goal:     bc.Meta.Parameters.Goal,
		Length:   bc.Meta.Parameters.Length,
		Language: bc.Meta.Language,
	}, scaffolding.Options{})

	okCount, failCount := 0, 0
	for key, res := range results {
		if res.Err != nil {
			failCount++
			j.Logf("框架失败 %s: %v", key, res.Err)
		} else {
			okCount++
		}
	}
	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		return err
	}
	if err := persistUsage(bc.BookDir, bc.Meta, tracker.Snapshot()); err != nil {
		return err
	}

	j.SetResult("retried", len(results))
	j.SetResult("recovered", okCount)
	j.SetResult("failed", failCount)
	j.SetProgress(100, fmt.Sprintf("恢复 %d 章，仍失败 %d 章", okCount, failCount))
	if failCount > 0 {
		return fmt.Errorf("%d 个章节框架重试仍失败", failCount)
	}
	return nil
}
