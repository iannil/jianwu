package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
)

// TestE2EExpandCommandWithMocks runs the full `jianwu expand` CLI surface
// against mocked providers injected via providerDepsHook (per Q20=B).
func TestE2EExpandCommandWithMocks(t *testing.T) {
	// 1. Initialize a workspace via init command.
	wsRoot := t.TempDir()
	initCmd := NewRootCmd()
	initCmd.SetArgs([]string{"init", wsRoot})
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	if err := initCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// 2. Create a book manually with one scaffolded chapter.
	bookDir := filepath.Join(wsRoot, "books", "e2e-book")
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := &book.Meta{
		ID: "e2e-id", Slug: "e2e-book", Title: "E2E Book",
		Archetype: "ontology-epistemology-practice", Language: "zh",
		Status:     book.BookStatusDraft,
		Parameters: book.Parameters{Audience: "scholar", Depth: "advanced", Goal: "understanding", Length: "medium"},
	}
	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"), meta); err != nil {
		t.Fatal(err)
	}
	outline := &book.Outline{
		Parts: []book.OutlinePart{
			{Index: 1, Title: "Part 1", Role: "ontology", Chapters: []book.OutlineChapter{
				{Index: 1, Title: "Chapter 1", Status: book.StatusScaffolded,
					Abstract: "Abstract", KeyConcepts: []string{"c1"}},
			}},
		},
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
		t.Fatal(err)
	}

	// 3. Set fake API keys + build mock deps.
	t.Setenv("GEMINI_API_KEY", "fake")
	t.Setenv("GLM_API_KEY", "fake")
	t.Setenv("BRAVE_API_KEY", "fake")

	chatter := &countingChatter{
		responses: []llm.ChatResponse{
			{Content: `{"findings":[],"candidates":[]}`},
			{Content: "## Chapter 1\n\nBody text...[^1]\n\n[^1]: [Example](https://example.com) accessed 2026-06-22"},
			{Content: `{"revised_markdown":"## Chapter 1\n\nBody text...[^1]\n\n[^1]: [Example](https://example.com) accessed 2026-06-22","claims":[{"text":"claim","has_citation":true}]}`},
		},
	}
	mockDeps := &ProviderDeps{
		Chatter:  chatter,
		Searcher: &stubSearcher{},
		Reader:   &stubReader{},
		Embedder: &stubEmbedder{},
	}

	// 4. Run the expand command from inside the workspace.
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(wsRoot); err != nil {
		t.Fatal(err)
	}

	cmd := newExpandCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := runExpand(cmd, []string{"e2e-book", "01-01"}, 0, mockDeps, false); err != nil {
		t.Fatalf("expand command: %v", err)
	}

	// 5. Verify chapter file exists with correct frontmatter.
	chapPath := filepath.Join(bookDir, "chapters", "01-01.md")
	if _, err := os.Stat(chapPath); err != nil {
		t.Fatalf("chapter file missing: %v", err)
	}
	fm, _, err := book.ReadChapter(chapPath)
	if err != nil {
		t.Fatalf("ReadChapter: %v", err)
	}
	if fm.Status != book.StatusExpanded {
		t.Errorf("frontmatter status = %q, want %q", fm.Status, book.StatusExpanded)
	}

	// 6. Verify outline.json chapter status updated.
	updated, err := book.LoadOutline(filepath.Join(bookDir, "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	if updated.Parts[0].Chapters[0].Status != book.StatusExpanded {
		t.Errorf("outline chapter status = %q, want %q",
			updated.Parts[0].Chapters[0].Status, book.StatusExpanded)
	}
}
