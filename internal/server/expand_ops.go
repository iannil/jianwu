package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/engine"
	"github.com/iannil/jianwu/internal/engine/expand"
	"github.com/iannil/jianwu/internal/storage"
	"github.com/iannil/jianwu/internal/workspace"
)

// expand progress phases mapped to an overall 0-100 scale.
const (
	progressResearchStart = 5
	progressDraftStart    = 40
	progressValidateStart = 85
)

// expandProgress returns an expand.ProgressCallback that maps per-phase
// percentages onto the job's overall progress.
func expandProgress(j *Job) expand.ProgressCallback {
	return func(ev expand.ProgressEvent) {
		var pct int
		switch ev.Phase {
		case expand.PhaseResearch:
			pct = progressResearchStart + ev.Percent*(progressDraftStart-progressResearchStart)/100
		case expand.PhaseDraft:
			pct = progressDraftStart + ev.Percent*(progressValidateStart-progressDraftStart)/100
		case expand.PhaseValidate:
			pct = progressValidateStart + ev.Percent*(100-progressValidateStart)/100
		default:
			pct = ev.Percent
		}
		j.SetProgress(pct, ev.Message)
	}
}

// checkExpandOverwrite mirrors cli.checkExpandOverwrite: expanded chapters
// need one force, reviewed/final chapters need two.
func checkExpandOverwrite(bookDir string, partIdx, chIdx, forceCount int) error {
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
		return fmt.Errorf("章节 %02d-%02d 状态为 %q；需要 force=%d 才能覆盖", partIdx, chIdx, fm.Status, needed)
	}
	return nil
}

// bookConfig loads the workspace config for a book context.
func (s *Server) bookConfig(bc *bookCtx) (*config.Config, error) {
	ws, err := workspace.Load(bc.WSRoot)
	if err != nil {
		return nil, err
	}
	return ws.Config, nil
}

// expandInputFor builds the ExpandInput for chapter (partIdx, chIdx) from a
// book snapshot.
func expandInputFor(bc *bookCtx, partIdx, chIdx int) (expand.ExpandInput, error) {
	ch, err := findChapter(bc.Outline, partIdx, chIdx)
	if err != nil {
		return expand.ExpandInput{}, err
	}
	p := findPartByIndex(bc.Outline, partIdx)
	partTitle, partRole := "", ""
	if p != nil {
		partTitle, partRole = p.Title, p.Role
	}
	in := expand.ExpandInput{
		ArchetypeID: bc.Meta.Archetype, Topic: bc.Meta.Title,
		Audience: bc.Meta.Parameters.Audience, Depth: bc.Meta.Parameters.Depth,
		Goal: bc.Meta.Parameters.Goal, Length: bc.Meta.Parameters.Length,
		Language: bc.Meta.Language, PartIndex: partIdx, PartTitle: partTitle, PartRole: partRole,
		ChapterIndex: chIdx, ChapterTitle: ch.Title, Abstract: ch.Abstract,
		KeyConcepts: ch.KeyConcepts, WebSearchEnabled: true,
	}
	in.PreviousChapter, _ = findChapter(bc.Outline, partIdx, chIdx-1)
	in.NextChapter, _ = findChapter(bc.Outline, partIdx, chIdx+1)
	return in, nil
}

// generateExpandedChapter generates one chapter from a stable book snapshot.
// It never writes book files. Usage is returned for persistence by the caller.
func (s *Server) generateExpandedChapter(ctx context.Context, bc *bookCtx, deps *Deps, j *Job, partIdx, chIdx int) (*expand.ExpandOutput, book.TokenUsage, error) {
	in, err := expandInputFor(bc, partIdx, chIdx)
	if err != nil {
		return nil, book.TokenUsage{}, err
	}
	cfg, err := s.bookConfig(bc)
	if err != nil {
		return nil, book.TokenUsage{}, err
	}
	registry := s.buildRegistry(deps, cfg)

	var progress expand.ProgressCallback
	if j != nil {
		progress = expandProgress(j)
	}
	tracker := &engine.TokenTracker{}
	chatter := engine.NewTrackingChatter(deps.Chatter, tracker)
	result, err := expand.Generate(ctx, chatter, registry, in, progress)
	return result, tracker.Snapshot(), err
}

