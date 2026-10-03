package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/engine"
	"github.com/iannil/jianwu/internal/engine/expand"
	"github.com/iannil/jianwu/internal/engine/factcheck"
	"github.com/iannil/jianwu/internal/engine/revise"
	"github.com/iannil/jianwu/internal/export"
	"github.com/iannil/jianwu/internal/storage"
	"github.com/iannil/jianwu/internal/style"
)

// chapterView is the detail payload for one chapter: outline metadata plus
// the on-disk markdown body.
type chapterView struct {
	Part    int    `json:"part"`
	Chapter int    `json:"chapter"`
	Title   string `json:"title"`
	// From outline (source of truth for state).
	Abstract         string              `json:"abstract,omitempty"`
	KeyConcepts      []string            `json:"key_concepts,omitempty"`
	Status           string              `json:"status"`
	WordCount        int                 `json:"word_count"`
	CitationsCount   int                 `json:"citations_count"`
	UnverifiedClaims int                 `json:"unverified_claims"`
	Claims           []book.Claim        `json:"claims,omitempty"`
	Citations        []book.Citation     `json:"citations,omitempty"`
	Verdicts         []book.ClaimVerdict `json:"verdicts,omitempty"`
	ExpandedWith     *book.ExpandedWith  `json:"expanded_with,omitempty"`
	ReviewedAt       *time.Time          `json:"reviewed_at,omitempty"`
	ReviewedBy       string              `json:"reviewed_by,omitempty"`
	// From the .md file (present once expanded).
	Model string `json:"model,omitempty"`
	Body  string `json:"body,omitempty"`
	// BodyHTML is the goldmark render (same engine as EPUB/reading site,
	// duplicate leading title stripped) so reviewers preview exactly what
	// readers get; empty when rendering failed (client falls back).
	BodyHTML string `json:"body_html,omitempty"`
	FileOK   bool   `json:"file_ok"`
}

// handleChapterGet returns one chapter's outline metadata and markdown body.
func (s *Server) handleChapterGet(w http.ResponseWriter, r *http.Request) {
	bc, addr, ok := s.chapterFromPath(w, r)
	if !ok {
		return
	}
	ch, err := findChapter(bc.Outline, addr[0], addr[1])
	if err != nil {
		fail(w, http.StatusNotFound, err.Error())
		return
	}
	v := chapterView{
		Part: addr[0], Chapter: addr[1],
		Title: ch.Title, Abstract: ch.Abstract, KeyConcepts: ch.KeyConcepts,
		Status: ch.Status, WordCount: ch.WordCount,
		CitationsCount: ch.CitationsCount, UnverifiedClaims: ch.UnverifiedClaims,
		Claims: ch.Claims, Citations: ch.Citations, Verdicts: ch.Verdicts,
		ExpandedWith: ch.ExpandedWith, ReviewedAt: ch.ReviewedAt, ReviewedBy: ch.ReviewedBy,
	}
	if fm, body, err := book.ReadChapter(book.ChapterPath(bc.BookDir, addr[0], addr[1])); err == nil {
		v.FileOK = true
		v.Body = body
		v.Model = fm.Model
		if html, rerr := export.RenderXHTML(export.StripLeadingTitle(body, ch.Title)); rerr == nil {
			v.BodyHTML = html
		}
	}
	writeJSON(w, http.StatusOK, v)
}

// chapterFromPath resolves slug + part/ch path values to a loaded book.
func (s *Server) chapterFromPath(w http.ResponseWriter, r *http.Request) (*bookCtx, [2]int, bool) {
	slug := r.PathValue("slug")
	partIdx, err := pathInt(r, "part")
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return nil, [2]int{}, false
	}
	chIdx, err := pathInt(r, "ch")
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return nil, [2]int{}, false
	}
	bc, err := s.loadBook(slug)
	if err != nil {
		failErr(w, err)
		return nil, [2]int{}, false
	}
	return bc, [2]int{partIdx, chIdx}, true
}

// mirrorChapterStatus updates the status field of a chapter's .md frontmatter.
func mirrorChapterStatus(bookDir string, partIdx, chIdx int, status string) error {
	path := book.ChapterPath(bookDir, partIdx, chIdx)
	fm, body, err := book.ReadChapter(path)
	if err != nil {
		return fmt.Errorf("read chapter %02d-%02d: %w", partIdx, chIdx, err)
	}
	fm.Status = status
	if _, err := book.WriteChapter(bookDir, partIdx, chIdx, *fm, body); err != nil {
		return fmt.Errorf("write chapter %02d-%02d: %w", partIdx, chIdx, err)
	}
	return nil
}

