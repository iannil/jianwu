package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/workspace"
)

// workspaceConfig loads the workspace and returns its resolved config.
func (s *Server) workspaceConfig() (*config.Config, error) {
	ws, err := workspace.Load(s.root())
	if err != nil {
		return nil, err
	}
	return ws.Config, nil
}

// handleWorkspace returns the workspace root, resolution source,
// initialization state and the book list with per-book progress. Always 200:
// an unconfigured workspace is a valid state the UI offers to fix.
func (s *Server) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"root":        s.root(),
		"source":      s.wsSourceOf(),
		"version":     s.version,
		"initialized": false,
		"books":       []any{},
	}
	if _, err := workspace.Load(s.root()); err != nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp["initialized"] = true

	cfg, err := s.workspaceConfig()
	if err == nil {
		resp["config"] = configView(cfg)
	}
	books, err := s.listBooks()
	if err == nil {
		resp["books"] = books
	}
	writeJSON(w, http.StatusOK, resp)
}

// SOURCE_ZH labels workspace resolution sources for the UI.
var SOURCE_ZH = map[string]string{
	"flag":    "命令行参数",
	"env":     "环境变量 JIANWU_WORKSPACE",
	"config":  "全局配置文件",
	"cwd":     "启动目录",
	"web":     "网页设置",
	"startup": "启动参数",
}

// expandHomePath resolves a leading ~/ to the user's home directory and makes
// the path absolute (against the server process CWD).
func expandHomePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("路径为空")
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve HOME: %w", err)
		}
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return filepath.Abs(p)
}

// handleWorkspaceSelect switches the active workspace root at runtime and
// persists the choice to the global config file so the CLI and future server
// starts pick it up. The target directory may be empty (not yet initialized);
// the UI then offers one-click initialization.
func (s *Server) handleWorkspaceSelect(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := decodeBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	abs, err := expandHomePath(body.Path)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if info, statErr := os.Stat(abs); statErr == nil && !info.IsDir() {
		failf(w, http.StatusBadRequest, "%s 不是目录", abs)
		return
	}
	if err := config.SetGlobalWorkspace(abs); err != nil {
		failErr(w, err)
		return
	}
	s.setRoot(abs, "web")

	resp := map[string]any{
		"root":        abs,
		"source":      s.wsSourceOf(),
		"source_zh":   SOURCE_ZH[s.wsSourceOf()],
		"initialized": false,
	}
	if _, err := workspace.Load(s.root()); err == nil {
		resp["initialized"] = true
		if cfg, err := s.workspaceConfig(); err == nil {
			resp["config"] = configView(cfg)
		}
		if books, err := s.listBooks(); err == nil {
			resp["books"] = books
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleWorkspaceInit initializes the workspace at the server root.
func (s *Server) handleWorkspaceInit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Bare bool `json:"bare"`
	}
	if err := decodeBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := workspace.Load(s.root()); err == nil {
		fail(w, http.StatusConflict, "workspace 已存在")
		return
	}
	if err := workspace.Init(s.root(), workspace.InitOpts{Bare: body.Bare}); err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "root": s.root()})
}

// modelRefView is the JSON projection of a stage model reference.
type modelRefView struct {
	Provider string        `json:"provider"`
	Model    string        `json:"model"`
	Fallback *modelRefView `json:"fallback,omitempty"`
	Timeout  int           `json:"timeout,omitempty"`
}

func modelRefViewOf(m config.ModelRef) *modelRefView {
	v := &modelRefView{Provider: m.Provider, Model: m.Model, Timeout: m.TimeoutSeconds}
	if m.Fallback != nil {
		v.Fallback = modelRefViewOf(*m.Fallback)
	}
	return v
}

func configView(cfg *config.Config) map[string]any {
	return map[string]any{
		"llm": map[string]any{"timeout": cfg.LLM.TimeoutSeconds},
		"models": map[string]any{
			"intake":      modelRefViewOf(cfg.Models.Intake),
			"outline":     modelRefViewOf(cfg.Models.Outline),
			"scaffolding": modelRefViewOf(cfg.Models.Scaffolding),
			"expand":      modelRefViewOf(cfg.Models.Expand),
		},
		"search":      cfg.Search,
		"archetypes":  map[string]any{"library": cfg.Archetypes.Library},
		"style":       map[string]any{"guide": cfg.Style.Guide, "samples": cfg.Style.Samples},
		"scaffolding": map[string]any{"concurrency": cfg.Scaffolding.Concurrency},
		"logging":     map[string]any{"level": cfg.Logging.Level},
	}
}

// bookSummary is the list-view projection of a book.
type bookSummary struct {
	Slug       string          `json:"slug"`
	Title      string          `json:"title"`
	Status     string          `json:"status"`
	Archetype  string          `json:"archetype,omitempty"`
	Language   string          `json:"language,omitempty"`
	UpdatedAt  string          `json:"updated_at,omitempty"`
	Usage      book.TokenUsage `json:"token_usage"`
	Stats      outlineStats    `json:"stats"`
	Parameters book.Parameters `json:"parameters"`
}

// listBooks loads every book's meta + outline for summary counts.
func (s *Server) listBooks() ([]bookSummary, error) {
	booksDir := filepath.Join(s.root(), "books")
	entries, err := os.ReadDir(booksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []bookSummary{}, nil
		}
		return nil, err
	}
	out := []bookSummary{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		bc, err := s.loadBook(e.Name())
		if err != nil {
			continue // skip corrupt/incomplete book dirs
		}
		out = append(out, bookSummary{
			// The directory name is the resolution key (loadBook, site.Scan);
			// reporting meta.slug here made the entry 404 whenever the two
			// diverged (hand-renamed dir, edited meta).
			Slug:       e.Name(),
			Title:      bc.Meta.Title,
			Status:     bc.Meta.Status,
			Archetype:  bc.Meta.Archetype,
			Language:   bc.Meta.Language,
			UpdatedAt:  bc.Meta.UpdatedAt.Format("2006-01-02 15:04"),
			Usage:      bc.Meta.TokenUsage,
			Stats:      computeStats(bc.Outline),
			Parameters: bc.Meta.Parameters,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out, nil
}

// handleBooksList returns the book summaries.
func (s *Server) handleBooksList(w http.ResponseWriter, r *http.Request) {
	books, err := s.listBooks()
	if err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"books": books})
}

// handleBookDetail returns meta + the full outline (chapters, claims,
// citations, verdicts) for one book.
func (s *Server) handleBookDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	bc, err := s.loadBook(slug)
	if err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"meta":    bc.Meta,
		"outline": bc.Outline,
		"stats":   computeStats(bc.Outline),
	})
}
