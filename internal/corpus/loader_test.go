package corpus

import (
	"path/filepath"
	"testing"

	"github.com/iannil/jianwu/internal/storage"
)

func TestLoadReturnsAllSixBooks(t *testing.T) {
	m, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	want := []string{
		"reality-construction",
		"advancement-of-reality",
		"silent-games",
		"forced-convergence",
		"ai-engineer-in-action",
		"intelligent-computing-center-construction-guide",
	}
	if len(m) != len(want) {
		t.Fatalf("got %d books, want %d", len(m), len(want))
	}
	for _, slug := range want {
		if _, ok := m[slug]; !ok {
			t.Errorf("missing book %q", slug)
		}
	}
}

func TestLoadWithWorkspaceMergesOverride(t *testing.T) {
	t.Parallel()

	// Create a temp workspace with a corpus override
	tmp := t.TempDir()
	wsDir := filepath.Join(tmp, "ws")
	storage.OS.MkdirAll(filepath.Join(wsDir, ".jianwu", "corpus"), 0o755)

	// Write an override for reality-construction with a different abstract
	override := `{
  "slug": "reality-construction",
  "title": {"zh": "覆盖标题", "en": "Override Title"},
  "subtitle": null,
  "archetype": "ontology-epistemology-practice",
  "audience": "educated-general",
  "depth": "advanced",
  "goal": "understanding",
  "length": "long",
  "language": ["zh"],
  "source": {"name": "user", "url": "", "accessed_at": "2026-06-28"},
  "abstract": "这是用户覆盖的摘要",
  "parts": []
}`
	if err := storage.OS.WriteFile(filepath.Join(wsDir, ".jianwu", "corpus", "reality-construction.json"), []byte(override), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadWithWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWithWorkspace error: %v", err)
	}

	// Should have 6 builtin books + overridden book
	b, ok := m["reality-construction"]
	if !ok {
		t.Fatal("reality-construction not found")
	}
	if b.Abstract != "这是用户覆盖的摘要" {
		t.Errorf("expected overridden abstract, got %q", b.Abstract)
	}
	if b.Title.Zh != "覆盖标题" {
		t.Errorf("expected overridden title, got %q", b.Title.Zh)
	}

	// Other books should still exist from builtin
	if _, ok := m["advancement-of-reality"]; !ok {
		t.Error("advancement-of-reality missing from merged corpus")
	}
}

func TestLoadWithWorkspaceNoCorpusDir(t *testing.T) {
	t.Parallel()

	// Workspace without corpus/ directory should fall back to builtin only
	tmp := t.TempDir()
	wsDir := filepath.Join(tmp, "ws")
	storage.OS.MkdirAll(filepath.Join(wsDir, ".jianwu"), 0o755)

	m, err := LoadWithWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWithWorkspace error: %v", err)
	}
	if len(m) != 6 {
		t.Errorf("expected 6 builtin books, got %d", len(m))
	}
}

func TestBookHasPartsAndChapters(t *testing.T) {
	m, _ := Load()
	b := m["reality-construction"]
	if len(b.Parts) == 0 {
		t.Fatal("book has no parts")
	}
	if len(b.Parts[0].Chapters) == 0 {
		t.Error("first part has no chapters")
	}
}
