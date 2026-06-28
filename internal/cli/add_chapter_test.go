package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/storage"
	"github.com/spf13/cobra"
)

func TestAddChapterNeedsFlags(t *testing.T) {
	tests := []struct {
		name    string
		after   string
		topic   string
		as      string
		wantErr string
	}{
		{"missing after", "", "topic", "", "--after is required"},
		{"missing topic", "01-01", "", "", "--topic is required"},
		{"bad after", "bad", "topic", "", "invalid chapter address"},
		{"bad as", "01-01", "topic", "bad", "invalid chapter address"},
		{"as wrong part", "01-01", "topic", "02-02", "--as part 2 must match --after part 1"},
		{"as not after", "01-01", "topic", "01-01", "--as chapter 1 must be greater than --after chapter 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			err := runAddChapter(cmd, "test-book", tt.after, tt.topic, tt.as)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			ie, ok := err.(*InfoError)
			if !ok {
				t.Fatalf("expected *InfoError, got %T", err)
			}
			if ie.Code != ExitCodeUsage {
				t.Errorf("exit code: got %d want %d", ie.Code, ExitCodeUsage)
			}
		})
	}
}

func TestAddChapterInsertsAtCorrectPosition(t *testing.T) {
	// Set up a workspace with a book that has one part, two chapters.
	wsRoot := t.TempDir()
	initWorkspace(t, wsRoot)
	slug := "test-book"
	bookDir := filepath.Join(wsRoot, "books", slug)
	mkAll(t, bookDir)

	meta := &book.Meta{
		Slug:  slug,
		Title: "Test",
	}
	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"), meta); err != nil {
		t.Fatal(err)
	}

	outline := &book.Outline{
		Parts: []book.OutlinePart{
			{
				Index: 1, Title: "Part 1", Role: "ontology",
				Chapters: []book.OutlineChapter{
					{Index: 1, Title: "Ch 1", Status: book.StatusScaffolded},
					{Index: 2, Title: "Ch 2", Status: book.StatusScaffolded},
				},
			},
		},
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
		t.Fatal(err)
	}

	// Run add-chapter after 01-01.
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	// Mock workspace detection by setting -dir flag.
	oldDir := cliWorkspaceDir
	cliWorkspaceDir = wsRoot
	t.Cleanup(func() { cliWorkspaceDir = oldDir })

	if err := runAddChapter(cmd, slug, "01-01", "New Chapter", "01-03"); err != nil {
		t.Fatalf("runAddChapter: %v", err)
	}

	// Reload outline.
	loaded, err := book.LoadOutline(filepath.Join(bookDir, "outline.json"))
	if err != nil {
		t.Fatalf("LoadOutline: %v", err)
	}
	if len(loaded.Parts) != 1 {
		t.Fatalf("parts: got %d want 1", len(loaded.Parts))
	}
	chs := loaded.Parts[0].Chapters
	if len(chs) != 3 {
		t.Fatalf("chapters: got %d want 3", len(chs))
	}
	if chs[0].Index != 1 || chs[0].Title != "Ch 1" {
		t.Errorf("ch[0]: got (%d, %q) want (1, Ch 1)", chs[0].Index, chs[0].Title)
	}
	if chs[1].Index != 3 || chs[1].Title != "New Chapter" {
		t.Errorf("ch[1]: got (%d, %q) want (3, New Chapter)", chs[1].Index, chs[1].Title)
	}
	if chs[2].Index != 2 || chs[2].Title != "Ch 2" {
		t.Errorf("ch[2]: got (%d, %q) want (2, Ch 2)", chs[2].Index, chs[2].Title)
	}

	// Chapter file should exist.
	chapPath := book.ChapterPath(bookDir, 1, 3)
	if _, err := storage.OS.Stat(chapPath); err != nil {
		t.Errorf("chapter file should exist: %v", err)
	}

	// Verify frontmatter.
	fm, body, err := book.ReadChapter(chapPath)
	if err != nil {
		t.Fatalf("ReadChapter: %v", err)
	}
	if fm.Title != "New Chapter" {
		t.Errorf("frontmatter title: got %q want %q", fm.Title, "New Chapter")
	}
	if fm.Status != book.StatusScaffolded {
		t.Errorf("frontmatter status: got %q want %q", fm.Status, book.StatusScaffolded)
	}
	if body != "" {
		t.Errorf("body should be empty for stub, got %q", body)
	}
}

func TestAddChapterConflict(t *testing.T) {
	wsRoot := t.TempDir()
	initWorkspace(t, wsRoot)
	slug := "test-book"
	bookDir := filepath.Join(wsRoot, "books", slug)
	mkAll(t, bookDir)

	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"), &book.Meta{Slug: slug}); err != nil {
		t.Fatal(err)
	}
	outline := &book.Outline{
		Parts: []book.OutlinePart{
			{
				Index: 1, Title: "P1", Role: "x",
				Chapters: []book.OutlineChapter{
					{Index: 1, Title: "Ch 1"},
					{Index: 2, Title: "Ch 2"},
				},
			},
		},
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
		t.Fatal(err)
	}

	oldDir := cliWorkspaceDir
	cliWorkspaceDir = wsRoot
	t.Cleanup(func() { cliWorkspaceDir = oldDir })

	// Try to add after 01-01 which would default to 01-02 (already exists).
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := runAddChapter(cmd, slug, "01-01", "Conflict", "")
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
}

// initWorkspace creates .jianwu/ marker + schema_version in root.
func initWorkspace(t *testing.T, root string) {
	t.Helper()
	mkAll(t, filepath.Join(root, ".jianwu"))
	if err := os.WriteFile(filepath.Join(root, ".jianwu", "schema_version"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write a minimal config so workspace.Load doesn't fail.
	cfgPath := filepath.Join(root, ".jianwu", "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("models:\n  intake:\n    provider: mock\n    model: mock\n  outline:\n    provider: mock\n    model: mock\n  scaffolding:\n    provider: mock\n    model: mock\n  expand:\n    provider: mock\n    model: mock\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mkAll creates a directory and all parents.
func mkAll(t *testing.T, dir string) {
	t.Helper()
	// initialize workspace.FindWorkspace for tests: clear caches if any.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestFindChapterByExact(t *testing.T) {
	o := &book.Outline{
		Parts: []book.OutlinePart{
			{Index: 1, Chapters: []book.OutlineChapter{{Index: 1}, {Index: 3}}},
			{Index: 2, Chapters: []book.OutlineChapter{{Index: 1}}},
		},
	}
	if c := findChapterByExact(o, 1, 1); c == nil {
		t.Error("expected to find 01-01")
	}
	if c := findChapterByExact(o, 1, 3); c == nil {
		t.Error("expected to find 01-03")
	}
	if c := findChapterByExact(o, 2, 1); c == nil {
		t.Error("expected to find 02-01")
	}
	if c := findChapterByExact(o, 1, 2); c != nil {
		t.Error("expected nil for nonexistent chapter")
	}
	if c := findChapterByExact(o, 3, 1); c != nil {
		t.Error("expected nil for nonexistent part")
	}
}

func TestFindPartByIndex(t *testing.T) {
	o := &book.Outline{
		Parts: []book.OutlinePart{
			{Index: 1, Title: "A"},
			{Index: 3, Title: "C"},
		},
	}
	if p := findPartByIndex(o, 1); p == nil || p.Title != "A" {
		t.Error("expected to find part 1")
	}
	if p := findPartByIndex(o, 3); p == nil || p.Title != "C" {
		t.Error("expected to find part 3")
	}
	if p := findPartByIndex(o, 2); p != nil {
		t.Error("expected nil for missing part")
	}
}