// saveExpandedChapterFiles persists one generated chapter and its outline
// state (mirrors cli.saveExpandedChapterFiles).
func (s *Server) saveExpandedChapterFiles(bc *bookCtx, partIdx, chIdx int, result *expand.ExpandOutput, usage book.TokenUsage) error {
	ch, err := findChapter(bc.Outline, partIdx, chIdx)
	if err != nil {
		return err
	}
	model, _ := stageModelOf(bc)
	fm := book.ChapterFrontmatter{
		Title: ch.Title, PartIndex: partIdx, ChapterIndex: chIdx, Status: book.StatusExpanded,
		WordCount: result.WordCount, GeneratedAt: time.Now().UTC(), Model: model.Model, EngineVersion: s.version,
		UnverifiedClaimsCount: len(result.UnverifiedClaims), Citations: toChapterCitations(result.Citations),
	}
	if _, err := book.WriteChapter(bc.BookDir, partIdx, chIdx, fm, result.Markdown); err != nil {
		return err
	}
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
	updated.ExpandedWith = &book.ExpandedWith{
		Provider: model.Provider, Model: model.Model, Iterations: 3,
		Tokens: book.Tokens{In: usage.PromptTokens, Out: usage.CompletionTokens},
	}
	previous := *ch
	*ch = updated
	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		*ch = previous
		return err
	}
	bc.Meta.Status = book.BookStatusDraft
	bc.Meta.UpdatedAt = time.Now().UTC()
	return book.SaveMeta(filepath.Join(bc.BookDir, "meta.json"), bc.Meta)
}

// stageModelOf returns the expand stage model for a book's workspace.
func stageModelOf(bc *bookCtx) (config.ModelRef, error) {
	cfg, err := workspaceLoadConfig(bc.WSRoot)
	if err != nil {
		return config.ModelRef{}, err
	}
	return stageModel(cfg, "expand")
}

// workspaceLoadConfig loads config for an arbitrary workspace root.
func workspaceLoadConfig(wsRoot string) (*config.Config, error) {
	ws, err := workspace.Load(wsRoot)
	if err != nil {
		return nil, err
	}
	return ws.Config, nil
}

// withBookRollback restores the affected chapter and metadata after ordinary
// I/O errors (mirror of cli.withBookRollback).
func (s *Server) withBookRollback(bc *bookCtx, partIdx, chIdx int, save func() error) error {
	type backup struct {
		path   string
		data   []byte
		exists bool
	}
	var before []backup
	for _, path := range []string{
		book.ChapterPath(bc.BookDir, partIdx, chIdx),
		filepath.Join(bc.BookDir, "outline.json"),
		filepath.Join(bc.BookDir, "meta.json"),
	} {
		data, err := book.DefaultStorage.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("backup %s: %w", path, err)
		}
		before = append(before, backup{path, data, err == nil})
	}
	ch, err := findChapter(bc.Outline, partIdx, chIdx)
	if err != nil {
		return err
	}
	oldChapter, oldMeta := *ch, *bc.Meta
	if err := save(); err != nil {
		*ch = oldChapter
		*bc.Meta = oldMeta
		for _, b := range before {
			current, readErr := book.DefaultStorage.ReadFile(b.path)
			if b.exists && readErr == nil && string(current) == string(b.data) {
				continue
			}
			var restoreErr error
			if b.exists {
				restoreErr = storage.WriteFileAtomic(book.DefaultStorage, b.path, b.data, 0o644)
			} else {
				restoreErr = book.DefaultStorage.RemoveAll(b.path)
			}
			if restoreErr != nil {
				err = errors.Join(err, fmt.Errorf("restore %s: %w", b.path, restoreErr))
			}
		}
		return err
	}
	return nil
}

// --- HTTP handlers ---

// handleChapterExpand starts a single-chapter expand job.
func (s *Server) handleChapterExpand(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Force int `json:"force"`
	}
	if err := decodeBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	slug := r.PathValue("slug")
	partIdx, err := pathInt(r, "part")
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	chIdx, err := pathInt(r, "ch")
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.requireWorkspace(w) {
		return
	}
	bc, err := s.loadBook(slug)
	if err != nil {
		failErr(w, err)
		return
	}
	if _, err := findChapter(bc.Outline, partIdx, chIdx); err != nil {
		fail(w, http.StatusNotFound, err.Error())
		return
	}
	if _, err := s.resolveDeps("expand", needExpand); err != nil {
		failErr(w, err)
		return
	}
	jobID := s.jobs.Start("expand", slug, func(ctx context.Context, j *Job) error {
		return s.runExpandChapter(ctx, j, slug, partIdx, chIdx, body.Force)
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID})
}

