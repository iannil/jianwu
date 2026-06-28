package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/iannil/jianwu/internal/book"
	"github.com/spf13/cobra"
)

func TestDeleteChapterNeedsArgs(t *testing.T) {
	tests := []struct {
		name    string
		slug    string
		addr    string
		wantErr string
	}{
		{"bad address", "test", "bad", "invalid chapter address"},
		{"missing part", "test", "99-01", "part 99 not found in outline"},
		{"missing chapter", "test", "01-99", "chapter 01-99 not found in outline"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wsRoot := t.TempDir()
			initWorkspace(t, wsRoot)
			bookDir := filepath.Join(wsRoot, "books", tt.slug)
			mkAll(t, bookDir)

			if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"),
				&book.Meta{Slug: tt.slug}); err != nil {
				t.Fatal(err)
			}
			if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"),
				&book.Outline{Parts: []book.OutlinePart{
					{Index: 1, Chapters: []book.OutlineChapter{{Index: 1}}},
				}}); err != nil {
				t.Fatal(err)
			}

			oldDir := cliWorkspaceDir
			cliWorkspaceDir = wsRoot
			t.Cleanup(func() { cliWorkspaceDir = oldDir })

			cmd := &cobra.Command{}
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			err := runDeleteChapter(cmd, []string{tt.slug, tt.addr})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			ie, ok := err.(*InfoError)
			if !ok {
				t.Fatalf("expected *InfoError, got %T", err)
			}
			if ie.Code != ExitCodeUsage {
				t.Errorf("code: got %d want %d", ie.Code, ExitCodeUsage)
			}
		})
	}
}

func TestDeleteChapterRemovesFromOutlineAndFile(t *testing.T) {
	wsRoot := t.TempDir()
	initWorkspace(t, wsRoot)
	slug := "test-book"
	bookDir := filepath.Join(wsRoot, "books", slug)
	mkAll(t, bookDir)

	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"),
		&book.Meta{Slug: slug}); err != nil {
		t.Fatal(err)
	}
	outline := &book.Outline{
		Parts: []book.OutlinePart{
			{
				Index: 1, Title: "P1", Role: "x",
				Chapters: []book.OutlineChapter{
					{Index: 1, Title: "Ch 1"},
					{Index: 2, Title: "Ch 2"},
					{Index: 3, Title: "Ch 3"},
				},
			},
		},
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
		t.Fatal(err)
	}

	// Create chapter files.
	for _, c := range outline.Parts[0].Chapters {
		book.WriteChapter(bookDir, 1, c.Index, book.ChapterFrontmatter{
			Title:  c.Title,
			Status: book.StatusScaffolded,
		}, "# "+c.Title)
	}

	oldDir := cliWorkspaceDir
	cliWorkspaceDir = wsRoot
	t.Cleanup(func() { cliWorkspaceDir = oldDir })

	// Delete chapter 01-02.
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := runDeleteChapter(cmd, []string{slug, "01-02"}); err != nil {
		t.Fatalf("runDeleteChapter: %v", err)
	}

	// Reload outline — should have 2 chapters.
	loaded, err := book.LoadOutline(filepath.Join(bookDir, "outline.json"))
	if err != nil {
		t.Fatalf("LoadOutline: %v", err)
	}
	chs := loaded.Parts[0].Chapters
	if len(chs) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(chs))
	}
	if chs[0].Index != 1 || chs[1].Index != 3 {
		t.Errorf("indexes: got %d and %d, want 1 and 3", chs[0].Index, chs[1].Index)
	}

	// Chapter file should be deleted.
	chapPath := book.ChapterPath(bookDir, 1, 2)
	if _, err := os.Stat(chapPath); !os.IsNotExist(err) {
		t.Errorf("chapter file should be deleted: %v", err)
	}

	// Chap 1 and Chap 3 files should still exist.
	for _, idx := range []int{1, 3} {
		p := book.ChapterPath(bookDir, 1, idx)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("chapter %02d file should exist: %v", idx, err)
		}
	}
}
