package scaffolding

import (
	"context"
	"errors"
	"testing"

	"github.com/zhurong/jianwu/internal/book"
	"github.com/zhurong/jianwu/internal/provider/llm"
	"github.com/zhurong/jianwu/internal/provider/llm/mock"
)

func TestScaffoldAllUpdatesOutline(t *testing.T) {
	outline := &book.Outline{
		Parts: []book.OutlinePart{
			{Index: 1, Title: "P1", Role: "ontology", Chapters: []book.OutlineChapter{
				{Index: 1, Title: "C1"},
				{Index: 2, Title: "C2"},
			}},
		},
	}
	sample := `{"abstract":"X","key_concepts":["a"],"learning_objectives":["y"],"suggested_examples":["z"]}`
	p := mock.New(llm.ChatResponse{Content: sample})
	results := ScaffoldAll(context.Background(), p, outline, "ontology-epistemology-practice",
		ChapterParams{Topic: "T", Audience: "scholar", Depth: "advanced", Goal: "understanding", Length: "long", Language: "zh"},
		Options{Concurrency: 2})
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	for _, c := range outline.Parts[0].Chapters {
		if c.Abstract != "X" {
			t.Errorf("chapter %d abstract: %q", c.Index, c.Abstract)
		}
		if c.Status != book.StatusScaffolded {
			t.Errorf("chapter %d status: %q", c.Index, c.Status)
		}
	}
}

func TestScaffoldAllContinueOnError(t *testing.T) {
	outline := &book.Outline{
		Parts: []book.OutlinePart{
			{Index: 1, Title: "P1", Role: "ontology", Chapters: []book.OutlineChapter{
				{Index: 1, Title: "C1"},
				{Index: 2, Title: "C2"},
				{Index: 3, Title: "C3"},
			}},
		},
	}
	// Always-error chatter
	p := mock.NewError(errors.New("LLM down"))
	results := ScaffoldAll(context.Background(), p, outline, "ontology-epistemology-practice",
		ChapterParams{Topic: "T", Audience: "scholar", Depth: "advanced", Goal: "understanding", Length: "long", Language: "zh"},
		Options{Concurrency: 2})
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	for _, c := range outline.Parts[0].Chapters {
		if c.Status != book.StatusFailed {
			t.Errorf("chapter %d status: %q (want failed)", c.Index, c.Status)
		}
	}
}

func TestScaffoldAllEmptyOutlineNoOp(t *testing.T) {
	outline := &book.Outline{}
	p := mock.New(llm.ChatResponse{Content: "{}"})
	results := ScaffoldAll(context.Background(), p, outline, "x",
		ChapterParams{Topic: "T", Audience: "scholar", Depth: "advanced", Goal: "understanding", Length: "long", Language: "zh"},
		Options{})
	if len(results) != 0 {
		t.Errorf("got %d results", len(results))
	}
}
