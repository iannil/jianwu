package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/corpus"
	"github.com/iannil/jianwu/internal/engine"
	"github.com/iannil/jianwu/internal/engine/grill"
	"github.com/iannil/jianwu/internal/engine/outline"
	"github.com/iannil/jianwu/internal/engine/scaffolding"
)

// dimView is the JSON projection of a grill dimension.
type dimView struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Question     string   `json:"question"`
	Options      []string `json:"options"`
	Required     bool     `json:"required"`
	DefaultValue string   `json:"default_value"`
}

func dimToView(d grill.Dimension) dimView {
	return dimView{
		ID: d.ID, Name: d.Name, Description: d.Description,
		Question: d.Question, Options: d.Options,
		Required: d.Required, DefaultValue: d.DefaultValue,
	}
}

// questionView is the payload returned by start/answer: the pending question
// plus the LLM recommendation for it.
type questionView struct {
	SessionID      string            `json:"session_id"`
	Status         string            `json:"status"`
	Complete       bool              `json:"complete"`
	Dim            *dimView          `json:"dimension,omitempty"`
	Recommendation string            `json:"recommendation,omitempty"`
	Answers        map[string]string `json:"answers"`
}

func sessionView(session *grill.Session, dim *grill.Dimension, rec string) questionView {
	v := questionView{
		SessionID: session.ID,
		Status:    string(session.Status),
		Complete:  dim == nil,
		Answers:   session.Answers,
	}
	if dim != nil {
		dv := dimToView(*dim)
		v.Dim = &dv
		v.Recommendation = rec
	}
	return v
}

// handleGrillTree serves the design-tree dimensions for the UI wizard.
func (s *Server) handleGrillTree(w http.ResponseWriter, r *http.Request) {
	tree := grill.DefaultTree()
	views := make([]dimView, 0, len(tree.Dimensions))
	for _, d := range tree.Dimensions {
		views = append(views, dimToView(d))
	}
	writeJSON(w, http.StatusOK, map[string]any{"dimensions": views})
}

// handleGrillSessions lists incomplete sessions (resumable).
func (s *Server) handleGrillSessions(w http.ResponseWriter, r *http.Request) {
	repo := grill.NewRepository(s.root())
	incomplete, err := repo.ListIncomplete()
	if err != nil {
		failErr(w, err)
		return
	}
	type entry struct {
		ID        string            `json:"id"`
		StartedAt time.Time         `json:"started_at"`
		Topic     string            `json:"topic,omitempty"`
		Answers   map[string]string `json:"answers"`
	}
	out := make([]entry, 0, len(incomplete))
	for _, sess := range incomplete {
		out = append(out, entry{ID: sess.ID, StartedAt: sess.StartedAt, Topic: sess.Answers["topic"], Answers: sess.Answers})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

// handleGrillStart creates a session and returns the first question with its
// LLM recommendation. The client may supply the topic up front (the first
// dimension is free-text and has no meaningful LLM recommendation).
func (s *Server) handleGrillStart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Topic string `json:"topic"`
	}
	if err := decodeBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.requireWorkspace(w) {
		return
	}

	cfg, err := s.workspaceConfig()
	if err != nil {
		failErr(w, err)
		return
	}
	deps, err := s.resolveDeps("intake", needChatterOnly)
	if err != nil {
		failErr(w, err)
		return
	}

	tree := grill.DefaultTree()
	repo := grill.NewRepository(s.root())
	session := grill.NewSession()
	if body.Topic != "" {
		session.RecordAnswer("topic", strings.TrimSpace(body.Topic))
	}

	next := tree.NextPending(session.Answers)
	var rec string
	if next != nil {
		rec, err = s.recommend(r.Context(), cfg, deps, next, session)
		if err != nil {
			failErr(w, err)
			return
		}
		session.RecordRecommendation(next.ID, rec)
	} else {
		session.Status = grill.SessionCompleted
	}
	if err := repo.Save(session); err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sessionView(session, next, rec))
}

// recommend calls the grill Recommend engine; the provider-reported usage of
// the call is merged into the session by the caller via session.TokenUsage.
func (s *Server) recommend(ctx context.Context, cfg *config.Config, deps *Deps, dim *grill.Dimension, session *grill.Session) (string, error) {
	tracker := &engine.TokenTracker{}
	chatter := engine.NewTrackingChatter(deps.Chatter, tracker)
	cctx, cancel := stageTimeoutCtx(ctx, cfg, "intake")
	defer cancel()
	rec, err := grill.Recommend(cctx, chatter, *dim, session.Answers)
	if err != nil {
		return "", fmt.Errorf("recommend for %s: %w", dim.ID, err)
	}
	session.TokenUsage.Add(tracker.Snapshot())
	return rec, nil
}

