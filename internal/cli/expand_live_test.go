package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/zhurong/jianwu/internal/book"
)

// TestExpandLive runs the expand command against real LLM/search/reader APIs.
// SKIP if any required API key is missing.
//
// Manual run:
//
//	GEMINI_API_KEY=... GLM_API_KEY=... BRAVE_API_KEY=... \
//	  go test -run TestExpandLive ./internal/cli/ -v -timeout 10m
func TestExpandLive(t *testing.T) {
	keys := []string{"GEMINI_API_KEY", "GLM_API_KEY", "BRAVE_API_KEY"}
	for _, k := range keys {
		if os.Getenv(k) == "" {
			t.Skipf("skipping: %s not set", k)
		}
	}

	// 1. Initialize workspace.
	wsRoot := t.TempDir()
	initCmd := NewRootCmd()
	initCmd.SetArgs([]string{"init", wsRoot})
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	if err := initCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// 2. Create a small book manually.
	bookDir := filepath.Join(wsRoot, "books", "live-test")
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := &book.Meta{
		ID: "live", Slug: "live-test", Title: "什么是现实",
		Archetype: "ontology-epistemology-practice", Language: "zh",
		Status:     book.BookStatusDraft,
		Parameters: book.Parameters{Audience: "scholar", Depth: "advanced", Goal: "understanding", Length: "short"},
	}
	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"), meta); err != nil {
		t.Fatal(err)
	}
	outline := &book.Outline{
		Parts: []book.OutlinePart{
			{Index: 1, Title: "本体", Role: "ontology", Chapters: []book.OutlineChapter{
				{Index: 1, Title: "现实作为边界", Status: book.StatusScaffolded,
					Abstract:    "探讨现实作为认识论的边界条件",
					KeyConcepts: []string{"现实", "边界", "认识论"}},
			}},
		},
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
		t.Fatal(err)
	}

	// 3. Run expand (real providers, no hook override).
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(wsRoot); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"expand", "live-test", "01-01"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expand live: %v\noutput: %s", err, out.String())
	}

	// 4. Verify chapter file exists.
	chapPath := filepath.Join(bookDir, "chapters", "01-01.md")
	if _, err := os.Stat(chapPath); err != nil {
		t.Fatalf("chapter file missing: %v", err)
	}
	fm, body, err := book.ReadChapter(chapPath)
	if err != nil {
		t.Fatal(err)
	}
	if fm.WordCount < 100 {
		t.Errorf("WordCount = %d, expected at least 100 for a real LLM call", fm.WordCount)
	}
	if len(body) == 0 {
		t.Error("body is empty")
	}
	t.Logf("expand live OK: %d words, %d citations, %d unverified",
		fm.WordCount, len(fm.Citations), fm.UnverifiedClaimsCount)
}
