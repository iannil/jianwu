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

// titleFailingChatter wraps a chatter and fails for requests containing a specific chapter title.
type titleFailingChatter struct {
	inner     llm.Chatter
	failTitle string
}

func (c *titleFailingChatter) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	for _, m := range req.Messages {
		if m.Role == "user" && contains(m.Content, c.failTitle) {
			return nil, errors.New("scripted failure for " + c.failTitle)
		}
	}
	return c.inner.Chat(ctx, req)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestScaffoldAllPartialFailure(t *testing.T) {
	outline := &book.Outline{
		Parts: []book.OutlinePart{
			{Index: 1, Title: "P1", Role: "ontology", Chapters: []book.OutlineChapter{
				{Index: 1, Title: "C1"}, // succeeds
				{Index: 2, Title: "C2"}, // fails
				{Index: 3, Title: "C3"}, // succeeds
			}},
		},
	}
	successJSON := `{"abstract":"X","key_concepts":["a"],"learning_objectives":["y"],"suggested_examples":["z"]}`
	// Use titleFailingChatter to fail C2 deterministically
	successChatter := mock.New(llm.ChatResponse{Content: successJSON})
	p := &titleFailingChatter{
		inner:     successChatter,
		failTitle: "C2",
	}

	results := ScaffoldAll(context.Background(), p, outline, "ontology-epistemology-practice",
		ChapterParams{Topic: "T", Audience: "scholar", Depth: "advanced", Goal: "understanding", Length: "long", Language: "zh"},
		Options{Concurrency: 2})

	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	// Chapter 1: success
	if outline.Parts[0].Chapters[0].Status != book.StatusScaffolded {
		t.Errorf("C1 status: %q (want scaffolded)", outline.Parts[0].Chapters[0].Status)
	}
	if outline.Parts[0].Chapters[0].Abstract != "X" {
		t.Errorf("C1 abstract: %q", outline.Parts[0].Chapters[0].Abstract)
	}
	// Chapter 2: failed
	if outline.Parts[0].Chapters[1].Status != book.StatusFailed {
		t.Errorf("C2 status: %q (want failed)", outline.Parts[0].Chapters[1].Status)
	}
	// Chapter 3: success despite C2 failing
	if outline.Parts[0].Chapters[2].Status != book.StatusScaffolded {
		t.Errorf("C3 status: %q (want scaffolded — partial failure should not block siblings)", outline.Parts[0].Chapters[2].Status)
	}
}
