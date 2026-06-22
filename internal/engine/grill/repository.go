package grill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	entries, err := os.ReadDir(r.SessionsDir)
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
	return out, nil
}

// Archive moves a completed session to books/<slug>/.session.json (audit log).
// Per Q11.A3.
func (r *Repository) Archive(s *Session, slug string) error {
	src := s.Path(r.SessionsDir)
	// Workspace root is two levels up from SessionsDir.
	wsRoot := filepath.Dir(filepath.Dir(r.SessionsDir))
	dst := filepath.Join(wsRoot, "books", slug, ".session.json")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir book dir: %w", err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read session for archive: %w", err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("write archived session: %w", err)
	}
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("remove active session after archive: %w", err)
	}
	return nil
}
