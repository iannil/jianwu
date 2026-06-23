// internal/cli/finalize_test.go
package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/iannil/jianwu/internal/book"
	"github.com/spf13/cobra"
)

func writeBookWithChapters(t *testing.T, slug string, statuses ...string) string {
	t.Helper()
	tmp := writeMinimalBook(t, slug)
	bookDir := filepath.Join(tmp, "books", slug)
	chs := make([]book.OutlineChapter, len(statuses))
	for i, st := range statuses {
		chs[i] = book.OutlineChapter{Index: i + 1, Title: "章", Status: st}
		_, _ = book.WriteChapter(bookDir, 1, i+1, book.ChapterFrontmatter{
			Title: "章", PartIndex: 1, ChapterIndex: i + 1, Status: st,
		}, "正文")
	}
	_ = book.SaveOutline(filepath.Join(bookDir, "outline.json"), &book.Outline{
		Parts: []book.OutlinePart{{Index: 1, Title: "P1", Chapters: chs}},
	})
	return tmp
}

func TestFinalize_AllReviewed(t *testing.T) {
	tmp := writeBookWithChapters(t, "demo", book.StatusReviewed, book.StatusReviewed)
	bookDir := filepath.Join(tmp, "books", "demo")
	chdir(t, tmp)

	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})
	if err := runFinalize(cmd, []string{"demo"}, false); err != nil {
		t.Fatalf("runFinalize: %v", err)
	}
	o, _ := book.LoadOutline(filepath.Join(bookDir, "outline.json"))
	for _, c := range o.Parts[0].Chapters {
		if c.Status != book.StatusFinal {
			t.Errorf("chapter %d status = %q, want final", c.Index, c.Status)
		}
	}
	m, _ := book.LoadMeta(filepath.Join(bookDir, "meta.json"))
	if m.Status != book.BookStatusFinal {
		t.Errorf("meta status = %q, want final", m.Status)
	}
	fm, _, _ := book.ReadChapter(book.ChapterPath(bookDir, 1, 1))
	if fm.Status != book.StatusFinal {
		t.Errorf("frontmatter not mirrored to final: %q", fm.Status)
	}
}

func TestFinalize_RejectsNonReviewed(t *testing.T) {
	tmp := writeBookWithChapters(t, "demo", book.StatusReviewed, book.StatusExpanded)
	chdir(t, tmp)
	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})
	if err := runFinalize(cmd, []string{"demo"}, false); err == nil {
		t.Fatal("expected rejection when a chapter is not reviewed")
	}
}

func TestFinalize_DryRunNoWrites(t *testing.T) {
	tmp := writeBookWithChapters(t, "demo", book.StatusReviewed)
	bookDir := filepath.Join(tmp, "books", "demo")
	chdir(t, tmp)
	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})
	if err := runFinalize(cmd, []string{"demo"}, true); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	m, _ := book.LoadMeta(filepath.Join(bookDir, "meta.json"))
	if m.Status != book.BookStatusDraft {
		t.Errorf("dry-run must not write; meta status = %q, want draft", m.Status)
	}
}
