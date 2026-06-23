package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/provider/llm"
)

func TestParseChapterAddrValid(t *testing.T) {
	cases := []struct {
		in       string
		wantPart int
		wantCh   int
	}{
		{"01-01", 1, 1},
		{"1-1", 1, 1},
		{"12-07", 12, 7},
		{"99-99", 99, 99},
	}
	for _, c := range cases {
		gotPart, gotCh, err := parseChapterAddr(c.in)
		if err != nil {
			t.Errorf("parseChapterAddr(%q) err: %v", c.in, err)
			continue
		}
		if gotPart != c.wantPart || gotCh != c.wantCh {
			t.Errorf("parseChapterAddr(%q) = (%d, %d), want (%d, %d)",
				c.in, gotPart, gotCh, c.wantPart, c.wantCh)
		}
	}
}

func TestParseChapterAddrInvalid(t *testing.T) {
	cases := []string{
		"",
		"1",
		"1-",
		"-1",
		"1-1-1",
		"abc",
		"01-00", // chapter 0 invalid
		"00-01", // part 0 invalid
		"01-ab",
	}
	for _, c := range cases {
		_, _, err := parseChapterAddr(c)
		if err == nil {
			t.Errorf("parseChapterAddr(%q) expected error, got nil", c)
		}
	}
}

func TestExpandCmdShape(t *testing.T) {
	cmd := newExpandCmd()
	if cmd.Use != "expand <slug> <NN-MM>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "expand <slug> <NN-MM>")
	}
	if cmd.Short == "" {
		t.Error("Short is empty")
	}
	// Args validation: cobra.ExactArgs(2)
	if cmd.Args == nil {
		t.Error("Args validator is nil")
	}
	// --force flag exists
	if cmd.Flags().Lookup("force") == nil {
		t.Error("--force flag missing")
	}
	// --force2 should NOT exist (replaced by count flag)
	if cmd.Flags().Lookup("force2") != nil {
		t.Error("--force2 flag should not exist (replaced by count)")
	}
}

func TestExpandCmdArgsValidation(t *testing.T) {
	cmd := newExpandCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	cases := [][]string{
		{"only-one-arg"},
		{"too", "many", "args"},
	}
	for _, args := range cases {
		err := cmd.Args(cmd, args)
		if err == nil {
			t.Errorf("expected error for args %v, got nil", args)
		}
	}

	// Valid: exactly 2 args
	if err := cmd.Args(cmd, []string{"my-book", "01-01"}); err != nil {
		t.Errorf("expected success for 2 args, got: %v", err)
	}
}