// handleChapterReview marks an expanded chapter reviewed (human approval).
func (s *Server) handleChapterReview(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	bc, addr, ok := s.chapterFromPath(w, r)
	if !ok {
		return
	}
	ch, err := findChapter(bc.Outline, addr[0], addr[1])
	if err != nil {
		fail(w, http.StatusNotFound, err.Error())
		return
	}
	if ch.Status != book.StatusExpanded {
		failf(w, http.StatusConflict, "章节 %02d-%02d 状态为 %q；仅 expanded 章节可审阅", addr[0], addr[1], ch.Status)
		return
	}
	now := time.Now().UTC()
	ch.Status = book.StatusReviewed
	ch.ReviewedAt = &now
	ch.ReviewedBy = osUsername()
	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		failErr(w, err)
		return
	}
	if err := mirrorChapterStatus(bc.BookDir, addr[0], addr[1], book.StatusReviewed); err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "reviewed_by": ch.ReviewedBy})
}

// handleChapterAdd inserts a new scaffolded chapter (mirror of cli.add-chapter).
func (s *Server) handleChapterAdd(w http.ResponseWriter, r *http.Request) {
	var body struct {
		After string `json:"after"` // "NN-MM"
		Topic string `json:"topic"`
		As    string `json:"as"` // optional "NN-MM"
	}
	if err := decodeBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.After == "" {
		fail(w, http.StatusBadRequest, "after 为必填（如 01-02）")
		return
	}
	if body.Topic == "" {
		fail(w, http.StatusBadRequest, "topic 为必填")
		return
	}
	if !s.requireWorkspace(w) {
		return
	}
	afterPart, afterCh, err := parseChapterAddr(body.After)
	if err != nil {
		failf(w, http.StatusBadRequest, "after: %v", err)
		return
	}
	newPart, newCh := afterPart, afterCh+1
	if body.As != "" {
		np, nc, err := parseChapterAddr(body.As)
		if err != nil {
			failf(w, http.StatusBadRequest, "as: %v", err)
			return
		}
		if np != afterPart {
			failf(w, http.StatusBadRequest, "as 的 part %d 必须与 after 的 part %d 一致", np, afterPart)
			return
		}
		if nc <= afterCh {
			failf(w, http.StatusBadRequest, "as 的章号 %d 必须大于 after 的章号 %d", nc, afterCh)
			return
		}
		newPart, newCh = np, nc
	}

	slug := r.PathValue("slug")
	bc, err := s.loadBook(slug)
	if err != nil {
		failErr(w, err)
		return
	}
	part := findPartByIndex(bc.Outline, afterPart)
	if part == nil {
		failf(w, http.StatusNotFound, "part %d 不存在", afterPart)
		return
	}
	// Conflict: target address already taken.
	for _, c := range part.Chapters {
		if c.Index == newCh {
			failf(w, http.StatusConflict, "章节 %02d-%02d 已存在；用 as 指定其他地址", newPart, newCh)
			return
		}
	}
	insertIdx := -1
	for i, c := range part.Chapters {
		if c.Index == afterCh {
			insertIdx = i + 1
			break
		}
	}
	if insertIdx < 0 {
		failf(w, http.StatusNotFound, "章节 %s 不存在", body.After)
		return
	}

	part.Chapters = append(part.Chapters, book.OutlineChapter{})
	copy(part.Chapters[insertIdx+1:], part.Chapters[insertIdx:])
	part.Chapters[insertIdx] = book.OutlineChapter{Index: newCh, Title: body.Topic, Status: book.StatusScaffolded}

	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		failErr(w, err)
		return
	}
	fm := book.ChapterFrontmatter{
		Title: body.Topic, PartIndex: newPart, ChapterIndex: newCh,
		Status: book.StatusScaffolded, GeneratedAt: time.Now().UTC(), EngineVersion: s.version,
	}
	if _, err := book.WriteChapter(bc.BookDir, newPart, newCh, fm, ""); err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"part": newPart, "chapter": newCh, "title": body.Topic})
}

