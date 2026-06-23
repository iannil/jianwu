package scaffolding

import (
	"context"
	"errors"
	"testing"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
)

func TestRetryFailedOnlyTouchesFailedChapters(t *testing.T) {
	outline := &book.Outline{
		Parts: []book.OutlinePart{
			{Index: 1, Title: "P1", Role: "ontology", Chapters: []book.OutlineChapter{
				{Index: 1, Title: "C1", Status: book.StatusScaffolded, Abstract: "already done"},
				{Index: 2, Title: "C2", Status: book.StatusFailed},
			}},
		},
	}
	sample := `{"abstract":"recovered","key_concepts":["a"],"learning_objectives":["y"],"suggested_examples":["z"]}`
	p := mock.New(llm.ChatResponse{Content: sample})

	results := RetryFailed(context.Background(), p, outline, "ontology-epistemology-practice",
		ChapterParams{Topic: "T", Audience: "scholar", Depth: "advanced", Goal: "understanding", Length: "long", Language: "zh"},
		Options{})

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (only failed)", len(results))
	}
	// Original successful chapter untouched.
	if outline.Parts[0].Chapters[0].Abstract != "already done" {
		t.Errorf("existing chapter was modified: %q", outline.Parts[0].Chapters[0].Abstract)
	}
	// Failed chapter recovered.
	if outline.Parts[0].Chapters[1].Status != book.StatusScaffolded {
		t.Errorf("failed chapter not recovered: %q", outline.Parts[0].Chapters[1].Status)
	}
	if outline.Parts[0].Chapters[1].Abstract != "recovered" {
		t.Errorf("recovered abstract: %q", outline.Parts[0].Chapters[1].Abstract)
	}
}

func TestRetryFailedNoFailedChaptersIsNoOp(t *testing.T) {
	outline := &book.Outline{
		Parts: []book.OutlinePart{
			{Index: 1, Title: "P1", Role: "ontology", Chapters: []book.OutlineChapter{
				{Index: 1, Title: "C1", Status: book.StatusScaffolded},
			}},
		},
	}
	p := mock.New(llm.ChatResponse{Content: "{}"})
	results := RetryFailed(context.Background(), p, outline, "x",
		ChapterParams{Topic: "T", Audience: "scholar", Depth: "advanced", Goal: "understanding", Length: "long", Language: "zh"},
		Options{})
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRetryFailedStillFailsReturnsErrorInResult(t *testing.T) {
	outline := &book.Outline{
		Parts: []book.OutlinePart{
			{Index: 1, Title: "P1", Role: "ontology", Chapters: []book.OutlineChapter{
				{Index: 1, Title: "C1", Status: book.StatusFailed},
			}},
		},
	}
	p := mock.NewError(errors.New("still down"))
	results := RetryFailed(context.Background(), p, outline, "ontology-epistemology-practice",
		ChapterParams{Topic: "T", Audience: "scholar", Depth: "advanced", Goal: "understanding", Length: "long", Language: "zh"},
		Options{})
	if len(results) != 1 {
		t.Fatalf("got %d results", len(results))
	}
	if outline.Parts[0].Chapters[0].Status != book.StatusFailed {
		t.Errorf("status should remain failed: %q", outline.Parts[0].Chapters[0].Status)
	}
}
