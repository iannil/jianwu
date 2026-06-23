// internal/cli/status_test.go
package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/zhurong/jianwu/internal/book"
)

func TestStatus_FailedSurfaced(t *testing.T) {
	tmp := writeBookWithChapters(t, "demo", book.StatusFailed, book.StatusReviewed)
	chdir(t, tmp)

	var buf strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	if err := runStatus(cmd, []string{"demo"}); err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "failed 1") {
		t.Errorf("summary missing failed count:\n%s", s)
	}
	if !strings.Contains(s, "re-run expand") {
		t.Errorf("missing failed next-action hint:\n%s", s)
	}
}

func TestStatus_TreeAndCounts(t *testing.T) {
	tmp := writeBookWithChapters(t, "demo", book.StatusReviewed, book.StatusExpanded)
	chdir(t, tmp)

	var buf strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	if err := runStatus(cmd, []string{"demo"}); err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "测试书") {
		t.Error("missing book title")
	}
	if !strings.Contains(s, "01-01") || !strings.Contains(s, "01-02") {
		t.Error("missing chapter address lines")
	}
	if !strings.Contains(s, "reviewed") || !strings.Contains(s, "expanded") {
		t.Error("missing per-chapter statuses")
	}
	// Summary counts present.
	if !strings.Contains(s, "reviewed 1") || !strings.Contains(s, "expanded 1") {
		t.Errorf("missing summary counts:\n%s", s)
	}
}
