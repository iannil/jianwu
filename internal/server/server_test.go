package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/engine/grill"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/reader"
	"github.com/iannil/jianwu/internal/provider/search"
	"github.com/iannil/jianwu/internal/workspace"
)

// --- test doubles ---

// scriptedChatter returns preset responses in order, cycling the last one.
type scriptedChatter struct {
	mu        sync.Mutex
	responses []llm.ChatResponse
	calls     int
}

func (s *scriptedChatter) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.calls
	s.calls++
	if i >= len(s.responses) {
		i = len(s.responses) - 1
	}
	resp := s.responses[i]
	resp.PopulateUsage()
	return &resp, nil
}

type fakeSearcher struct {
	mu    sync.Mutex
	calls int
}

func (f *fakeSearcher) Search(ctx context.Context, query string, opts search.SearchOpts) ([]search.SearchResult, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return []search.SearchResult{
		{Title: "Result A", URL: "https://example.com/a", Snippet: "snippet a"},
	}, nil
}

// stageChatter classifies each request by shape and answers per stage:
// research (schema with findings), validate (schema with revised_markdown),
// draft (no schema). Safe under concurrency, unlike scriptedChatter.
type stageChatter struct {
	research llm.ChatResponse
	draft    llm.ChatResponse
	validate llm.ChatResponse
}

func (s *stageChatter) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	schema := string(req.JSONSchema)
	var resp llm.ChatResponse
	switch {
	case schema == "":
		resp = s.draft
	case strings.Contains(schema, "revised_markdown"):
		resp = s.validate
	default:
		resp = s.research
	}
	resp.PopulateUsage()
	return &resp, nil
}

type fakeReader struct{}

func (f *fakeReader) Read(ctx context.Context, url string) (reader.Content, error) {
	return reader.Content{URL: url, Title: "Page", Markdown: "page content"}, nil
}

// --- helpers ---

const testOutlineJSON = `{"parts":[{"index":1,"title":"P1","role":"ontology","chapters":[{"index":1,"title":"C1","status":"scaffolded","abstract":"A","key_concepts":["k1"]}]}]}`

const testScaffoldJSON = `{"abstract":"abstract","key_concepts":["k"],"learning_objectives":["lo"],"suggested_examples":["ex"]}`

const testDraftMD = "## C1\n\nBody text with a claim.[^1]\n\n[^1]: [Example](https://example.com/a) accessed 2026-10-01"

const testValidateJSON = `{"revised_markdown":"## C1\n\nBody text with a claim.[^1]\n\n[^1]: [Example](https://example.com/a) accessed 2026-10-01","claims":[{"text":"Body text with a claim.","has_citation":true}]}`

// newTestEnv creates an initialized workspace + server with scripted deps.
func newTestEnv(t *testing.T) (*Server, *scriptedChatter) {
	t.Helper()
	root := t.TempDir()
	if err := workspace.Init(root, workspace.InitOpts{}); err != nil {
		t.Fatalf("workspace init: %v", err)
	}
	return newTestEnvAt(t, root)
}

// newTestEnvAt creates a server on the given (possibly uninitialized) root.
func newTestEnvAt(t *testing.T, root string) (*Server, *scriptedChatter) {
	t.Helper()
	chat := &scriptedChatter{}
	deps := &Deps{Chatter: chat, Searcher: &fakeSearcher{}, Reader: &fakeReader{}}
	return New(root, "test", deps), chat
}

// SetGlobalConfigPathForTest points the global config file at a temp path and
// restores the default afterwards.
func SetGlobalConfigPathForTest(t *testing.T, path string) {
	t.Helper()
	prev := os.Getenv("HOME")
	config.SetGlobalConfigPath(path)
	t.Cleanup(func() {
		config.SetGlobalConfigPath(filepath.Join(prev, ".config", "jianwu", "config.yaml"))
	})
}

// do executes one HTTP request against the server handler.
func do(t *testing.T, srv *Server, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var rd *strings.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rd = strings.NewReader(string(data))
	} else {
		rd = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func wantCode(t *testing.T, rec *httptest.ResponseRecorder, code int) {
	t.Helper()
	if rec.Code != code {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, code, rec.Body.String())
	}
}