// handleGrillSessionGet returns a session snapshot without calling the LLM.
func (s *Server) handleGrillSessionGet(w http.ResponseWriter, r *http.Request) {
	repo := grill.NewRepository(s.root())
	session, err := repo.Load(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, fmt.Sprintf("session %q not found", r.PathValue("id")))
		return
	}
	tree := grill.DefaultTree()
	next := tree.NextPending(session.Answers)
	var rec string
	if next != nil {
		rec = session.Recommendations[next.ID]
	}
	writeJSON(w, http.StatusOK, sessionView(session, next, rec))
}

// handleGrillAnswer records the user's answer for the pending dimension and
// returns the next question (with a fresh LLM recommendation) or completion.
// Answer resolution mirrors grill.Run: empty = accept recommendation first
// line; "skip" = dimension default; invalid option values fall back to default.
func (s *Server) handleGrillAnswer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Answer string `json:"answer"`
	}
	if err := decodeBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.requireWorkspace(w) {
		return
	}

	cfg, err := s.workspaceConfig()
	if err != nil {
		failErr(w, err)
		return
	}
	deps, err := s.resolveDeps("intake", needChatterOnly)
	if err != nil {
		failErr(w, err)
		return
	}

	repo := grill.NewRepository(s.root())
	session, err := repo.Load(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, fmt.Sprintf("session %q not found", r.PathValue("id")))
		return
	}
	tree := grill.DefaultTree()

	pending := tree.NextPending(session.Answers)
	if pending == nil {
		writeJSON(w, http.StatusOK, sessionView(session, nil, ""))
		return
	}

	rec := session.Recommendations[pending.ID]
	final := strings.TrimSpace(body.Answer)
	switch {
	case final == "":
		final = firstLine(rec)
	case final == "skip":
		final = pending.DefaultValue
	}
	if !pending.ValidateAnswer(final) && pending.DefaultValue != "" {
		final = pending.DefaultValue
	}

	session.RecordAnswer(pending.ID, final)
	session.AddTurn(grill.Turn{
		Dimension:      pending.ID,
		Question:       pending.Question,
		Recommendation: rec,
		UserAnswer:     final,
	})
	session.CurrentDim = pending.ID

	// Fetch the recommendation for the next dimension, if any.
	next := tree.NextPending(session.Answers)
	var nextRec string
	if next != nil {
		nextRec, err = s.recommend(r.Context(), cfg, deps, next, session)
		if err != nil {
			// Persist the answer even when the next recommendation fails so
			// progress is not lost; the client can retry to fetch the question.
			_ = repo.Save(session)
			failErr(w, err)
			return
		}
		session.RecordRecommendation(next.ID, nextRec)
	} else {
		session.Status = grill.SessionCompleted
	}
	if err := repo.Save(session); err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionView(session, next, nextRec))
}

// handleGrillAbandon marks a session abandoned so it stops appearing for resume.
func (s *Server) handleGrillAbandon(w http.ResponseWriter, r *http.Request) {
	repo := grill.NewRepository(s.root())
	session, err := repo.Load(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, fmt.Sprintf("session %q not found", r.PathValue("id")))
		return
	}
	session.Status = grill.SessionAbandoned
	if err := repo.Save(session); err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleGrillGenerate launches the outline → scaffolding job for a completed
// interview session (the automated half of `jianwu new`).
func (s *Server) handleGrillGenerate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Force bool `json:"force"`
	}
	if err := decodeBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.requireWorkspace(w) {
		return
	}
	repo := grill.NewRepository(s.root())
	session, err := repo.Load(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, fmt.Sprintf("session %q not found", r.PathValue("id")))
		return
	}
	tree := grill.DefaultTree()
	if !session.IsComplete(tree) {
		fail(w, http.StatusConflict, "访谈未完成；请先回答所有必答维度")
		return
	}
	slug := book.Slugify(session.Answers["topic"])
	if slug == "" {
		fail(w, http.StatusBadRequest, fmt.Sprintf("无法从主题 %q 生成 slug", session.Answers["topic"]))
		return
	}
	if s.bookDirExists(slug) && !body.Force {
		fail(w, http.StatusConflict, fmt.Sprintf("图书 %q 已存在；设置 force 覆盖", slug))
		return
	}
	deps, err := s.resolveDeps("outline", needChatterOnly)
	if err != nil {
		failErr(w, err)
		return
	}
	jobID := s.jobs.Start("new-book", slug, func(ctx context.Context, j *Job) error {
		return s.runNewBookJob(ctx, j, session, deps, body.Force)
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID, "slug": slug})
}

