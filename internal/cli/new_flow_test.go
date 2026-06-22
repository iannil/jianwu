package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhurong/jianwu/internal/engine/grill"
)

func TestCheckSlugConflictEmpty(t *testing.T) {
	ws := t.TempDir()
	if err := checkSlugConflict(ws, "my-book", false); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestCheckSlugConflictExistingNoForce(t *testing.T) {
	ws := t.TempDir()
	bookDir := filepath.Join(ws, "books", "my-book")
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	err := checkSlugConflict(ws, "my-book", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error: %v", err)
	}
}

func TestCheckSlugConflictExistingForceRemoves(t *testing.T) {
	ws := t.TempDir()
	bookDir := filepath.Join(ws, "books", "my-book")
	if err := os.MkdirAll(filepath.Join(bookDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bookDir, "meta.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkSlugConflict(ws, "my-book", true); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if _, err := os.Stat(bookDir); !os.IsNotExist(err) {
		t.Errorf("book dir should be removed")
	}
}

func TestOfferResumeNoSessions(t *testing.T) {
	ws := t.TempDir()
	repo := grill.NewRepository(ws)
	var out bytes.Buffer
	p := &TerminalPrompt{In: strings.NewReader(""), Out: &out}
	s, err := offerResume(repo, p)
	if err != nil {
		t.Fatal(err)
	}
	if s != nil {
		t.Errorf("expected nil, got %v", s)
	}
}

func TestOfferResumeWithChoice(t *testing.T) {
	ws := t.TempDir()
	repo := grill.NewRepository(ws)
	s := grill.NewSession()
	s.RecordAnswer("topic", "时间的实在")
	if err := repo.Save(s); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	p := &TerminalPrompt{In: strings.NewReader("1\n"), Out: &out}
	loaded, err := offerResume(repo, p)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.ID != s.ID {
		t.Errorf("expected resumed session %s, got %v", s.ID, loaded)
	}
}

func TestOfferResumeEmptyInputStartsFresh(t *testing.T) {
	ws := t.TempDir()
	repo := grill.NewRepository(ws)
	s := grill.NewSession()
	s.RecordAnswer("topic", "X")
	if err := repo.Save(s); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	p := &TerminalPrompt{In: strings.NewReader("\n"), Out: &out}
	loaded, err := offerResume(repo, p)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != nil {
		t.Errorf("expected nil (fresh start), got %v", loaded)
	}
}

func TestDeriveSlugFromTopic(t *testing.T) {
	s := deriveSlugFromTopic("Reality of Time")
	if s != "reality-of-time" {
		t.Errorf("got %q", s)
	}
}
