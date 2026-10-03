package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/engine/grill"
)

// --- small shared helpers for the MCP toolset (ADR 30) ---

// isWorkspaceInit reports whether root looks like an initialized workspace.
func isWorkspaceInit(root string) bool {
	_, err := os.Stat(filepath.Join(root, ".jianwu"))
	return err == nil
}

// mcpChapterCount counts chapters across all parts.
func mcpChapterCount(bc *bookCtx) int {
	n := 0
	for _, p := range bc.Outline.Parts {
		n += len(p.Chapters)
	}
	return n
}

// totalWords sums outline word counts.
func totalWords(bc *bookCtx) int {
	n := 0
	for _, p := range bc.Outline.Parts {
		for _, c := range p.Chapters {
			n += c.WordCount
		}
	}
	return n
}

// failedVerdicts counts verdicts whose source did not support the claim.
func failedVerdicts(vs []book.ClaimVerdict) int {
	n := 0
	for _, v := range vs {
		if !v.Verified {
			n++
		}
	}
	return n
}

// saveBookState persists outline.json + meta.json for one book.
func saveBookState(bc *bookCtx) error {
	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		return err
	}
	return book.SaveMeta(filepath.Join(bc.BookDir, "meta.json"), bc.Meta)
}

// mcpCreateBook builds a completed grill session from explicit dimensions
// (defaults for the rest), then runs the same new-book job the web UI uses
// and waits for it. Returns the slug and chapter count.
func (s *Server) mcpCreateBook(ctx context.Context, topic string, dims map[string]string) (*callToolShim, error) {
	if topic == "" {
		return nil, fmt.Errorf("topic is required")
	}
	tree := grill.DefaultTree()
	session := grill.NewSession()
	session.RecordAnswer("topic", topic)
	defaults := map[string]string{
		"audience":  "beginner",
		"goal":      "understanding",
		"depth":     "intro",
		"length":    "short",
		"language":  "zh",
		"archetype": "foundations-application-practice",
	}
	for dim, val := range defaults {
		if v := dims[dim]; v != "" {
			val = v
		}
		session.RecordAnswer(dim, val)
	}
	// Remaining dimensions (scope/example_type/visualization/timeliness/…)
	// take the design tree's defaults — no interview round-trips.
	for {
		pending := tree.NextPending(session.Answers)
		if pending == nil {
			break
		}
		if pending.DefaultValue == "" {
			return nil, fmt.Errorf("dimension %q has no default; unsupported for agent creation", pending.ID)
		}
		session.RecordAnswer(pending.ID, pending.DefaultValue)
	}

	deps, err := s.resolveDeps("outline", needChatterOnly)
	if err != nil {
		return nil, err
	}
	var jobErr error
	done := make(chan struct{})
	jobID := s.jobs.Start("new-book", book.Slugify(topic), func(jctx context.Context, j *Job) error {
		defer close(done)
		err := s.runNewBookJob(jctx, j, session, deps, false)
		jobErr = err
		return err
	})
	select {
	case <-done:
	case <-ctx.Done():
		return nil, fmt.Errorf("create_book cancelled (job %s continues server-side)", jobID)
	case <-time.After(8 * time.Minute):
		return nil, fmt.Errorf("create_book timed out after 8m (job %s continues server-side); poll job_status", jobID)
	}
	if jobErr != nil {
		return nil, jobErr
	}
	bc, err := s.loadBook(book.Slugify(topic))
	if err != nil {
		return nil, err
	}
	return &callToolShim{
		slug:     filepath.Base(bc.BookDir),
		chapters: mcpChapterCount(bc),
		parts:    len(bc.Outline.Parts),
	}, nil
}

// callToolShim carries create_book results (kept as a struct so the MCP
// layer can render it as JSON without importing server internals).
type callToolShim struct {
	slug     string
	chapters int
	parts    int
}
