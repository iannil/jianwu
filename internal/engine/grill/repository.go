package grill

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iannil/jianwu/internal/storage"
)

// Repository manages session files in a workspace.
// Per Q11.A1: active sessions live in .jianwu/sessions/.
// Per Q11.A3: completed sessions move to books/<slug>/.session.json.
type Repository struct {
	SessionsDir string // <workspace>/.jianwu/sessions/
}

// NewRepository constructs a Repository rooted at the workspace root.
func NewRepository(workspaceRoot string) *Repository {
	return &Repository{
		SessionsDir: filepath.Join(workspaceRoot, ".jianwu", "sessions"),
	}
}

// Save persists a session to the active sessions directory.
func (r *Repository) Save(s *Session) error {
	return s.Save(r.SessionsDir)
}

// Load reads a session by ID from the active sessions directory.
func (r *Repository) Load(id string) (*Session, error) {
	return LoadSession(r.SessionsDir, id)
}

// ListIncomplete returns sessions in progress, sorted by StartedAt ascending.
// Used for resume prompts on startup (per Q11.A2).
func (r *Repository) ListIncomplete() ([]*Session, error) {
	entries, err := storage.OS.ReadDir(r.SessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	var out []*Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		s, err := r.Load(id)
		if err != nil {
			continue // skip corrupt files
		}
		if s.Status == SessionInProgress {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.Before(out[j].StartedAt)
	})
	return out, nil
}

// Archive moves a completed session to books/<slug>/.session.json (audit log).
// Per Q11.A3. Uses os.Rename for an atomic move within the same filesystem.
func (r *Repository) Archive(s *Session, slug string) error {
	src := s.Path(r.SessionsDir)
	// Workspace root is two levels up from SessionsDir.
	wsRoot := filepath.Dir(filepath.Dir(r.SessionsDir))
	dst := filepath.Join(wsRoot, "books", slug, ".session.json")
	if err := storage.OS.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir book dir: %w", err)
	}
	// Atomic rename — both paths are under the same workspace root.
	if err := storage.OS.Rename(src, dst); err != nil {
		return fmt.Errorf("archive session %s: %w", s.ID, err)
	}
	return nil
}