func TestExpandRunHappyPath(t *testing.T) {
	// 1. Set up temp workspace with a book containing one scaffolded chapter.
	tmp := t.TempDir()
	wsRoot := tmp
	// Create workspace marker
	if err := os.MkdirAll(filepath.Join(wsRoot, ".jianwu"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsRoot, ".jianwu", "schema_version"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bookDir := filepath.Join(wsRoot, "books", "test-book")
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := &book.Meta{
		ID: "test-id", Slug: "test-book", Title: "Test Book",
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
				{
					Index: 1, Title: "Chapter 1", Status: book.StatusScaffolded,
					Abstract: "An abstract", KeyConcepts: []string{"concept1"},
				},
			}},
		},
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
		t.Fatal(err)
	}

	// 2. Set up mock providerDepsHook returning mock providers.
	original := providerDepsHook
	defer func() { providerDepsHook = original }()

	// Mock chatter returns 3 scripted responses for the 3 iterations:
	// iter 1 (research): JSON ResearchNotes
	// iter 2 (draft): markdown with footnote
	// iter 3 (validate): JSON ValidationResult
	chatter := &countingChatter{
		responses: []llm.ChatResponse{
			{Content: `{"findings":[],"candidates":[]}`},                                                // research
			{Content: "## 标题\n\n正文...[^1]\n\n[^1]: [Example](https://example.com) accessed 2026-06-22"}, // draft
			{Content: `{"revised_markdown":"## 标题\n\n正文...[^1]\n\n[^1]: [Example](https://example.com) accessed 2026-06-22","claims":[{"text":"claim1","has_citation":true}]}`}, // validate
		},
	}

	providerDepsHook = func(_ *config.Config, _ *config.Secrets) (*ProviderDeps, error) {
		return &ProviderDeps{
			Chatter:  chatter,
			Searcher: &stubSearcher{},
			Reader:   &stubReader{},
			Embedder: &stubEmbedder{},
		}, nil
	}

	// 3. Build the command and run it.
	cmd := newExpandCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"test-book", "01-01"})

	// Run from the workspace root.
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(wsRoot); err != nil {
		t.Fatal(err)
	}

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// 4. Verify chapter file was written.
	chapPath := filepath.Join(bookDir, "chapters", "01-01.md")
	if _, err := os.Stat(chapPath); err != nil {
		t.Fatalf("chapter file not created: %v", err)
	}
	fm, body, err := book.ReadChapter(chapPath)
	if err != nil {
		t.Fatalf("ReadChapter: %v", err)
	}
	if fm.Status != book.StatusExpanded {
		t.Errorf("frontmatter.Status = %q, want %q", fm.Status, book.StatusExpanded)
	}
	if !strings.Contains(body, "正文") {
		t.Errorf("body missing expected content: %s", body)
	}

	// 5. Verify outline.json was updated.
	updated, err := book.LoadOutline(filepath.Join(bookDir, "outline.json"))
	if err != nil {
		t.Fatalf("LoadOutline: %v", err)
	}
	ch := updated.Parts[0].Chapters[0]
	if ch.Status != book.StatusExpanded {
		t.Errorf("outline chapter status = %q, want %q", ch.Status, book.StatusExpanded)
	}
	if ch.WordCount == 0 {
		t.Error("WordCount not set")
	}
}

