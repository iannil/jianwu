// internal/cli/export_test.go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/zhurong/jianwu/internal/book"
)

func TestExport_AssemblesWithGlobalFootnotes(t *testing.T) {
	tmp := writeBookWithChapters(t, "demo", book.StatusFinal, book.StatusFinal)
	bookDir := filepath.Join(tmp, "books", "demo")
	// Overwrite chapter bodies with per-chapter [^1] footnotes to test global renumber.
	_, _ = book.WriteChapter(bookDir, 1, 1, book.ChapterFrontmatter{Title: "第一章", PartIndex: 1, ChapterIndex: 1, Status: book.StatusFinal}, "甲[^1]。\n\n[^1]: 来源甲")
	_, _ = book.WriteChapter(bookDir, 1, 2, book.ChapterFrontmatter{Title: "第二章", PartIndex: 1, ChapterIndex: 2, Status: book.StatusFinal}, "乙[^1]。\n\n[^1]: 来源乙")
	// Give chapters distinct outline titles (export headings come from the outline).
	o, _ := book.LoadOutline(filepath.Join(bookDir, "outline.json"))
	o.Parts[0].Chapters[0].Title = "第一章"
	o.Parts[0].Chapters[1].Title = "第二章"
	_ = book.SaveOutline(filepath.Join(bookDir, "outline.json"), o)
	chdir(t, tmp)

	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})
	if err := runExport(cmd, []string{"demo"}, "md", false); err != nil {
		t.Fatalf("runExport: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(bookDir, "export", "demo.md"))
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "# 测试书") {
		t.Error("missing title page")
	}
	if !strings.Contains(s, "### 第一章") || !strings.Contains(s, "### 第二章") {
		t.Error("missing chapter headings")
	}
	// Global footnotes: ch1 -> [^1], ch2 -> [^2], no duplicate [^1] def.
	if !strings.Contains(s, "甲[^1]。") || !strings.Contains(s, "乙[^2]。") {
		t.Errorf("footnotes not globally renumbered:\n%s", s)
	}
	if strings.Count(s, "[^1]: ") != 1 {
		t.Errorf("expected exactly one [^1] definition, got %d", strings.Count(s, "[^1]: "))
	}
}

func TestExport_PlaceholderForMissingChapter(t *testing.T) {
	tmp := writeMinimalBook(t, "demo") // outline has 1 chapter (expanded) but no .md written
	chdir(t, tmp)
	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})
	if err := runExport(cmd, []string{"demo"}, "md", false); err != nil {
		t.Fatalf("runExport: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(tmp, "books", "demo", "export", "demo.md"))
	if !strings.Contains(string(data), "本章尚未展开") {
		t.Errorf("missing placeholder for un-expanded chapter:\n%s", string(data))
	}
}

func TestExport_DryRunNoFile(t *testing.T) {
	tmp := writeBookWithChapters(t, "demo", book.StatusFinal)
	chdir(t, tmp)
	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})
	if err := runExport(cmd, []string{"demo"}, "md", true); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "books", "demo", "export", "demo.md")); !os.IsNotExist(err) {
		t.Error("dry-run must not write the export file")
	}
}

func TestExport_RejectsNonMdTarget(t *testing.T) {
	tmp := writeBookWithChapters(t, "demo", book.StatusFinal)
	chdir(t, tmp)
	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})
	if err := runExport(cmd, []string{"demo"}, "pdf", false); err == nil {
		t.Fatal("expected rejection for non-md target")
	}
}
