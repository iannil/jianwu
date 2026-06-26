package revise

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
)

func TestRunRevise(t *testing.T) {
	chatter := mock.New(llm.ChatResponse{Content: "## Revised\n\nCorrected content with citation[^1].\n\n[^1]: [Source](https://example.com)"})

	out, err := Run(context.Background(), chatter, Input{
		ChapterTitle: "Test",
		Markdown:     "## Original\n\nWrong claim[^1].\n\n[^1]: [Source](https://example.com)",
		Citations: []book.Citation{
			{ID: "1", URL: "https://example.com"},
		},
		Verdicts: []book.ClaimVerdict{
			{ClaimText: "Wrong claim", Verified: false, SuggestedRewrite: "Corrected claim"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.RevisedMarkdown == "" {
		t.Fatal("empty revised markdown")
	}
	if !strings.Contains(out.RevisedMarkdown, "Revised") {
		t.Errorf("expected revised content, got: %s", out.RevisedMarkdown)
	}
}

func TestRunReviseError(t *testing.T) {
	chatter := mock.NewError(errors.New("llm down"))
	_, err := Run(context.Background(), chatter, Input{
		ChapterTitle: "Test",
		Markdown:     "content",
		Verdicts: []book.ClaimVerdict{
			{ClaimText: "Bad claim", Verified: false},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunReviseEmptyOutput(t *testing.T) {
	chatter := mock.New(llm.ChatResponse{Content: ""})
	_, err := Run(context.Background(), chatter, Input{
		ChapterTitle: "Test",
		Markdown:     "content",
	})
	if err == nil {
		t.Fatal("expected error for empty output")
	}
}

func TestFormatClaimList(t *testing.T) {
	items := []item{
		{ClaimText: "Claim A", Suggestion: "Rewrite A"},
		{ClaimText: "Claim B"},
	}
	result := formatClaimList(items)
	if !strings.Contains(result, "Claim A") {
		t.Error("missing Claim A")
	}
	if !strings.Contains(result, "Rewrite A") {
		t.Error("missing suggestion")
	}
	if !strings.Contains(result, "Claim B") {
		t.Error("missing Claim B")
	}
}