// handleChapterMove moves a chapter to another part (mirror of cli.move-chapter).
func (s *Server) handleChapterMove(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetPart int    `json:"target_part"`
		After      string `json:"after"` // optional "NN-MM" in target part
	}
	if err := decodeBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.requireWorkspace(w) {
		return
	}
	bc, addr, ok := s.chapterFromPath(w, r)
	if !ok {
		return
	}
	partIdx, chIdx := addr[0], addr[1]
	if body.TargetPart < 1 {
		fail(w, http.StatusBadRequest, "target_part 必填（≥1）")
		return
	}

	sourcePart := findPartByIndex(bc.Outline, partIdx)
	if sourcePart == nil {
		failf(w, http.StatusNotFound, "part %d 不存在", partIdx)
		return
	}
	found := -1
	var chCopy book.OutlineChapter
	for i := range sourcePart.Chapters {
		if sourcePart.Chapters[i].Index == chIdx {
			chCopy = sourcePart.Chapters[i]
			found = i
			break
		}
	}
	if found < 0 {
		failf(w, http.StatusNotFound, "章节 %02d-%02d 不存在", partIdx, chIdx)
		return
	}
	sourcePart.Chapters = append(sourcePart.Chapters[:found], sourcePart.Chapters[found+1:]...)

	targetPart := findPartByIndex(bc.Outline, body.TargetPart)
	if targetPart == nil {
		failf(w, http.StatusNotFound, "目标 part %d 不存在", body.TargetPart)
		return
	}
	newChIdx := chCopy.Index
	insertPos := len(targetPart.Chapters)
	if body.After != "" {
		ap, ac, err := parseChapterAddr(body.After)
		if err != nil {
			failf(w, http.StatusBadRequest, "after: %v", err)
			return
		}
		if ap != body.TargetPart {
			failf(w, http.StatusBadRequest, "after 的 part %d 必须与目标 part %d 一致", ap, body.TargetPart)
			return
		}
		newChIdx = ac + 1
		insertPos = -1
		for i, c := range targetPart.Chapters {
			if c.Index == ac {
				insertPos = i + 1
				break
			}
		}
		if insertPos < 0 {
			failf(w, http.StatusNotFound, "after 章节 %s 在目标 part 中不存在", body.After)
			return
		}
	}
	for _, c := range targetPart.Chapters {
		if c.Index == newChIdx {
			failf(w, http.StatusConflict, "目标 part %d 已有章节 %02d-%02d；用 after 指定其他位置", body.TargetPart, body.TargetPart, newChIdx)
			return
		}
	}

	chCopy.Index = newChIdx
	targetPart.Chapters = append(targetPart.Chapters, book.OutlineChapter{})
	copy(targetPart.Chapters[insertPos+1:], targetPart.Chapters[insertPos:])
	targetPart.Chapters[insertPos] = chCopy

	oldPath := book.ChapterPath(bc.BookDir, partIdx, chIdx)
	fm, bodyMD, rerr := book.ReadChapter(oldPath)
	if rerr == nil {
		fm.PartIndex = body.TargetPart
		fm.ChapterIndex = newChIdx
		fm.GeneratedAt = time.Now().UTC()
		if _, err := book.WriteChapter(bc.BookDir, body.TargetPart, newChIdx, *fm, bodyMD); err != nil {
			failErr(w, err)
			return
		}
		if err := storage.OS.RemoveAll(oldPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: remove old chapter file: %v\n", err)
		}
	} else {
		stub := book.ChapterFrontmatter{
			Title: chCopy.Title, PartIndex: body.TargetPart, ChapterIndex: newChIdx,
			Status: chCopy.Status, GeneratedAt: time.Now().UTC(), EngineVersion: s.version,
		}
		if _, err := book.WriteChapter(bc.BookDir, body.TargetPart, newChIdx, stub, ""); err != nil {
			failErr(w, err)
			return
		}
	}
	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"part": body.TargetPart, "chapter": newChIdx})
}

