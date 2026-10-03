// Package server exposes jianwu's creation pipeline over HTTP with an
// embedded web UI. It mirrors the CLI's orchestration glue (engine + book +
// workspace calls) so the CLI stays a thin wrapper and the web layer reuses
// the same engines directly.
//
// The server is single-workspace, single-process: it binds localhost by
// default and serializes all mutating jobs through one worker, matching the
// "no concurrent writers for one book" delivery constraint.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/reader"
	"github.com/iannil/jianwu/internal/provider/search"
	"github.com/iannil/jianwu/internal/workspace"
)

// Deps bundles providers used by server handlers. Tests inject mocks here;
// in production deps are built lazily from workspace config + secrets.
// A nil Deps (or nil field) means "build from config".
type Deps struct {
	Chatter  llm.Chatter
	Searcher search.Searcher
	Reader   reader.Reader
	Embedder llm.Embedder
}

// Server is one HTTP server operating on a configurable workspace root.
// The root can be switched at runtime (POST /api/v1/workspace/select), which
// persists the choice to the global config file; resolution precedence at
// startup is --dir flag > JIANWU_WORKSPACE > global config > CWD.
type Server struct {
	mu       sync.Mutex
	wsRootV  string
	wsSource string // flag | env | config | cwd | web
	version  string
	apiToken string // optional Bearer token for /api/v1 (ADR 30)
	inject   *Deps
	jobs     *JobManager
}

// New constructs a Server rooted at wsRoot. deps may be nil (build from
// config + secrets) or partially filled (nil fields fall back to config).
func New(wsRoot, version string, deps *Deps) *Server {
	return &Server{
		wsRootV:  wsRoot,
		wsSource: "startup",
		version:  version,
		inject:   deps,
		jobs:     NewJobManager(),
	}
}

// NewWithSource constructs a Server and records where the initial workspace
// root came from (workspace.SourceFlag / SourceEnv / SourceConfig / SourceCWD).
func NewWithSource(wsRoot, source, version string, deps *Deps) *Server {
	s := New(wsRoot, version, deps)
	s.wsSource = source
	return s
}

// SetToken enables Bearer-token auth for all /api/v1 requests (ADR 30).
// An empty token keeps the default localhost trust model.
func (s *Server) SetToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apiToken = token
}

// root returns the current workspace root (thread-safe; the root can be
// switched at runtime).
func (s *Server) root() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wsRootV
}

// setRoot switches the workspace root and records the source.
func (s *Server) setRoot(root, source string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wsRootV = root
	s.wsSource = source
}

// wsSourceOf returns the current workspace source label.
func (s *Server) wsSourceOf() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wsSource
}

// WSRoot returns the workspace root the server operates on.
func (s *Server) WSRoot() string { return s.root() }

// requireWorkspace fails the request with a clear setup message unless the
// current root is an initialized workspace. All book-creating / generating /
// mutating endpoints call this, so an unconfigured workspace blocks them
// until the user configures one (web, env var, or global config).
func (s *Server) requireWorkspace(w http.ResponseWriter) bool {
	if _, err := workspace.Load(s.root()); err == nil {
		return true
	}
	fail(w, http.StatusPreconditionRequired,
		"工作区未配置或未初始化：请先在工作区页面选择目录并初始化，或设置 JIANWU_WORKSPACE / 全局配置后重启")
	return false
}