// createBookFixture writes meta + outline with n scaffolded chapters.
func createBookFixture(t *testing.T, root, slug string, chapters int) string {
	t.Helper()
	bookDir := filepath.Join(root, "books", slug)
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := &book.Meta{
		ID: "id-" + slug, Slug: slug, Title: "Fixture " + slug,
		Archetype: "ontology-epistemology-practice", Language: "zh",
		Status:     book.BookStatusDraft,
		Parameters: book.Parameters{Audience: "educated-general", Depth: "intermediate", Goal: "understanding", Length: "medium"},
	}
	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"), meta); err != nil {
		t.Fatal(err)
	}
	chs := make([]book.OutlineChapter, chapters)
	for i := range chs {
		chs[i] = book.OutlineChapter{
			Index: i + 1, Title: fmt.Sprintf("C%d", i+1), Status: book.StatusScaffolded,
			Abstract: "abstract", KeyConcepts: []string{"k"},
		}
	}
	outline := &book.Outline{Parts: []book.OutlinePart{{Index: 1, Title: "P1", Role: "ontology", Chapters: chs}}}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
		t.Fatal(err)
	}
	return bookDir
}

// waitJob polls a job until it leaves running state, then returns its view.
func waitJob(t *testing.T, srv *Server, jobID string) *jobView {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := srv.jobs.Get(jobID)
		if ok && job.Status != JobRunning {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s did not finish in time", jobID)
	return nil
}

// --- tests ---

func TestWorkspaceLifecycle(t *testing.T) {
	root := t.TempDir()
	srv := New(root, "test", nil)

	rec, ws := do(t, srv, "GET", "/api/v1/workspace", nil)
	wantCode(t, rec, http.StatusOK)
	if ws["initialized"] != false {
		t.Fatalf("initialized = %v, want false", ws["initialized"])
	}

	rec, _ = do(t, srv, "POST", "/api/v1/workspace/init", map[string]any{})
	wantCode(t, rec, http.StatusCreated)

	rec, ws = do(t, srv, "GET", "/api/v1/workspace", nil)
	wantCode(t, rec, http.StatusOK)
	if ws["initialized"] != true {
		t.Fatalf("initialized = %v, want true", ws["initialized"])
	}

	rec, cfg := do(t, srv, "GET", "/api/v1/config", nil)
	wantCode(t, rec, http.StatusOK)
	models, ok := cfg["models"].(map[string]any)
	if !ok || models["expand"] == nil {
		t.Fatalf("config models missing: %v", cfg)
	}

	// Re-init conflicts.
	rec, _ = do(t, srv, "POST", "/api/v1/workspace/init", map[string]any{})
	wantCode(t, rec, http.StatusConflict)
}

func TestGrillFlowGeneratesBook(t *testing.T) {
	srv, chat := newTestEnv(t)

	// 10 recommendations (topic pre-answered) + outline + scaffold.
	recs := []llm.ChatResponse{
		{Content: "educated-general\n为受过教育的普通读者"},
		{Content: "understanding\n以理解为目标"},
		{Content: "ontology-epistemology-practice\n本体-认识-实践结构"},
		{Content: "intermediate\n中级深度"},
		{Content: "medium\n中等篇幅"},
		{Content: "zh\n中文"},
		{Content: "single\n单本"},
		{Content: "mixed\n混合例子"},
		{Content: "tables\n表格"},
		{Content: "timeless\n永恒取向"},
		{Content: testOutlineJSON},
		{Content: testScaffoldJSON},
	}
	for _, r := range recs {
		chat.responses = append(chat.responses, r)
	}

	// Start session with topic.
	rec, sess := do(t, srv, "POST", "/api/v1/grill/sessions", map[string]any{"topic": "时间的本质"})
	wantCode(t, rec, http.StatusCreated)
	if sess["complete"] != false {
		t.Fatalf("expected incomplete session, got %v", sess)
	}
	dim, _ := sess["dimension"].(map[string]any)
	if dim == nil || dim["id"] != "audience" {
		t.Fatalf("first pending dim = %v, want audience", sess["dimension"])
	}
	sessionID, _ := sess["session_id"].(string)

	// Answer all dimensions with empty input (accept recommendation).
	for i := 0; i < 12; i++ {
		rec, resp := do(t, srv, "POST", "/api/v1/grill/sessions/"+sessionID+"/answer", map[string]any{"answer": ""})
		wantCode(t, rec, http.StatusOK)
		if resp["complete"] == true {
			break
		}
		if resp["dimension"] == nil {
			t.Fatalf("answer %d: no dimension and not complete: %v", i, resp)
		}
	}

	// Session must now be complete.
	rec, sess = do(t, srv, "GET", "/api/v1/grill/sessions/"+sessionID, nil)
	wantCode(t, rec, http.StatusOK)
	if sess["complete"] != true {
		t.Fatalf("session not complete: %v", sess)
	}

	// Generate the book.
	rec, gen := do(t, srv, "POST", "/api/v1/grill/sessions/"+sessionID+"/generate", map[string]any{})
	wantCode(t, rec, http.StatusAccepted)
	jobID, _ := gen["job_id"].(string)
	job := waitJob(t, srv, jobID)
	if job.Status != JobSucceeded {
		t.Fatalf("generate job failed: %s\nlog: %s", job.Err, job.Log)
	}

	// Book exists with meta + outline (chapter .md files appear on expand).
	slug, _ := gen["slug"].(string)
	bookDir := filepath.Join(srv.WSRoot(), "books", slug)
	for _, name := range []string{"meta.json", "outline.json"} {
		if _, err := os.Stat(filepath.Join(bookDir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	meta, err := book.LoadMeta(filepath.Join(bookDir, "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	if meta.TokenUsage.CallCount == 0 {
		t.Errorf("expected token usage recorded from grill+outline+scaffold calls")
	}
	// Session archived out of the active dir.
	repo := grill.NewRepository(srv.WSRoot())
	incomplete, err := repo.ListIncomplete()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range incomplete {
		if s.ID == sessionID {
			t.Errorf("session %s should have been archived", sessionID)
		}
	}
}

func TestGrillAnswerInvalidOptionFallsBackToDefault(t *testing.T) {
	srv, chat := newTestEnv(t)
	chat.responses = []llm.ChatResponse{
		{Content: "scholar\n学术读者"}, // recommendation for audience
		{Content: "understanding\n…"},
	}
	rec, sess := do(t, srv, "POST", "/api/v1/grill/sessions", map[string]any{"topic": "测试主题"})
	wantCode(t, rec, http.StatusCreated)
	sessionID, _ := sess["session_id"].(string)

	// audience recommendation is "scholar"; send a bogus answer → default fallback.
	rec, resp := do(t, srv, "POST", "/api/v1/grill/sessions/"+sessionID+"/answer", map[string]any{"answer": "not-a-valid-option"})
	wantCode(t, rec, http.StatusOK)
	answers, _ := resp["answers"].(map[string]any)
	if answers["audience"] != "educated-general" {
		t.Fatalf("audience = %v, want default educated-general", answers["audience"])
	}
}

func TestExpandChapterLifecycle(t *testing.T) {
	srv, chat := newTestEnv(t)
	createBookFixture(t, srv.WSRoot(), "life", 1)
	chat.responses = []llm.ChatResponse{
		{Content: `{"findings":[],"candidates":[]}`}, // research
		{Content: testDraftMD},                       // draft
		{Content: testValidateJSON},                  // validate
	}

	// Unknown book → 404.
	rec, _ := do(t, srv, "GET", "/api/v1/books/nope", nil)
	wantCode(t, rec, http.StatusNotFound)

	// Expand.
	rec, res := do(t, srv, "POST", "/api/v1/books/life/chapters/1/1/expand", map[string]any{})
	wantCode(t, rec, http.StatusAccepted)
	job := waitJob(t, srv, res["job_id"].(string))
	if job.Status != JobSucceeded {
		t.Fatalf("expand job failed: %s\nlog: %s", job.Err, job.Log)
	}

	// Chapter file + outline updated.
	fm, body, err := book.ReadChapter(filepath.Join(srv.WSRoot(), "books", "life", "chapters", "01-01.md"))
	if err != nil {
		t.Fatal(err)
	}
	if fm.Status != book.StatusExpanded || !strings.Contains(body, "Body text") {
		t.Fatalf("chapter not expanded: status=%q body=%q", fm.Status, body[:30])
	}
	outline, err := book.LoadOutline(filepath.Join(srv.WSRoot(), "books", "life", "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	if outline.Parts[0].Chapters[0].Status != book.StatusExpanded {
		t.Fatalf("outline status = %q", outline.Parts[0].Chapters[0].Status)
	}
	if len(outline.Parts[0].Chapters[0].Citations) != 1 {
		t.Fatalf("citations = %d, want 1", len(outline.Parts[0].Chapters[0].Citations))
	}

	// Chapter detail endpoint.
	rec, ch := do(t, srv, "GET", "/api/v1/books/life/chapters/1/1", nil)
	wantCode(t, rec, http.StatusOK)
	if ch["file_ok"] != true {
		t.Fatalf("file_ok = %v", ch["file_ok"])
	}

	// Review → finalize → export.
	rec, _ = do(t, srv, "POST", "/api/v1/books/life/chapters/1/1/review", map[string]any{})
	wantCode(t, rec, http.StatusOK)

	rec, _ = do(t, srv, "POST", "/api/v1/books/life/finalize", map[string]any{"dry_run": true})
	wantCode(t, rec, http.StatusOK)

	rec, _ = do(t, srv, "POST", "/api/v1/books/life/finalize", map[string]any{})
	wantCode(t, rec, http.StatusOK)

	rec, res = do(t, srv, "POST", "/api/v1/books/life/export", map[string]any{"target": "md"})
	wantCode(t, rec, http.StatusAccepted)
	job = waitJob(t, srv, res["job_id"].(string))
	if job.Status != JobSucceeded {
		t.Fatalf("export job failed: %s", job.Err)
	}

	req := httptest.NewRequest("GET", "/api/v1/books/life/export/file?target=md", nil)
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK || !strings.Contains(rec2.Body.String(), "Body text") {
		t.Fatalf("export download: code=%d body=%q", rec2.Code, rec2.Body.String())
	}
}

func TestExpandAllBatch(t *testing.T) {
	root := t.TempDir()
	if err := workspace.Init(root, workspace.InitOpts{}); err != nil {
		t.Fatal(err)
	}
	// Concurrent batch: classify responses per stage instead of scripted order.
	deps := &Deps{
		Chatter: &stageChatter{
			research: llm.ChatResponse{Content: `{"findings":[],"candidates":[]}`},
			draft:    llm.ChatResponse{Content: testDraftMD},
			validate: llm.ChatResponse{Content: testValidateJSON},
		},
		Searcher: &fakeSearcher{},
		Reader:   &fakeReader{},
	}
	srv := New(root, "test", deps)
	createBookFixture(t, srv.WSRoot(), "batch", 2)

	rec, res := do(t, srv, "POST", "/api/v1/books/batch/expand-all", map[string]any{})
	wantCode(t, rec, http.StatusAccepted)
	job := waitJob(t, srv, res["job_id"].(string))
	if job.Status != JobSucceeded {
		t.Fatalf("expand-all job failed: %s\nlog: %s", job.Err, job.Log)
	}
	outline, err := book.LoadOutline(filepath.Join(srv.WSRoot(), "books", "batch", "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range outline.Parts[0].Chapters {
		if c.Status != book.StatusExpanded {
			t.Fatalf("chapter %d status = %q, want expanded", c.Index, c.Status)
		}
	}
	if got := job.Result["succeeded"].(int); got != 2 {
		t.Fatalf("succeeded = %v, want 2", job.Result["succeeded"])
	}
}

func TestFactcheckUnlinkedClaim(t *testing.T) {
	srv, chat := newTestEnv(t)
	createBookFixture(t, srv.WSRoot(), "fc", 1)
	chat.responses = []llm.ChatResponse{
		{Content: `{"findings":[],"candidates":[]}`},
		{Content: testDraftMD},
		{Content: testValidateJSON}, // claim without citation_ids → unlinked
	}
	rec, res := do(t, srv, "POST", "/api/v1/books/fc/chapters/1/1/expand", map[string]any{})
	wantCode(t, rec, http.StatusAccepted)
	waitJob(t, srv, res["job_id"].(string))

	rec, res = do(t, srv, "POST", "/api/v1/books/fc/chapters/1/1/factcheck", map[string]any{})
	wantCode(t, rec, http.StatusAccepted)
	job := waitJob(t, srv, res["job_id"].(string))
	if job.Status != JobSucceeded {
		t.Fatalf("factcheck job failed: %s", job.Err)
	}
	outline, err := book.LoadOutline(filepath.Join(srv.WSRoot(), "books", "fc", "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	verdicts := outline.Parts[0].Chapters[0].Verdicts
	if len(verdicts) != 1 || verdicts[0].Verified {
		t.Fatalf("verdicts = %+v, want one unverified", verdicts)
	}
}

func TestReviseRevokesReview(t *testing.T) {
	srv, chat := newTestEnv(t)
	createBookFixture(t, srv.WSRoot(), "rv", 1)
	chat.responses = []llm.ChatResponse{
		{Content: `{"findings":[],"candidates":[]}`},
		{Content: testDraftMD},
		{Content: testValidateJSON},
		// revise: plain markdown revision, then validate JSON.
		{Content: "## C1\n\nRevised body.[^1]\n\n[^1]: [Example](https://example.com/a) accessed 2026-10-01"},
		{Content: testValidateJSON},
	}
	rec, res := do(t, srv, "POST", "/api/v1/books/rv/chapters/1/1/expand", map[string]any{})
	wantCode(t, rec, http.StatusAccepted)
	waitJob(t, srv, res["job_id"].(string))

	rec, _ = do(t, srv, "POST", "/api/v1/books/rv/chapters/1/1/review", map[string]any{})
	wantCode(t, rec, http.StatusOK)

	rec, res = do(t, srv, "POST", "/api/v1/books/rv/chapters/1/1/revise", map[string]any{})
	wantCode(t, rec, http.StatusAccepted)
	job := waitJob(t, srv, res["job_id"].(string))
	if job.Status != JobSucceeded {
		t.Fatalf("revise job failed: %s\nlog: %s", job.Err, job.Log)
	}

	outline, err := book.LoadOutline(filepath.Join(srv.WSRoot(), "books", "rv", "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	ch := outline.Parts[0].Chapters[0]
	if ch.Status != book.StatusExpanded {
		t.Fatalf("status after revise = %q, want expanded (review revoked)", ch.Status)
	}
	if ch.ReviewedAt != nil {
		t.Fatalf("reviewed_at should be nil after revise")
	}
}

func TestJobQueueSerialization(t *testing.T) {
	m := NewJobManager()
	var mu sync.Mutex
	concurrent, maxConcurrent := 0, 0
	worker := func(ctx context.Context, j *Job) error {
		mu.Lock()
		concurrent++
		if concurrent > maxConcurrent {
			maxConcurrent = concurrent
		}
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		concurrent--
		mu.Unlock()
		return nil
	}
	m.Start("a", "", worker)
	m.Start("b", "", worker)
	m.Start("c", "", worker)
	m.waitIdle()
	if maxConcurrent != 1 {
		t.Fatalf("max concurrent jobs = %d, want 1 (serialized)", maxConcurrent)
	}
}

func TestSPAAndUnknownRoutes(t *testing.T) {
	srv, _ := newTestEnv(t)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "肩吾") {
		t.Fatalf("SPA index: code=%d", rec.Code)
	}
	// Client-side route falls back to index.
	req = httptest.NewRequest("GET", "/books", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("SPA fallback: code=%d", rec.Code)
	}
	// Unknown API route → 404 from mux.
	req = httptest.NewRequest("GET", "/api/v1/nope", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown api: code=%d", rec.Code)
	}
}

func TestWorkspaceSelectAndGuards(t *testing.T) {
	// Server starts on an uninitialized root.
	root := t.TempDir()
	SetGlobalConfigPathForTest(t, filepath.Join(t.TempDir(), "config.yaml"))
	srv, chat := newTestEnvAt(t, root)
	chat.responses = []llm.ChatResponse{{Content: "educated-general\n…"}}

	// Creating a book session is blocked until the workspace is configured+initialized.
	rec, resp := do(t, srv, "POST", "/api/v1/grill/sessions", map[string]any{"topic": "测试"})
	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("grill start without workspace: code=%d body=%v", rec.Code, resp)
	}
	// Chapter add is blocked too.
	rec, _ = do(t, srv, "POST", "/api/v1/books/x/chapters", map[string]any{"after": "01-01", "topic": "t"})
	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("chapter add without workspace: code=%d", rec.Code)
	}
	// Config view is blocked.
	rec, _ = do(t, srv, "GET", "/api/v1/config", nil)
	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("config without workspace: code=%d", rec.Code)
	}

	// Select a second, uninitialized directory → still blocked, root switched + persisted.
	other := t.TempDir()
	rec, sel := do(t, srv, "POST", "/api/v1/workspace/select", map[string]any{"path": other})
	wantCode(t, rec, http.StatusOK)
	if sel["initialized"] != false {
		t.Fatalf("selected dir should be uninitialized: %v", sel)
	}
	if got, _ := sel["root"].(string); got != other {
		t.Fatalf("root = %q, want %q", got, other)
	}
	if source, _ := sel["source"].(string); source != "web" {
		t.Fatalf("source = %v, want web", sel["source"])
	}
	// Persisted to the global config file for CLI reuse.
	if ws, err := config.GlobalWorkspace(); err != nil || ws != other {
		t.Fatalf("global config workspace = %q (err %v), want %q", ws, err, other)
	}

	// Initialize the selected directory → guards open.
	rec, _ = do(t, srv, "POST", "/api/v1/workspace/init", map[string]any{})
	wantCode(t, rec, http.StatusCreated)
	rec, _ = do(t, srv, "GET", "/api/v1/config", nil)
	wantCode(t, rec, http.StatusOK)
	rec, _ = do(t, srv, "POST", "/api/v1/grill/sessions", map[string]any{"topic": "测试"})
	wantCode(t, rec, http.StatusCreated)
}