// handleExpandBook accepts either a chapter address or all=true (compat alias
// for expand-all). Prefer the dedicated endpoints.
func (s *Server) handleExpandBook(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Part    *int `json:"part"`
		Chapter *int `json:"chapter"`
		All     bool `json:"all"`
		Force   int  `json:"force"`
	}
	if err := decodeBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	slug := r.PathValue("slug")
	if body.All {
		s.startExpandAll(w, slug, body.Force)
		return
	}
	if body.Part == nil || body.Chapter == nil {
		fail(w, http.StatusBadRequest, "需要 part+chapter 或 all=true")
		return
	}
	if !s.requireWorkspace(w) {
		return
	}
	bc, err := s.loadBook(slug)
	if err != nil {
		failErr(w, err)
		return
	}
	if _, err := findChapter(bc.Outline, *body.Part, *body.Chapter); err != nil {
		fail(w, http.StatusNotFound, err.Error())
		return
	}
	if _, err := s.resolveDeps("expand", needExpand); err != nil {
		failErr(w, err)
		return
	}
	p, c := *body.Part, *body.Chapter
	jobID := s.jobs.Start("expand", slug, func(ctx context.Context, j *Job) error {
		return s.runExpandChapter(ctx, j, slug, p, c, body.Force)
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID})
}

// handleExpandAll starts the whole-book batch expand job.
func (s *Server) handleExpandAll(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Force int `json:"force"`
	}
	if err := decodeBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	s.startExpandAll(w, r.PathValue("slug"), body.Force)
}

// startExpandAll validates inputs and enqueues the batch job.
func (s *Server) startExpandAll(w http.ResponseWriter, slug string, forceCount int) {
	if !s.requireWorkspace(w) {
		return
	}
	bc, err := s.loadBook(slug)
	if err != nil {
		failErr(w, err)
		return
	}
	hasScaffolded := false
	for _, p := range bc.Outline.Parts {
		for _, c := range p.Chapters {
			if c.Status == book.StatusScaffolded {
				hasScaffolded = true
			}
		}
	}
	if !hasScaffolded {
		fail(w, http.StatusConflict, "没有可展开的 scaffolded 章节")
		return
	}
	if _, err := s.resolveDeps("expand", needExpand); err != nil {
		failErr(w, err)
		return
	}
	jobID := s.jobs.Start("expand-all", slug, func(ctx context.Context, j *Job) error {
		return s.runExpandAll(ctx, j, slug, forceCount)
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID})
}

// runExpandChapter mirrors cli.runExpand: generate one chapter, then persist
// with rollback; usage is recorded even on failure.
func (s *Server) runExpandChapter(ctx context.Context, j *Job, slug string, partIdx, chIdx, forceCount int) error {
	bc, err := s.loadBook(slug)
	if err != nil {
		return err
	}
	deps, err := s.resolveDeps("expand", needExpand)
	if err != nil {
		return err
	}
	if err := checkExpandOverwrite(bc.BookDir, partIdx, chIdx, forceCount); err != nil {
		return err
	}
	j.Logf("展开 %02d-%02d…", partIdx, chIdx)
	cfg, err := s.bookConfig(bc)
	if err != nil {
		return err
	}
	cctx, cancel := stageTimeoutCtx(ctx, cfg, "expand")
	defer cancel()
	result, usage, err := s.generateExpandedChapter(cctx, bc, deps, j, partIdx, chIdx)
	if err := persistUsage(bc.BookDir, bc.Meta, usage); err != nil {
		return err
	}
	if err != nil {
		return fmt.Errorf("生成失败: %w", err)
	}
	if err := s.withBookRollback(bc, partIdx, chIdx, func() error {
		return s.saveExpandedChapterFiles(bc, partIdx, chIdx, result, usage)
	}); err != nil {
		return err
	}
	j.Logf("完成 %02d-%02d：%d 字，%d 引用，%d 未验证论断", partIdx, chIdx, result.WordCount, len(result.Citations), len(result.UnverifiedClaims))
	j.SetResult("part", partIdx)
	j.SetResult("chapter", chIdx)
	j.SetResult("word_count", result.WordCount)
	j.SetResult("citations", len(result.Citations))
	j.SetResult("unverified", len(result.UnverifiedClaims))
	return nil
}