// Handler returns the HTTP handler: JSON API under /api/v1 plus the
// embedded single-page UI for everything else.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Workspace + config.
	mux.HandleFunc("GET /api/v1/workspace", s.handleWorkspace)
	mux.HandleFunc("POST /api/v1/workspace/init", s.handleWorkspaceInit)
	mux.HandleFunc("POST /api/v1/workspace/select", s.handleWorkspaceSelect)
	mux.HandleFunc("GET /api/v1/config", s.handleConfig)
	mux.HandleFunc("POST /api/v1/config", s.handleConfigSave)
	mux.HandleFunc("GET /api/v1/secrets", s.handleSecretsGet)
	mux.HandleFunc("POST /api/v1/secrets", s.handleSecretsSave)

	// Books.
	mux.HandleFunc("GET /api/v1/books", s.handleBooksList)
	mux.HandleFunc("GET /api/v1/books/{slug}", s.handleBookDetail)
	mux.HandleFunc("POST /api/v1/books/{slug}/expand", s.handleExpandBook)    // {part, chapter, force} or {all, force}
	mux.HandleFunc("POST /api/v1/books/{slug}/expand-all", s.handleExpandAll) // {force}
	mux.HandleFunc("POST /api/v1/books/{slug}/scaffold-retry", s.handleScaffoldRetry)
	mux.HandleFunc("POST /api/v1/books/{slug}/finalize", s.handleFinalize)     // {dry_run}
	mux.HandleFunc("POST /api/v1/books/{slug}/export", s.handleExport)         // {target, dry_run}
	mux.HandleFunc("GET /api/v1/books/{slug}/export/file", s.handleExportFile) // ?target=md|hugo|pdf
	mux.HandleFunc("POST /api/v1/books/{slug}/publish", s.handlePublish)       // {version, major, dry_run}
	mux.HandleFunc("GET /api/v1/books/{slug}/releases", s.handleReleases)      // published versions
	mux.HandleFunc("POST /api/v1/books/{slug}/chapters", s.handleChapterAdd)   // {after, topic, as}

	// Chapters.
	mux.HandleFunc("GET /api/v1/books/{slug}/chapters/{part}/{ch}", s.handleChapterGet)
	mux.HandleFunc("DELETE /api/v1/books/{slug}/chapters/{part}/{ch}", s.handleChapterDelete)
	mux.HandleFunc("POST /api/v1/books/{slug}/chapters/{part}/{ch}/move", s.handleChapterMove) // {target_part, after}
	mux.HandleFunc("POST /api/v1/books/{slug}/chapters/{part}/{ch}/review", s.handleChapterReview)
	mux.HandleFunc("POST /api/v1/books/{slug}/chapters/{part}/{ch}/expand", s.handleChapterExpand) // {force}
	mux.HandleFunc("POST /api/v1/books/{slug}/chapters/{part}/{ch}/factcheck", s.handleChapterFactcheck)
	mux.HandleFunc("POST /api/v1/books/{slug}/chapters/{part}/{ch}/revise", s.handleChapterRevise)

	// Grill (design interview).
	mux.HandleFunc("GET /api/v1/grill/tree", s.handleGrillTree)
	mux.HandleFunc("GET /api/v1/grill/sessions", s.handleGrillSessions)
	mux.HandleFunc("POST /api/v1/grill/sessions", s.handleGrillStart)
	mux.HandleFunc("GET /api/v1/grill/sessions/{id}", s.handleGrillSessionGet)
	mux.HandleFunc("DELETE /api/v1/grill/sessions/{id}", s.handleGrillAbandon)
	mux.HandleFunc("POST /api/v1/grill/sessions/{id}/answer", s.handleGrillAnswer)
	mux.HandleFunc("DELETE /api/v1/grill/sessions/{id}/answers/{dim}", s.handleGrillUnanswer)
	mux.HandleFunc("POST /api/v1/grill/sessions/{id}/generate", s.handleGrillGenerate)

	// Corpus.
	mux.HandleFunc("POST /api/v1/site/generate", s.handleSiteGenerate) // {dry_run}

	mux.HandleFunc("GET /api/v1/corpus", s.handleCorpusList)
	mux.HandleFunc("GET /api/v1/corpus/stats", s.handleCorpusStats)
	mux.HandleFunc("GET /api/v1/corpus/{slug}", s.handleCorpusShow)
	mux.HandleFunc("POST /api/v1/corpus/sync", s.handleCorpusSync)
	mux.HandleFunc("POST /api/v1/corpus/collect", s.handleCorpusCollect)
	mux.HandleFunc("POST /api/v1/corpus/reindex", s.handleCorpusReindex)

	// Jobs.
	mux.HandleFunc("GET /api/v1/jobs", s.handleJobsList)
	mux.HandleFunc("GET /api/v1/jobs/{id}", s.handleJobGet)
	mux.HandleFunc("DELETE /api/v1/jobs/{id}", s.handleJobCancel)

	// Unmatched /api paths are 404, not the SPA fallback.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		fail(w, http.StatusNotFound, fmt.Sprintf("unknown API route: %s %s", r.Method, r.URL.Path))
	})

	mux.Handle("/", spaHandler())

	return s.authAPI(logRequests(mux))
}

