package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iannil/jianwu/internal/book"
)

func writeMinimalBook(t *testing.T, slug string) string {
	t.Helper()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".jianwu"), 0o755); err != nil {
		t.Fatal(err)
	}
	bookDir := filepath.Join(tmp, "books", slug)
	if err := os.MkdirAll(filepath.Join(bookDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"), &book.Meta{
		Slug: slug, Title: "测试书", Archetype: "ontology-epistemology-practice",
		Language: "zh", Status: book.BookStatusDraft,
	}); err != nil {
		t.Fatal(err)
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), &book.Outline{
		Parts: []book.OutlinePart{{Index: 1, Title: "第一部分", Chapters: []book.OutlineChapter{
			{Index: 1, Title: "第一章", Status: book.StatusExpanded},
		}}},
	}); err != nil {
		t.Fatal(err)
	}
	return tmp
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
}

func TestLoadBook_OK(t *testing.T) {
	tmp := writeMinimalBook(t, "demo")
	chdir(t, tmp)
	bc, err := loadBook("demo")
	if err != nil {
		t.Fatalf("loadBook: %v", err)
	}
	if bc.Meta.Slug != "demo" || len(bc.Outline.Parts) != 1 {
		t.Errorf("unexpected bookCtx: %+v", bc)
	}
	if bc.BookDir == "" || bc.WSRoot == "" {
		t.Errorf("WSRoot/BookDir empty: %+v", bc)
	}
}

func TestLoadBook_MissingBook(t *testing.T) {
	tmp := writeMinimalBook(t, "demo")
	chdir(t, tmp)
	_, err := loadBook("nope")
	if err == nil {
		t.Fatal("expected error for missing book")
	}
}

func TestMirrorChapterStatus(t *testing.T) {
	tmp := writeMinimalBook(t, "demo")
	bookDir := filepath.Join(tmp, "books", "demo")
	if _, err := book.WriteChapter(bookDir, 1, 1, book.ChapterFrontmatter{
		Title: "第一章", PartIndex: 1, ChapterIndex: 1, Status: book.StatusExpanded,
	}, "正文内容"); err != nil {
		t.Fatal(err)
	}
	if err := mirrorChapterStatus(bookDir, 1, 1, book.StatusReviewed); err != nil {
		t.Fatalf("mirrorChapterStatus: %v", err)
	}
	fm, body, err := book.ReadChapter(book.ChapterPath(bookDir, 1, 1))
	if err != nil {
		t.Fatal(err)
	}
	if fm.Status != book.StatusReviewed {
		t.Errorf("frontmatter status = %q, want reviewed", fm.Status)
	}
	if body != "正文内容" {
		t.Errorf("body not preserved: %q", body)
	}
}
