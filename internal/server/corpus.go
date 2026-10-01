package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/corpus"
	"github.com/iannil/jianwu/internal/engine/collect"
	"github.com/iannil/jianwu/internal/provider/llmfactory"
	"github.com/iannil/jianwu/internal/storage"
	"github.com/iannil/jianwu/internal/workspace"
)

// corpusBookView is the list projection of a corpus book.
type corpusBookView struct {
	Slug      string `json:"slug"`
	TitleZh   string `json:"title_zh"`
	TitleEn   string `json:"title_en,omitempty"`
	Archetype string `json:"archetype,omitempty"`
	Origin    string `json:"origin"` // always "workspace" — there is no builtin corpus
	Parts     int    `json:"parts"`
	Chapters  int    `json:"chapters"`
}

// handleCorpusList returns all corpus books with their origin.
func (s *Server) handleCorpusList(w http.ResponseWriter, r *http.Request) {
	m, err := corpus.Load(s.root())
	if err != nil {
		failErr(w, err)
		return
	}
	out := make([]corpusBookView, 0, len(m))
	for _, slug := range sortedKeys(m) {
		b := m[slug]
		chapters := 0
		for _, p := range b.Parts {
			chapters += len(p.Chapters)
		}
		out = append(out, corpusBookView{
			Slug: b.Slug, TitleZh: b.Title.Zh, TitleEn: b.Title.En,
			Archetype: b.Archetype, Origin: "workspace",
			Parts: len(b.Parts), Chapters: chapters,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"books": out})
}

// handleCorpusStats returns aggregate corpus statistics.
func (s *Server) handleCorpusStats(w http.ResponseWriter, r *http.Request) {
	m, err := corpus.Load(s.root())
	if err != nil {
		failErr(w, err)
		return
	}
	totalParts, totalChapters := 0, 0
	archetypes := map[string]int{}
	for _, b := range m {
		totalParts += len(b.Parts)
		for _, p := range b.Parts {
			totalChapters += len(p.Chapters)
		}
		archetypes[b.Archetype]++
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"books":      len(m),
		"parts":      totalParts,
		"chapters":   totalChapters,
		"archetypes": archetypes,
	})
}

// handleCorpusShow returns one corpus book's full detail.
func (s *Server) handleCorpusShow(w http.ResponseWriter, r *http.Request) {
	m, err := corpus.Load(s.root())
	if err != nil {
		failErr(w, err)
		return
	}
	slug := r.PathValue("slug")
	b, ok := m[slug]
	if !ok {
		failf(w, http.StatusNotFound, "语料图书 %q 不存在", slug)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// handleCorpusSync copies corpus JSON files from a local directory into the
// workspace (mirror of cli corpus sync --from).
func (s *Server) handleCorpusSync(w http.ResponseWriter, r *http.Request) {
	var body struct {
		From string `json:"from"`
	}
	if err := decodeBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.From == "" {
		fail(w, http.StatusBadRequest, "from 为必填（语料 JSON 目录）")
		return
	}
	if !s.requireWorkspace(w) {
		return
	}
	if _, err := workspace.Load(s.root()); err != nil {
		failErr(w, err)
		return
	}
	info, err := storage.OS.Stat(body.From)
	if err != nil {
		failf(w, http.StatusBadRequest, "源路径: %v", err)
		return
	}
	if !info.IsDir() {
		failf(w, http.StatusBadRequest, "%q 不是目录", body.From)
		return
	}
	entries, err := storage.OS.ReadDir(body.From)
	if err != nil {
		failErr(w, err)
		return
	}
	synced, issues := 0, []string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := storage.OS.ReadFile(filepath.Join(body.From, e.Name()))
		if err != nil {
			issues = append(issues, fmt.Sprintf("%s: %v", e.Name(), err))
			continue
		}
		var b corpus.Book
		if err := json.Unmarshal(data, &b); err != nil {
			issues = append(issues, fmt.Sprintf("%s: %v", e.Name(), err))
			continue
		}
		if b.Slug == "" || (b.Title.Zh == "" && b.Title.En == "") {
			issues = append(issues, fmt.Sprintf("%s: 缺少 slug 或 title", e.Name()))
			continue
		}
		corpusDir := filepath.Join(s.root(), workspace.MarkerName, workspace.CorpusDirName)
		if err := storage.OS.MkdirAll(corpusDir, 0o755); err != nil {
			failErr(w, err)
			return
		}
		if err := storage.OS.WriteFile(filepath.Join(corpusDir, b.Slug+".json"), data, 0o644); err != nil {
			issues = append(issues, fmt.Sprintf("%s: %v", e.Name(), err))
			continue
		}
		synced++
	}
	writeJSON(w, http.StatusOK, map[string]any{"synced": synced, "issues": issues})
}

// handleCorpusCollect starts an auto-collect job: search the web, read pages,
// extract structured corpus books for the topic, save them into the workspace
// and best-effort rebuild the embedding index.
func (s *Server) handleCorpusCollect(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Topic    string `json:"topic"`
		Audience string `json:"audience"`
		Count    int    `json:"count"`
	}
	if err := decodeBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(body.Topic) == "" {
		fail(w, http.StatusBadRequest, "topic 为必填")
		return
	}
	if !s.requireWorkspace(w) {
		return
	}
	if _, err := workspace.Load(s.root()); err != nil {
		failErr(w, err)
		return
	}
	jobID := s.jobs.Start("corpus-collect", "", func(ctx context.Context, j *Job) error {
		return s.runCorpusCollect(ctx, j, body)
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID})
}