// authAPI enforces the optional Bearer token on /api/v1/* when one is set.
// SPA assets and non-API paths are not gated (ADR 30: token mode is for
// agent API access).
func (s *Server) authAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		token := s.apiToken
		s.mu.Unlock()
		if token != "" && strings.HasPrefix(r.URL.Path, "/api/") {
			want := "Bearer " + token
			if !constantTimeEqual(r.Header.Get("Authorization"), want) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="jianwu"`)
				fail(w, http.StatusUnauthorized, "missing or invalid bearer token")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// constantTimeEqual compares two strings without early exit on mismatch.
func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// logRequests emits one INFO line per request; failures surface in responses.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// decodeBody parses the JSON request body; an empty body is allowed (dst untouched).
func decodeBody(r *http.Request, dst any) error {
	if r.Body == nil {
		return nil
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		if err.Error() == "EOF" {
			return nil
		}
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}

// apiError is the JSON error envelope: {"error": "..."}.
type apiError struct {
	Error string `json:"error"`
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, apiError{Error: msg})
}

func failf(w http.ResponseWriter, code int, format string, args ...any) {
	fail(w, code, fmt.Sprintf(format, args...))
}

// failErr maps an internal error to an HTTP response. Status selection:
// not-exist errors → 404, everything else → 400 with the unwrapped message.
func failErr(w http.ResponseWriter, err error) {
	msg := err.Error()
	if strings.Contains(msg, "not found") {
		fail(w, http.StatusNotFound, msg)
		return
	}
	fail(w, http.StatusBadRequest, msg)
}

// pathInt parses an integer path segment.
func pathInt(r *http.Request, name string) (int, error) {
	v, err := strconv.Atoi(r.PathValue(name))
	if err != nil || v < 1 {
		return 0, fmt.Errorf("invalid %s %q", name, r.PathValue(name))
	}
	return v, nil
}

// --- workspace / book loading ---

func (s *Server) loadWorkspace() (*workspace.Workspace, error) {
	return workspace.Load(s.root())
}

// bookCtx is the resolved on-disk context for one book (mirrors cli.bookCtx).
type bookCtx struct {
	WSRoot  string
	BookDir string
	Meta    *book.Meta
	Outline *book.Outline
}

func (s *Server) loadBook(slug string) (*bookCtx, error) {
	if err := validSlug(slug); err != nil {
		return nil, err
	}
	bookDir := filepath.Join(s.root(), "books", slug)
	meta, err := book.LoadMeta(filepath.Join(bookDir, "meta.json"))
	if err != nil {
		return nil, fmt.Errorf("book %q not found: %w", slug, err)
	}
	outline, err := book.LoadOutline(filepath.Join(bookDir, "outline.json"))
	if err != nil {
		return nil, fmt.Errorf("load outline for %q: %w", slug, err)
	}
	return &bookCtx{WSRoot: s.root(), BookDir: bookDir, Meta: meta, Outline: outline}, nil
}

// validSlug rejects path traversal in slug path segments.
func validSlug(slug string) error {
	if slug == "" || strings.ContainsAny(slug, "/\\") || slug == "." || slug == ".." {
		return fmt.Errorf("invalid slug %q", slug)
	}
	return nil
}

// findChapter returns a pointer to the chapter at (partIdx, chIdx).
func findChapter(outline *book.Outline, partIdx, chIdx int) (*book.OutlineChapter, error) {
	for i := range outline.Parts {
		if outline.Parts[i].Index == partIdx {
			for j := range outline.Parts[i].Chapters {
				if outline.Parts[i].Chapters[j].Index == chIdx {
					return &outline.Parts[i].Chapters[j], nil
				}
			}
			return nil, fmt.Errorf("chapter %02d-%02d not found", partIdx, chIdx)
		}
	}
	return nil, fmt.Errorf("part %d not found", partIdx)
}

func findPartByIndex(outline *book.Outline, partIdx int) *book.OutlinePart {
	for i := range outline.Parts {
		if outline.Parts[i].Index == partIdx {
			return &outline.Parts[i]
		}
	}
	return nil
}

// outlineStats summarizes chapter status counts for list views.
type outlineStats struct {
	Total      int `json:"total"`
	Scaffolded int `json:"scaffolded"`
	Expanded   int `json:"expanded"`
	Reviewed   int `json:"reviewed"`
	Final      int `json:"final"`
	Failed     int `json:"failed"`
}

func computeStats(outline *book.Outline) outlineStats {
	var st outlineStats
	for _, p := range outline.Parts {
		for _, c := range p.Chapters {
			st.Total++
			switch c.Status {
			case book.StatusScaffolded:
				st.Scaffolded++
			case book.StatusExpanded:
				st.Expanded++
			case book.StatusReviewed:
				st.Reviewed++
			case book.StatusFinal:
				st.Final++
			case book.StatusFailed:
				st.Failed++
			}
		}
	}
	return st
}

// bookDirExists reports whether books/<slug>/ exists.
func (s *Server) bookDirExists(slug string) bool {
	info, err := os.Stat(filepath.Join(s.root(), "books", slug))
	return err == nil && info.IsDir()
}