func TestExpandRunRefusesWithoutForce(t *testing.T) {
	// Create a chapter that's already expanded.
	tmp := t.TempDir()
	// Create workspace marker
	if err := os.MkdirAll(filepath.Join(tmp, ".jianwu"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".jianwu", "schema_version"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bookDir := filepath.Join(tmp, "books", "test-book")
	if err := os.MkdirAll(filepath.Join(bookDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	existingFM := book.ChapterFrontmatter{
		Title: "Existing", PartIndex: 1, ChapterIndex: 1,
		Status: book.StatusExpanded, WordCount: 100,
		GeneratedAt: time.Now().UTC(), Model: "glm-4.6", EngineVersion: "v0.1.1",
	}
	if _, err := book.WriteChapter(bookDir, 1, 1, existingFM, "old content"); err != nil {
		t.Fatal(err)
	}
	meta := &book.Meta{ID: "x", Slug: "test-book", Title: "Test", Status: book.BookStatusDraft}
	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"), meta); err != nil {
		t.Fatal(err)
	}
	outline := &book.Outline{
		Parts: []book.OutlinePart{
			{Index: 1, Chapters: []book.OutlineChapter{{Index: 1, Title: "C1", Status: book.StatusExpanded}}},
		},
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	cmd := newExpandCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"test-book", "01-01"})
	// No --force
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Should NOT have overwritten the file.
	_, body, _ := book.ReadChapter(book.ChapterPath(bookDir, 1, 1))
	if !strings.Contains(body, "old content") {
		t.Errorf("file was overwritten without --force; body: %s", body)
	}
}

func TestExpandRunRefusesReviewedEvenWithForce(t *testing.T) {
	// Chapter status = reviewed; --force alone should refuse; --force --force2 should allow.
	tmp := t.TempDir()
	// Create workspace marker
	if err := os.MkdirAll(filepath.Join(tmp, ".jianwu"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".jianwu", "schema_version"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bookDir := filepath.Join(tmp, "books", "test-book")
	if err := os.MkdirAll(filepath.Join(bookDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	existingFM := book.ChapterFrontmatter{
		Title: "Reviewed", PartIndex: 1, ChapterIndex: 1,
		Status: book.StatusReviewed, WordCount: 100,
		GeneratedAt: time.Now().UTC(), Model: "glm-4.6", EngineVersion: "v0.1.1",
	}
	if _, err := book.WriteChapter(bookDir, 1, 1, existingFM, "reviewed content"); err != nil {
		t.Fatal(err)
	}
	meta := &book.Meta{ID: "x", Slug: "test-book", Title: "Test", Status: book.BookStatusDraft}
	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"), meta); err != nil {
		t.Fatal(err)
	}
	outline := &book.Outline{
		Parts: []book.OutlinePart{
			{Index: 1, Chapters: []book.OutlineChapter{{Index: 1, Title: "C1", Status: book.StatusReviewed}}},
		},
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	// With --force only: should refuse.
	cmd := newExpandCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"test-book", "01-01", "--force"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with --force on reviewed chapter")
	}
	_, body, _ := book.ReadChapter(book.ChapterPath(bookDir, 1, 1))
	if !strings.Contains(body, "reviewed content") {
		t.Errorf("file was overwritten with --force alone on reviewed chapter")
	}
}
func TestExpandRunAllowWithDoubleForce(t *testing.T) {
	// Chapter status = reviewed; --force --force (forceCount=2) should allow overwrite.
	tmp := t.TempDir()
	// Create workspace marker
	if err := os.MkdirAll(filepath.Join(tmp, ".jianwu"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".jianwu", "schema_version"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bookDir := filepath.Join(tmp, "books", "test-book")
	if err := os.MkdirAll(filepath.Join(bookDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	existingFM := book.ChapterFrontmatter{
		Title: "Reviewed", PartIndex: 1, ChapterIndex: 1,
		Status: book.StatusReviewed, WordCount: 100,
		GeneratedAt: time.Now().UTC(), Model: "glm-4.6", EngineVersion: "v0.1.1",
	}
	if _, err := book.WriteChapter(bookDir, 1, 1, existingFM, "old reviewed content"); err != nil {
		t.Fatal(err)
	}
	meta := &book.Meta{
		ID: "x", Slug: "test-book", Title: "Test", Status: book.BookStatusDraft,
		Archetype:  "ontology-epistemology-practice",
		Parameters: book.Parameters{Audience: "scholar", Depth: "advanced", Goal: "understanding", Length: "short"},
	}
	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"), meta); err != nil {
		t.Fatal(err)
	}
	outline := &book.Outline{
		Parts: []book.OutlinePart{
			{Index: 1, Chapters: []book.OutlineChapter{{Index: 1, Title: "C1", Status: book.StatusReviewed}}},
		},
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
		t.Fatal(err)
	}

	// Inject mock deps so expand.Generate runs without real API.
	original := providerDepsHook
	defer func() { providerDepsHook = original }()
	chatter := &countingChatter{
		responses: []llm.ChatResponse{
			{Content: `{"findings":[],"candidates":[]}`},
			{Content: "## New\n\nnew content after force-force...[^1]\n\n[^1]: [X](https://x.com) accessed 2026-06-22"},
			{Content: `{"revised_markdown":"## New\n\nnew content after force-force...[^1]\n\n[^1]: [X](https://x.com) accessed 2026-06-22","claims":[{"text":"x","has_citation":true}]}`},
		},
	}
	providerDepsHook = func(_ *config.Config, _ *config.Secrets) (*ProviderDeps, error) {
		return &ProviderDeps{Chatter: chatter, Searcher: &stubSearcher{}, Reader: &stubReader{}, Embedder: &stubEmbedder{}}, nil
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	cmd := newExpandCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"test-book", "01-01", "--force", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected success with --force --force on reviewed chapter, got: %v", err)
	}

	// Verify file was overwritten.
	_, body, err := book.ReadChapter(book.ChapterPath(bookDir, 1, 1))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "new content after force-force") {
		t.Errorf("file was not overwritten; body: %s", body)
	}
}