// runCorpusCollect mirrors cli corpus collect: resolve providers, run the
// collect engine, persist books, then rebuild the index (best-effort so a
// missing embedder key doesn't discard the collected corpus).
func (s *Server) runCorpusCollect(ctx context.Context, j *Job, body struct {
	Topic    string `json:"topic"`
	Audience string `json:"audience"`
	Count    int    `json:"count"`
}) error {
	deps, err := s.resolveDeps("scaffolding", needCollect)
	if err != nil {
		return err
	}
	res, err := collect.Run(ctx, collect.Deps{
		Chatter:  deps.Chatter,
		Searcher: deps.Searcher,
		Reader:   deps.Reader,
	}, collect.Input{Topic: body.Topic, Audience: body.Audience, Count: body.Count},
		func(p int, msg string) { j.SetProgress(p/2, msg) })
	if err != nil {
		return err
	}

	saved, skipped := 0, 0
	var slugs []string
	for _, b := range res.Books {
		if _, err := storage.OS.Stat(corpus.BookPath(s.root(), b.Slug)); err == nil {
			skipped++
			j.Logf("跳过 %s（已存在）", b.Slug)
			continue
		}
		if err := corpus.SaveBook(s.root(), b); err != nil {
			res.Issues = append(res.Issues, fmt.Sprintf("保存 %s 失败: %v", b.Slug, err))
			continue
		}
		saved++
		slugs = append(slugs, b.Slug)
		j.Logf("已保存 %s（%s，%d parts / %d 章）", b.Slug, b.Title.Zh, len(b.Parts), chapterCount(b))
	}
	for _, issue := range res.Issues {
		j.Logf("注意: %s", issue)
	}
	j.SetResult("saved", saved)
	j.SetResult("skipped", skipped)
	j.SetResult("slugs", slugs)
	if res.Usage.CallCount > 0 {
		j.SetResult("total_tokens", res.Usage.TotalTokens)
		j.SetResult("call_count", res.Usage.CallCount)
	}

	// Best-effort index rebuild: the corpus is saved either way; a failed
	// rebuild (e.g. no embedder key) only costs the similar-book lookup.
	j.SetProgress(60, "重建语料索引")
	if err := s.runCorpusReindex(ctx, j, ""); err != nil {
		j.Logf("索引重建跳过: %v（语料已保存，可稍后手动重建）", err)
	}
	j.SetProgress(100, fmt.Sprintf("采集完成：保存 %d，跳过 %d", saved, skipped))
	return nil
}

// chapterCount counts chapters across parts of a corpus book.
func chapterCount(b *corpus.Book) int {
	n := 0
	for _, p := range b.Parts {
		n += len(p.Chapters)
	}
	return n
}

// handleCorpusReindex starts a corpus embedding reindex job.
func (s *Server) handleCorpusReindex(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Model string `json:"model"`
	}
	if err := decodeBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.requireWorkspace(w) {
		return
	}
	if _, err := workspace.Load(s.root()); err != nil {
		failErr(w, err)
		return
	}
	modelOverride := body.Model
	jobID := s.jobs.Start("corpus-reindex", "", func(ctx context.Context, j *Job) error {
		return s.runCorpusReindex(ctx, j, modelOverride)
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID})
}

// runCorpusReindex mirrors cli corpus reindex.
func (s *Server) runCorpusReindex(ctx context.Context, j *Job, modelOverride string) error {
	ws, err := workspace.Load(s.root())
	if err != nil {
		return err
	}
	secrets, err := config.LoadSecrets()
	if err != nil {
		return fmt.Errorf("load secrets: %w", err)
	}
	m, err := corpus.Load(s.root())
	if err != nil {
		return fmt.Errorf("load corpus: %w", err)
	}
	if len(m) == 0 {
		return fmt.Errorf("没有可索引的语料图书")
	}
	j.Logf("载入 %d 本语料图书", len(m))

	modelRef := ws.Config.Models.Scaffolding
	if modelOverride != "" {
		modelRef.Model = modelOverride
	}
	if modelRef.Provider == "" {
		modelRef.Provider = "gemini"
		modelRef.Model = "gemini-2.5-flash"
	}
	embedder, err := llmfactory.NewEmbedder(modelRef, secrets)
	if err != nil {
		return fmt.Errorf("build embedder: %w", err)
	}
	modelName := modelRef.Model
	if modelName == "" {
		modelName = modelRef.Provider
	}
	j.SetProgress(20, fmt.Sprintf("Embedding %s/%s…", modelRef.Provider, modelRef.Model))
	idx, err := corpus.BuildIndex(ctx, embedder, modelName, m)
	if err != nil {
		return fmt.Errorf("build index: %w", err)
	}
	indexPath := filepath.Join(s.root(), workspace.MarkerName, "corpus_index.json")
	if err := corpus.SaveIndex(indexPath, idx); err != nil {
		return fmt.Errorf("save index: %w", err)
	}
	j.Logf("索引已保存（%d 本，dim=%d）", len(idx.Books), idx.Dim)
	j.SetResult("books", len(idx.Books))
	j.SetResult("dim", idx.Dim)
	return nil
}

// sortedKeys returns sorted string keys of a map.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
