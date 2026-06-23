// internal/cli/review_test.go
package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/zhurong/jianwu/internal/book"
)

func TestReview_ExpandedToReviewed(t *testing.T) {
	tmp := writeMinimalBook(t, "demo")
	bookDir := filepath.Join(tmp, "books", "demo")
	if _, err := book.WriteChapter(bookDir, 1, 1, book.ChapterFrontmatter{
		Title: "第一章", PartIndex: 1, ChapterIndex: 1, Status: book.StatusExpanded,
	}, "正文"); err != nil {
		t.Fatal(err)
	}
	chdir(t, tmp)

	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})
	if err := runReview(cmd, []string{"demo", "1-1"}); err != nil {
		t.Fatalf("runReview: %v", err)
	}

	o, _ := book.LoadOutline(filepath.Join(bookDir, "outline.json"))
	ch := o.Parts[0].Chapters[0]
	if ch.Status != book.StatusReviewed {
		t.Errorf("outline status = %q, want reviewed", ch.Status)
	}
	if ch.ReviewedAt == nil {
		t.Error("ReviewedAt not set")
	}
	if ch.ReviewedBy == "" {
		t.Error("ReviewedBy not set")
	}
	fm, _, _ := book.ReadChapter(book.ChapterPath(bookDir, 1, 1))
	if fm.Status != book.StatusReviewed {
		t.Errorf("frontmatter status = %q, want reviewed (mirror)", fm.Status)
	}
}

func TestReview_RejectsNonExpanded(t *testing.T) {
	for _, st := range []string{book.StatusScaffolded, book.StatusReviewed, book.StatusFinal, book.StatusFailed} {
		tmp := writeMinimalBook(t, "demo")
		bookDir := filepath.Join(tmp, "books", "demo")
		// Set outline chapter to the non-expanded status.
		o, _ := book.LoadOutline(filepath.Join(bookDir, "outline.json"))
		o.Parts[0].Chapters[0].Status = st
		_ = book.SaveOutline(filepath.Join(bookDir, "outline.json"), o)
		chdir(t, tmp)

		cmd := &cobra.Command{}
		cmd.SetOut(&strings.Builder{})
		err := runReview(cmd, []string{"demo", "1-1"})
		if err == nil {
			t.Errorf("status %q: expected rejection, got nil", st)
		}
	}
}
