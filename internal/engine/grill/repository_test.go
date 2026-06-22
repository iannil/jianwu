package grill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepositorySaveLoadRoundTrip(t *testing.T) {
	ws := t.TempDir()
	repo := NewRepository(ws)
	s := NewSession()
	s.RecordAnswer("topic", "X")
	if err := repo.Save(s); err != nil {
		t.Fatal(err)
	}
	loaded, err := repo.Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Answers["topic"] != "X" {
		t.Errorf("answer: %q", loaded.Answers["topic"])
	}
}

func TestRepositoryListIncompleteOnlyReturnsInProgress(t *testing.T) {
	ws := t.TempDir()
	repo := NewRepository(ws)

	s1 := NewSession()
	s1.RecordAnswer("topic", "A")
	if err := repo.Save(s1); err != nil {
		t.Fatal(err)
	}

	s2 := NewSession()
	s2.Status = SessionCompleted
	if err := repo.Save(s2); err != nil {
		t.Fatal(err)
	}

	incomplete, err := repo.ListIncomplete()
	if err != nil {
		t.Fatal(err)
	}
	if len(incomplete) != 1 {
		t.Fatalf("got %d incomplete, want 1", len(incomplete))
	}
	if incomplete[0].ID != s1.ID {
		t.Errorf("wrong session")
	}
}

func TestRepositoryListIncompleteEmptyDir(t *testing.T) {
	ws := t.TempDir()
	repo := NewRepository(ws)
	incomplete, err := repo.ListIncomplete()
	if err != nil {
		t.Fatal(err)
	}
	if len(incomplete) != 0 {
		t.Errorf("expected 0, got %d", len(incomplete))
	}
}

func TestRepositoryArchive(t *testing.T) {
	ws := t.TempDir()
	repo := NewRepository(ws)
	s := NewSession()
	s.Status = SessionCompleted
	s.RecordAnswer("topic", "X")
	if err := repo.Save(s); err != nil {
		t.Fatal(err)
	}
	if err := repo.Archive(s, "my-book"); err != nil {
		t.Fatal(err)
	}
	// Active session file should be gone.
	if _, err := os.Stat(s.Path(repo.SessionsDir)); !os.IsNotExist(err) {
		t.Errorf("active session still exists: %v", err)
	}
	// Archived session should be in books/<slug>/.session.json.
	archived := filepath.Join(ws, "books", "my-book", ".session.json")
	if _, err := os.Stat(archived); err != nil {
		t.Errorf("archived file missing: %v", err)
	}
}
