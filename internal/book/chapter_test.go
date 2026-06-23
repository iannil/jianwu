package book

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteAndReadChapterRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	bookDir := filepath.Join(tmp, "books", "my-book")

	inFM := ChapterFrontmatter{
		Title:                 "现实作为边界",
		PartIndex:             1,
		ChapterIndex:          1,
		Status:                StatusExpanded,
		WordCount:             3421,
		GeneratedAt:           time.Date(2026, 6, 22, 19, 30, 0, 0, time.UTC),
		Model:                 "glm-4.6",
		EngineVersion:         "v0.1.1",
		UnverifiedClaimsCount: 3,
		Citations: []ChapterCitation{
			{ID: "1", URL: "https://example.com/a", Title: "Example A", Site: "example.com"},
			{ID: "2", URL: "https://example.com/b", Title: "Example B", Site: "example.com"},
		},
	}
	inBody := "## 现实作为边界\n\n正文段落...[^1]\n\n[^1]: [Example A](https://example.com/a) accessed 2026-06-22"

	path, err := WriteChapter(bookDir, 1, 1, inFM, inBody)
	if err != nil {
		t.Fatalf("WriteChapter: %v", err)
	}

	wantPath := filepath.Join(bookDir, "chapters", "01-01.md")
	if path != wantPath {
		t.Errorf("path = %s, want %s", path, wantPath)
	}

	outFM, outBody, err := ReadChapter(path)
	if err != nil {
		t.Fatalf("ReadChapter: %v", err)
	}

	if outFM.Title != inFM.Title {
		t.Errorf("Title = %q, want %q", outFM.Title, inFM.Title)
	}
	if outFM.Status != inFM.Status {
		t.Errorf("Status = %q, want %q", outFM.Status, inFM.Status)
	}
	if outFM.WordCount != inFM.WordCount {
		t.Errorf("WordCount = %d, want %d", outFM.WordCount, inFM.WordCount)
	}
	if len(outFM.Citations) != len(inFM.Citations) {
		t.Errorf("Citations len = %d, want %d", len(outFM.Citations), len(inFM.Citations))
	}
	if !strings.Contains(outBody, "正文段落") {
		t.Errorf("body missing expected content; got: %s", outBody)
	}
}

func TestChapterPathFormat(t *testing.T) {
	got := ChapterPath("/ws/books/my-book", 1, 1)
	want := "/ws/books/my-book/chapters/01-01.md"
	if got != want {
		t.Errorf("ChapterPath = %s, want %s", got, want)
	}

	got = ChapterPath("/ws/books/my-book", 12, 7)
	want = "/ws/books/my-book/chapters/12-07.md"
	if got != want {
		t.Errorf("ChapterPath = %s, want %s", got, want)
	}
}

func TestReadChapterMissingFile(t *testing.T) {
	_, _, err := ReadChapter("/nonexistent/01-01.md")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist, got: %v", err)
	}
}