// runNewBookJob mirrors cli.runNewFlowWithChatters steps 3-8: write meta,
// generate outline, scaffold chapters, archive the session.
func (s *Server) runNewBookJob(ctx context.Context, j *Job, session *grill.Session, deps *Deps, force bool) error {
	ws, err := s.loadWorkspace()
	if err != nil {
		return err
	}
	slug := book.Slugify(session.Answers["topic"])
	bookDir := filepath.Join(s.root(), "books", slug)
	if force {
		if err := os.RemoveAll(bookDir); err != nil {
			return fmt.Errorf("remove existing book dir: %w", err)
		}
	}

	j.SetProgress(5, "写入图书元数据")
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		return fmt.Errorf("mkdir book dir: %w", err)
	}
	meta := &book.Meta{
		TokenUsage:   session.TokenUsage,
		SessionUsage: map[string]book.TokenUsage{session.ID: session.TokenUsage},
		ID:           uuid.NewString(),
		Slug:         slug,
		Title:        session.Answers["topic"],
		Archetype:    session.Answers["archetype"],
		Language:     session.Answers["language"],
		Status:       book.BookStatusDraft,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		Parameters: book.Parameters{
			Audience: session.Answers["audience"],
			Depth:    session.Answers["depth"],
			Goal:     session.Answers["goal"],
			Length:   session.Answers["length"],
		},
		Engine: book.EngineMeta{JianwuVersion: s.version},
	}
	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"), meta); err != nil {
		return err
	}

	// One tracker across outline + scaffolding; merged into meta at the end.
	tracker := &engine.TokenTracker{}

	j.SetProgress(10, "生成大纲")
	// Reference corpus: workspace-collected books matching the archetype feed
	// the outline prompt. Empty corpus is fine — the prompt degrades gracefully.
	corpusBooks, err := corpus.List(s.root())
	if err != nil {
		return fmt.Errorf("load corpus: %w", err)
	}
	outlineCtx, cancelOutline := stageTimeoutCtx(ctx, ws.Config, "outline")
	outlineChatter := engine.NewTrackingChatter(deps.Chatter, tracker)
	outlineResult, err := outline.Generate(outlineCtx, outlineChatter, outline.Input{
		ArchetypeID: session.Answers["archetype"],
		Topic:       session.Answers["topic"],
		Audience:    session.Answers["audience"],
		Depth:       session.Answers["depth"],
		Goal:        session.Answers["goal"],
		Length:      session.Answers["length"],
		Language:    session.Answers["language"],
		CorpusBooks: corpusBooks,
	})
	cancelOutline()
	if err != nil {
		return fmt.Errorf("generate outline: %w", err)
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outlineResult); err != nil {
		return err
	}

	total := 0
	for _, p := range outlineResult.Parts {
		total += len(p.Chapters)
	}
	j.SetProgress(35, fmt.Sprintf("生成 %d 章框架", total))
	scaffCtx, cancelScaff := stageTimeoutCtx(ctx, ws.Config, "scaffolding")
	scaffChatter := engine.NewTrackingChatter(deps.Chatter, tracker)
	results := scaffolding.ScaffoldAll(scaffCtx, scaffChatter, outlineResult, session.Answers["archetype"],
		scaffolding.ChapterParams{
			Topic:    session.Answers["topic"],
			Audience: session.Answers["audience"],
			Depth:    session.Answers["depth"],
			Goal:     session.Answers["goal"],
			Length:   session.Answers["length"],
			Language: session.Answers["language"],
		},
		scaffolding.Options{},
	)
	cancelScaff()

	failed := 0
	for key, res := range results {
		if res.Err != nil {
			failed++
			j.Logf("框架失败 %s: %v", key, res.Err)
		}
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outlineResult); err != nil {
		return err
	}

	// Merge LLM usage into the book meta (this job is the sole writer).
	meta.TokenUsage.Add(tracker.Snapshot())
	meta.UpdatedAt = time.Now().UTC()
	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"), meta); err != nil {
		return err
	}

	repo := grill.NewRepository(s.root())
	if err := repo.Archive(session, slug); err != nil {
		j.Logf("归档会话失败（不阻塞）: %v", err)
	}

	j.SetResult("slug", slug)
	j.SetResult("chapters", total)
	j.SetResult("failed", failed)
	if failed > 0 {
		return fmt.Errorf("%d 个章节框架生成失败", failed)
	}
	return nil
}

// firstLine returns the first line of s (mirror of grill's recommendation parsing).
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
