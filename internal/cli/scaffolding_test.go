package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
)

// scaffoldChatter returns one valid chapter-schema JSON response per call.
type scaffoldChatter struct{ calls int }

func (c *scaffoldChatter) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	c.calls++
	payload, _ := json.Marshal(map[string]any{
		"abstract":            "重试生成的摘要",
		"key_concepts":        []string{"概念A", "概念B"},
		"learning_objectives": []string{"目标A"},
		"suggested_examples":  []string{"示例A"},
	})
	return &llm.ChatResponse{Content: string(payload)}, nil
}

func newScaffoldingTestCmd() (*cobra.Command, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.Flags().Bool("retry-failed", false, "")
	cmd.Flags().Bool("tokens", false, "")
	return cmd, buf
}

func TestScaffolding_ListWithoutFlag(t *testing.T) {
	tmp := writeBookWithChapters(t, "demo", book.StatusFailed, book.StatusScaffolded)
	chdir(t, tmp)

	cmd, buf := newScaffoldingTestCmd()
	if err := runScaffoldingWithChatter(cmd, []string{"demo"}, &scaffoldChatter{}); err != nil {
		t.Fatalf("runScaffoldingWithChatter: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "1 failed scaffold(s)") || !strings.Contains(got, "--retry-failed") {
		t.Errorf("output should list failed scaffolds and hint --retry-failed, got: %s", got)
	}
}

// writeBookWithRoleAndChapters is writeBookWithChapters with a part role set
// (scaffold generation requires a non-empty part role, as real outlines have).
func writeBookWithRoleAndChapters(t *testing.T, slug string, statuses ...string) string {
	t.Helper()
	tmp := writeBookWithChapters(t, slug, statuses...)
	bookDir := filepath.Join(tmp, "books", slug)
	o, err := book.LoadOutline(filepath.Join(bookDir, "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	o.Parts[0].Role = "foundations"
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), o); err != nil {
		t.Fatal(err)
	}
	return tmp
}

func TestScaffolding_RetryRecoversFailed(t *testing.T) {
	tmp := writeBookWithRoleAndChapters(t, "demo", book.StatusFailed, book.StatusScaffolded)
	bookDir := filepath.Join(tmp, "books", "demo")
	chdir(t, tmp)

	chatter := &scaffoldChatter{}
	cmd, buf := newScaffoldingTestCmd()
	_ = cmd.Flags().Set("retry-failed", "true")
	if err := runScaffoldingWithChatter(cmd, []string{"demo"}, chatter); err != nil {
		t.Fatalf("runScaffoldingWithChatter: %v\noutput: %s", err, buf.String())
	}
	if chatter.calls != 1 {
		t.Errorf("expected 1 LLM call for the single failed chapter, got %d", chatter.calls)
	}
	if !strings.Contains(buf.String(), "1 recovered, 0 still failed") {
		t.Errorf("summary mismatch, got: %s", buf.String())
	}
	o, err := book.LoadOutline(filepath.Join(bookDir, "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	c1 := o.Parts[0].Chapters[0]
	if c1.Status != book.StatusScaffolded || c1.Abstract != "重试生成的摘要" {
		t.Errorf("chapter 1 not recovered: status=%q abstract=%q", c1.Status, c1.Abstract)
	}
	if o.Parts[0].Chapters[1].Status != book.StatusScaffolded {
		t.Errorf("untouched chapter changed status: %q", o.Parts[0].Chapters[1].Status)
	}
}

func TestScaffolding_NoFailedChapters(t *testing.T) {
	tmp := writeBookWithChapters(t, "demo", book.StatusScaffolded)
	chdir(t, tmp)

	cmd, buf := newScaffoldingTestCmd()
	if err := runScaffoldingWithChatter(cmd, []string{"demo"}, &scaffoldChatter{}); err != nil {
		t.Fatalf("runScaffoldingWithChatter: %v", err)
	}
	if !strings.Contains(buf.String(), "No failed scaffolds") {
		t.Errorf("expected no-failed message, got: %s", buf.String())
	}
}