// handleChapterDelete removes a chapter from outline and disk.
func (s *Server) handleChapterDelete(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	bc, addr, ok := s.chapterFromPath(w, r)
	if !ok {
		return
	}
	part := findPartByIndex(bc.Outline, addr[0])
	if part == nil {
		failf(w, http.StatusNotFound, "part %d 不存在", addr[0])
		return
	}
	found := false
	for i, c := range part.Chapters {
		if c.Index == addr[1] {
			part.Chapters = append(part.Chapters[:i], part.Chapters[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		failf(w, http.StatusNotFound, "章节 %02d-%02d 不存在", addr[0], addr[1])
		return
	}
	chapPath := book.ChapterPath(bc.BookDir, addr[0], addr[1])
	if err := storage.OS.RemoveAll(chapPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: delete chapter file: %v\n", err)
	}
	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// parseChapterAddr parses "NN-MM" into (part, ch), both 1-based.
func parseChapterAddr(s string) (int, int, error) {
	if s == "" {
		return 0, 0, fmt.Errorf("空章节地址")
	}
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("章节地址 %q 格式应为 NN-MM", s)
	}
	p, c := 0, 0
	if _, err := fmt.Sscanf(parts[0], "%d", &p); err != nil || p < 1 {
		return 0, 0, fmt.Errorf("无效 part %q", parts[0])
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &c); err != nil || c < 1 {
		return 0, 0, fmt.Errorf("无效章节号 %q", parts[1])
	}
	return p, c, nil
}

// osUsername returns the current OS username, or "" if undeterminable.
func osUsername() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return ""
}

// handleChapterFactcheck starts a factcheck job for one chapter.
func (s *Server) handleChapterFactcheck(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	bc, addr, ok := s.chapterFromPath(w, r)
	if !ok {
		return
	}
	ch, err := findChapter(bc.Outline, addr[0], addr[1])
	if err != nil {
		fail(w, http.StatusNotFound, err.Error())
		return
	}
	if ch.Status != book.StatusExpanded && ch.Status != book.StatusReviewed {
		failf(w, http.StatusConflict, "章节 %02d-%02d 状态为 %q；仅 expanded/reviewed 章节可事实核查", addr[0], addr[1], ch.Status)
		return
	}
	if len(ch.Claims) == 0 {
		fail(w, http.StatusConflict, "章节没有可核查的 claims；请先 expand")
		return
	}
	if _, err := s.resolveDeps("expand", needChatterReader); err != nil {
		failErr(w, err)
		return
	}
	slug := r.PathValue("slug")
	p, c := addr[0], addr[1]
	jobID := s.jobs.Start("factcheck", slug, func(ctx context.Context, j *Job) error {
		return s.runFactcheck(ctx, j, slug, p, c)
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID})
}

// runFactcheck mirrors cli.runFactCheckWithDeps: verify claims against cited
// sources and persist verdicts to outline.json.
func (s *Server) runFactcheck(ctx context.Context, j *Job, slug string, partIdx, chIdx int) error {
	bc, err := s.loadBook(slug)
	if err != nil {
		return err
	}
	deps, err := s.resolveDeps("expand", needChatterReader)
	if err != nil {
		return err
	}
	ch, err := findChapter(bc.Outline, partIdx, chIdx)
	if err != nil {
		return err
	}
	j.Logf("事实核查 %02d-%02d（%d 条论断）…", partIdx, chIdx, len(ch.Claims))

	tracker := &engine.TokenTracker{}
	tracked := *deps
	tracked.Chatter = engine.NewTrackingChatter(deps.Chatter, tracker)

	result, err := factcheck.Run(ctx, tracked.Chatter, tracked.Reader, factcheck.Input{
		ChapterTitle: ch.Title,
		Claims:       ch.Claims,
		Citations:    ch.Citations,
	})
	if err := persistUsage(bc.BookDir, bc.Meta, tracker.Snapshot()); err != nil {
		return err
	}
	if err != nil {
		return fmt.Errorf("factcheck: %w", err)
	}

	ch.Verdicts = make([]book.ClaimVerdict, len(result.Verdicts))
	verified := 0
	for i, v := range result.Verdicts {
		ch.Verdicts[i] = book.ClaimVerdict{
			ClaimText: v.ClaimText, Verified: v.Verified, Reasoning: v.Reasoning,
			SuggestedRewrite: v.SuggestedRewrite, CitationID: v.CitationID,
		}
		if v.Verified {
			verified++
		}
	}
	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		return err
	}
	for _, v := range result.Verdicts {
		mark := "✓"
		if !v.Verified {
			mark = "✗"
		}
		j.Logf("%s %s", mark, truncateText(v.ClaimText, 60))
	}
	for _, u := range result.SourceErrors {
		j.Logf("⚠ 来源读取失败: %s", u)
	}
	j.Logf("核查完成：%d/%d 通过", verified, len(result.Verdicts))
	j.SetResult("verified", verified)
	j.SetResult("total", len(result.Verdicts))
	j.SetResult("source_errors", len(result.SourceErrors))
	return nil
}

// handleChapterRevise starts a revise job for one chapter.
func (s *Server) handleChapterRevise(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	bc, addr, ok := s.chapterFromPath(w, r)
	if !ok {
		return
	}
	ch, err := findChapter(bc.Outline, addr[0], addr[1])
	if err != nil {
		fail(w, http.StatusNotFound, err.Error())
		return
	}
	if ch.Status != book.StatusExpanded && ch.Status != book.StatusReviewed {
		failf(w, http.StatusConflict, "章节 %02d-%02d 状态为 %q；仅 expanded/reviewed 章节可修订", addr[0], addr[1], ch.Status)
		return
	}
	if _, err := s.resolveDeps("expand", needChatterOnly); err != nil {
		failErr(w, err)
		return
	}
	slug := r.PathValue("slug")
	p, c := addr[0], addr[1]
	jobID := s.jobs.Start("revise", slug, func(ctx context.Context, j *Job) error {
		return s.runRevise(ctx, j, slug, p, c)
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID})
}

// runRevise mirrors cli.runReviseWithDeps: LLM revision driven by verdicts,
// re-validated, claims/citations rebuilt, old review + verdicts revoked.
func (s *Server) runRevise(ctx context.Context, j *Job, slug string, partIdx, chIdx int) error {
	bc, err := s.loadBook(slug)
	if err != nil {
		return err
	}
	deps, err := s.resolveDeps("expand", needChatterOnly)
	if err != nil {
		return err
	}
	ch, err := findChapter(bc.Outline, partIdx, chIdx)
	if err != nil {
		return err
	}
	chapPath := book.ChapterPath(bc.BookDir, partIdx, chIdx)
	_, bodyMD, err := book.ReadChapter(chapPath)
	if err != nil {
		return fmt.Errorf("read chapter: %w", err)
	}

	j.Logf("修订 %02d-%02d…", partIdx, chIdx)
	tracker := &engine.TokenTracker{}
	chatter := engine.NewTrackingChatter(deps.Chatter, tracker)

	result, err := revise.Run(ctx, chatter, revise.Input{
		ChapterTitle: ch.Title,
		Markdown:     bodyMD,
		Citations:    ch.Citations,
		Unverified:   ch.Claims,
		Verdicts:     ch.Verdicts,
	})
	if err != nil {
		if perr := persistUsage(bc.BookDir, bc.Meta, tracker.Snapshot()); perr != nil {
			err = errors.Join(err, perr)
		}
		return fmt.Errorf("revise: %w", err)
	}
	if strings.TrimSpace(result.RevisedMarkdown) == "" {
		return errors.Join(fmt.Errorf("修订返回空正文"), persistErr(bc, tracker))
	}

	// The post-revise validation rewrite must run under the same style guide
	// as the main expand pipeline, or it strips prose quality (P2 fix).
	guide, gerr := style.LoadGuide()
	if gerr != nil {
		slog.Warn("style guide unavailable, revising without it", "err", gerr)
		guide = ""
	}
	validated, err := expand.RunValidate(ctx, chatter, result.RevisedMarkdown, expand.ResearchNotes{}, guide, nil)
	if err != nil {
		if perr := persistUsage(bc.BookDir, bc.Meta, tracker.Snapshot()); perr != nil {
			err = errors.Join(err, perr)
		}
		return fmt.Errorf("validate revision: %w", err)
	}
	if validated.RevisedMarkdown != "" {
		result.RevisedMarkdown = validated.RevisedMarkdown
	}
	if strings.TrimSpace(result.RevisedMarkdown) == "" {
		return errors.Join(fmt.Errorf("校验后正文为空"), persistErr(bc, tracker))
	}
	usage := tracker.Snapshot()

	err = s.withBookRollback(bc, partIdx, chIdx, func() error {
		return s.applyRevision(bc, chapPath, partIdx, chIdx, result.RevisedMarkdown, &validated)
	})
	if perr := persistUsage(bc.BookDir, bc.Meta, usage); perr != nil {
		err = errors.Join(err, perr)
	}
	if err != nil {
		return err
	}

	ch, _ = findChapter(bc.Outline, partIdx, chIdx)
	j.Logf("修订完成：%d 字；请再次 factcheck 并人工 review", ch.WordCount)
	j.SetResult("word_count", ch.WordCount)
	return nil
}

// persistErr is a helper that persists usage and joins any error into nil-able err chains.
func persistErr(bc *bookCtx, tracker *engine.TokenTracker) error {
	return persistUsage(bc.BookDir, bc.Meta, tracker.Snapshot())
}

// applyRevision writes the revised markdown and updates outline state
// (mirror of the mutation block in cli.runReviseWithDeps).
func (s *Server) applyRevision(bc *bookCtx, chapPath string, partIdx, chIdx int, markdown string, validated *expand.ValidationResult) error {
	ch, err := findChapter(bc.Outline, partIdx, chIdx)
	if err != nil {
		return err
	}
	ch.Claims = nil
	ch.UnverifiedClaims = 0
	for _, c := range validated.Claims {
		ch.Claims = append(ch.Claims, book.Claim{Text: c.Text, HasCitation: c.HasCitation, CitationIDs: c.CitationIDs})
		if !c.HasCitation {
			ch.UnverifiedClaims++
		}
	}
	oldCitations := map[string]book.Citation{}
	for _, c := range ch.Citations {
		oldCitations[c.ID] = c
	}
	ch.Citations = nil
	defs := expand.ParseFootnotes(markdown)
	ids := make([]string, 0, len(defs))
	for id := range defs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		d := defs[id]
		c := oldCitations[id]
		if c.URL != d.URL {
			c = book.Citation{}
		}
		c.ID, c.URL, c.Title = id, d.URL, d.Title
		ch.Citations = append(ch.Citations, c)
	}
	ch.CitationsCount = len(ch.Citations)
	ch.Status = book.StatusExpanded
	ch.ReviewedAt = nil
	ch.ReviewedBy = ""
	ch.Verdicts = nil

	// Backfill footnote dates from the (rebuilt) citation metadata; the
	// revise LLM invents the "accessed DATE" tail (P3 fix).
	accessed := make(map[string]time.Time, len(ch.Citations))
	for _, c := range ch.Citations {
		accessed[c.URL] = c.AccessedAt
	}
	markdown = book.NormalizeFootnoteDates(markdown, accessed)

	existingFM, _, readErr := book.ReadChapter(chapPath)
	var fm book.ChapterFrontmatter
	if readErr == nil && existingFM != nil {
		fm = *existingFM
	}
	fm.Title = ch.Title
	fm.PartIndex = partIdx
	fm.ChapterIndex = chIdx
	fm.Status = ch.Status
	oldFMCitations := map[string]book.ChapterCitation{}
	for _, c := range fm.Citations {
		oldFMCitations[c.ID] = c
	}
	fm.Citations = nil
	for _, c := range ch.Citations {
		entry := oldFMCitations[c.ID]
		if entry.URL != c.URL {
			entry = book.ChapterCitation{}
		}
		entry.ID, entry.URL, entry.Title = c.ID, c.URL, c.Title
		fm.Citations = append(fm.Citations, entry)
	}
	fm.UnverifiedClaimsCount = ch.UnverifiedClaims
	fm.WordCount = roughWordCount(markdown)
	fm.GeneratedAt = time.Now().UTC()
	if _, err := book.WriteChapter(bc.BookDir, partIdx, chIdx, fm, markdown); err != nil {
		return err
	}
	ch.WordCount = fm.WordCount
	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		return err
	}
	bc.Meta.Status = book.BookStatusDraft
	bc.Meta.UpdatedAt = time.Now().UTC()
	return book.SaveMeta(filepath.Join(bc.BookDir, "meta.json"), bc.Meta)
}

// roughWordCount approximates word count for CJK + latin text.
func roughWordCount(s string) int {
	words := strings.Fields(s)
	count := len(words)
	for _, w := range words {
		for _, rr := range w {
			if rr >= 0x4E00 && rr <= 0x9FFF {
				count++
			}
		}
	}
	return count
}

// truncateText truncates s to n runes with an ellipsis.
func truncateText(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
