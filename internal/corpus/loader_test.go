package corpus

import (
	"path/filepath"
	"testing"

	"github.com/iannil/jianwu/internal/storage"
)

func writeCorpusFile(t *testing.T, wsDir, name, content string) {
	t.Helper()
	dir := CorpusDir(wsDir)
	if err := storage.OS.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := storage.OS.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadWorkspaceOnly(t *testing.T) {
	t.Parallel()
	wsDir := t.TempDir()
	writeCorpusFile(t, wsDir, "book-a.json", `{
		"slug": "book-a",
		"title": {"zh": "书 A", "en": "Book A"},
		"archetype": "ontology-epistemology-practice",
		"audience": "educated-general",
		"depth": "intermediate",
		"goal": "understanding",
		"length": "medium",
		"language": ["zh"],
		"source": {"name": "web", "url": "https://example.com/a", "accessed_at": "2026-10-01"},
		"abstract": "摘要",
		"parts": [{"index": 1, "title": {"zh": "第一部分"}, "role": "ontology", "chapters": [{"index": 1, "title": {"zh": "第一章"}}]}]
	}`)

	m, err := Load(wsDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m) != 1 {
		t.Fatalf("got %d books, want 1", len(m))
	}
	b := m["book-a"]
	if b == nil || b.Title.Zh != "书 A" {
		t.Fatalf("book-a = %+v", b)
	}
	if len(b.Parts) != 1 || len(b.Parts[0].Chapters) != 1 {
		t.Errorf("parts/chapters = %+v", b.Parts)
	}
}

func TestLoadMissingDirYieldsEmpty(t *testing.T) {
	t.Parallel()
	// No .jianwu/corpus directory and empty wsRoot both yield empty maps.
	for _, ws := range []string{t.TempDir(), ""} {
		m, err := Load(ws)
		if err != nil {
			t.Fatalf("Load(%q): %v", ws, err)
		}
		if len(m) != 0 {
			t.Errorf("Load(%q) = %d books, want 0", ws, len(m))
		}
	}
}

func TestLoadInvalidReported(t *testing.T) {
	t.Parallel()
	wsDir := t.TempDir()
	writeCorpusFile(t, wsDir, "bad.json", `{"title": {"zh": "无 slug"}}`)
	if _, err := Load(wsDir); err == nil {
		t.Fatal("expected error for book with empty slug")
	}
}

func TestSaveBookRoundTrip(t *testing.T) {
	t.Parallel()
	wsDir := t.TempDir()
	in := &Book{
		Slug: "book-b", Title: LocalizedTitle{Zh: "书 B", En: "Book B"},
		Archetype: "mindset-method-practice", Audience: "scholar",
		Depth: "advanced", Goal: "understanding", Length: "long",
		Language: []string{"zh"},
		Source:   Source{Name: "web", URL: "https://example.com/b", AccessedAt: "2026-10-01"},
		Abstract: "采集的摘要",
		Parts: []Part{{
			Index: 1, Title: LocalizedTitle{Zh: "第一部分"}, Role: "ontology",
			Chapters: []Chapter{{Index: 1, Title: LocalizedTitle{Zh: "第一章"}}},
		}},
	}
	if err := SaveBook(wsDir, in); err != nil {
		t.Fatalf("SaveBook: %v", err)
	}

	// File exists where expected, and reloads identically.
	data, err := storage.OS.ReadFile(BookPath(wsDir, "book-b"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty corpus file")
	}
	m, err := Load(wsDir)
	if err != nil {
		t.Fatal(err)
	}
	out := m["book-b"]
	if out == nil || out.Title.En != "Book B" || len(out.Parts) != 1 {
		t.Fatalf("round trip mismatch: %+v", out)
	}

	// SaveBook rejects empty slug.
	if err := SaveBook(wsDir, &Book{}); err == nil {
		t.Error("expected error for empty slug")
	}
}