// runExpandAll mirrors cli.runExpandAllWithDeps: read-only outline snapshot,
// up to 5 concurrent generations, sequential persistence, failures visible.
func (s *Server) runExpandAll(ctx context.Context, j *Job, slug string, forceCount int) error {
	bc, err := s.loadBook(slug)
	if err != nil {
		return err
	}
	deps, err := s.resolveDeps("expand", needExpand)
	if err != nil {
		return err
	}
	cfg, err := s.bookConfig(bc)
	if err != nil {
		return err
	}
	registry := s.buildRegistry(deps, cfg)

	type target struct {
		part, ch int
		tracker  *engine.TokenTracker
		output   *expand.ExpandOutput
		err      error
	}
	var targets []*target
	for _, p := range bc.Outline.Parts {
		for _, c := range p.Chapters {
			if c.Status == book.StatusScaffolded {
				targets = append(targets, &target{part: p.Index, ch: c.Index, tracker: &engine.TokenTracker{}})
			}
		}
	}
	if len(targets) == 0 {
		j.SetResult("message", "没有可展开的 scaffolded 章节")
		return nil
	}
	for _, t := range targets {
		if err := checkExpandOverwrite(bc.BookDir, t.part, t.ch, forceCount); err != nil {
			t.err = err
		}
	}
	j.Logf("批量展开 %d 个章节（最多 5 并发）", len(targets))
	j.SetProgress(1, fmt.Sprintf("0/%d 已生成", len(targets)))

	var completed atomic.Int32
	var g errgroup.Group
	g.SetLimit(5)
	for _, t := range targets {
		t := t
		if t.err != nil {
			continue
		}
		g.Go(func() error {
			cctx, cancel := stageTimeoutCtx(ctx, cfg, "expand")
			defer cancel()
			in, ferr := expandInputFor(bc, t.part, t.ch)
			if ferr != nil {
				t.err = ferr
				return nil
			}
			chatter := engine.NewTrackingChatter(deps.Chatter, t.tracker)
			t.output, t.err = expand.Generate(cctx, chatter, registry, in, nil)
			done := completed.Add(1)
			j.SetProgress(1+int(done)*80/len(targets), fmt.Sprintf("%d/%d 已生成", done, len(targets)))
			return nil // preserve independent successes after another chapter fails
		})
	}
	_ = g.Wait()

	var allErr error
	success, failed := 0, 0
	for _, t := range targets {
		usage := t.tracker.Snapshot()
		j.AddUsage(usage)
		if t.err == nil {
			t.err = s.withBookRollback(bc, t.part, t.ch, func() error {
				return s.saveExpandedChapterFiles(bc, t.part, t.ch, t.output, usage)
			})
		}
		if err := persistUsage(bc.BookDir, bc.Meta, usage); err != nil {
			t.err = errors.Join(t.err, err)
		}
		if t.err != nil {
			failed++
			allErr = errors.Join(allErr, fmt.Errorf("章节 %02d-%02d: %w", t.part, t.ch, t.err))
			j.Logf("✗ %02d-%02d: %v", t.part, t.ch, t.err)
		} else {
			success++
			j.Logf("✓ %02d-%02d", t.part, t.ch)
		}
	}
	j.SetResult("succeeded", success)
	j.SetResult("failed", failed)
	j.Logf("批量展开结束：%d 成功，%d 失败", success, failed)
	if allErr != nil {
		return allErr
	}
	return nil
}

// persistUsage merges one run's usage into meta exactly once.
func persistUsage(bookDir string, meta *book.Meta, usage book.TokenUsage) error {
	if usage.CallCount == 0 {
		return nil
	}
	meta.TokenUsage.Add(usage)
	return book.SaveMeta(filepath.Join(bookDir, "meta.json"), meta)
}

// toChapterCitations converts expand.Citation to book.ChapterCitation.
func toChapterCitations(cs []expand.Citation) []book.ChapterCitation {
	out := make([]book.ChapterCitation, 0, len(cs))
	for _, c := range cs {
		out = append(out, book.ChapterCitation{ID: c.ID, URL: c.URL, Title: c.Title, Site: extractSite(c.URL)})
	}
	return out
}

// toBookCitations converts expand.Citation to book.Citation.
func toBookCitations(cs []expand.Citation) []book.Citation {
	out := make([]book.Citation, 0, len(cs))
	for _, c := range cs {
		out = append(out, book.Citation{
			ID: c.ID, URL: c.URL, Title: c.Title, AccessedAt: c.AccessedAt,
			Snippet: c.Snippet, SearchProvider: c.SearchProvider, ReaderProvider: c.ReaderProvider,
		})
	}
	return out
}

// extractSite returns the host portion of a URL for the frontmatter site field.
func extractSite(rawURL string) string {
	s := rawURL
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return s
}
