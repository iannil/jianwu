package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/iannil/jianwu/internal/storage"
	"github.com/iannil/jianwu/internal/workspace"
)

func TestCorpusListBuiltinOnly(t *testing.T) {
	var buf strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	if err := runCorpusList(cmd); err != nil {
		t.Fatalf("runCorpusList: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "reality-construction") {
		t.Errorf("expected reality-construction in output:\n%s", s)
	}
	if !strings.Contains(s, "builtin") {
		t.Errorf("expected 'builtin' origin marker:\n%s", s)
	}
}

func TestCorpusListWithWorkspaceOverride(t *testing.T) {
	tmp := t.TempDir()
	wsDir := filepath.Join(tmp, "ws")
	createWorkspace(t, wsDir)
	addCorpusBook(t, wsDir, "reality-construction", `{
		"slug": "reality-construction",
		"title": {"zh": "覆盖版", "en": "Override"},
		"archetype": "ontology-epistemology-practice",
		"audience": "educated-general",
		"depth": "advanced",
		"goal": "understanding",
		"length": "long",
		"language": ["zh"],
		"source": {"name": "user", "url": "", "accessed_at": "2026-06-28"},
		"abstract": "override",
		"parts": []
	}`)

	oldDir := cliWorkspaceDir
	cliWorkspaceDir = wsDir
	defer func() { cliWorkspaceDir = oldDir }()

	var buf strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	if err := runCorpusList(cmd); err != nil {
		t.Fatalf("runCorpusList: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "workspace") {
		t.Errorf("expected workspace indicator:\n%s", s)
	}
}

func TestCorpusShowExisting(t *testing.T) {
	var buf strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	if err := runCorpusShow(cmd, "reality-construction"); err != nil {
		t.Fatalf("runCorpusShow: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "实在建构") {
		t.Errorf("expected title in output:\n%s", s)
	}
	if !strings.Contains(s, "Parts:") {
		t.Errorf("expected Parts: in output:\n%s", s)
	}
}

func TestCorpusShowMissing(t *testing.T) {
	var buf strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	err := runCorpusShow(cmd, "nonexistent-book")
	if err == nil {
		t.Fatal("expected error for missing book")
	}
	if ie, ok := err.(*InfoError); ok {
		if ie.Code != ExitCodeGeneric {
			t.Errorf("expected ExitCodeGeneric, got %d", ie.Code)
		}
	} else {
		t.Errorf("expected *InfoError, got %T", err)
	}
}

func TestCorpusStats(t *testing.T) {
	var buf strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	if err := runCorpusStats(cmd); err != nil {
		t.Fatalf("runCorpusStats: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "Total books:") {
		t.Errorf("expected Total books: in output:\n%s", s)
	}
	if !strings.Contains(s, "Archetype distribution:") {
		t.Errorf("expected archetype distribution:\n%s", s)
	}
}

func TestCorpusSync(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "source")
	wsDir := filepath.Join(tmp, "ws")

	createWorkspace(t, wsDir)
	storage.OS.MkdirAll(srcDir, 0o755)

	// Valid book
	validBook := `{
		"slug": "new-book",
		"title": {"zh": "新书", "en": "New Book"},
		"archetype": "ontology-epistemology-practice",
		"audience": "educated-general",
		"depth": "intermediate",
		"goal": "understanding",
		"length": "medium",
		"language": ["zh"],
		"source": {"name": "user", "url": "", "accessed_at": "2026-06-28"},
		"abstract": "一本测试新书",
		"parts": []
	}`
	storage.OS.WriteFile(filepath.Join(srcDir, "new-book.json"), []byte(validBook), 0o644)

	// Invalid: missing slug
	invalidBook := `{"title": {"zh": "无slug"}}`
	storage.OS.WriteFile(filepath.Join(srcDir, "no-slug.json"), []byte(invalidBook), 0o644)

	oldDir := cliWorkspaceDir
	cliWorkspaceDir = wsDir
	defer func() { cliWorkspaceDir = oldDir }()

	var buf strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	// Bypass the cobra flag by calling the core function directly
	if err := runCorpusSync(cmd, srcDir); err != nil {
		t.Fatalf("runCorpusSync: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "Synced 1") {
		t.Errorf("expected 1 synced book:\n%s", s)
	}
	if !strings.Contains(s, "Errors (1)") {
		t.Errorf("expected 1 error for invalid book:\n%s", s)
	}

	// Verify the book was written
	corpusDir := filepath.Join(wsDir, ".jianwu", "corpus")
	data, err := storage.OS.ReadFile(filepath.Join(corpusDir, "new-book.json"))
	if err != nil {
		t.Fatalf("expected synced book: %v", err)
	}
	if !strings.Contains(string(data), "新书") {
		t.Errorf("expected new book content, got: %s", string(data))
	}
}

func TestCorpusReindex(t *testing.T) {
	tmp := t.TempDir()
	wsDir := filepath.Join(tmp, "ws")
	createWorkspace(t, wsDir)

	oldDir := cliWorkspaceDir
	cliWorkspaceDir = wsDir
	defer func() { cliWorkspaceDir = oldDir }()

	var buf strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	// Without API keys, the embedder factory will fail with an LLM provider error.
	// That validates the workspace + corpus loading succeeded (would return different errors).
	err := runCorpusReindex(cmd, "")
	if err == nil {
		t.Fatal("expected error (no API keys in test)")
	}
	if ie, ok := err.(*InfoError); ok {
		if ie.Code != ExitCodeLLMProvider {
			t.Errorf("expected ExitCodeLLMProvider, got %d (%v)", ie.Code, ie)
		}
	} else {
		t.Errorf("expected *InfoError, got %T: %v", err, err)
	}
}

// --- test helpers (not in other files) ---

func createWorkspace(t *testing.T, dir string) {
	t.Helper()
	if err := workspace.Init(dir, workspace.InitOpts{Bare: true}); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
}

func addCorpusBook(t *testing.T, wsRoot, slug, content string) {
	t.Helper()
	corpusDir := filepath.Join(wsRoot, ".jianwu", "corpus")
	if err := storage.OS.MkdirAll(corpusDir, 0o755); err != nil {
		t.Fatalf("mkdir corpus: %v", err)
	}
	if err := storage.OS.WriteFile(filepath.Join(corpusDir, slug+".json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write corpus: %v", err)
	}
}

// syncCorpusFrom is a test helper that extracts the from flag from a sync cmd
// and runs the core logic, since cobra flags aren't set when calling runXxx directly.
func syncCorpusFrom(cmd *cobra.Command, from string) error {
	return runCorpusSync(cmd, from)
}
