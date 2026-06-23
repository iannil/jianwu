package scaffolding

import (
	"context"
	"testing"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
)

func TestGenerateChapterValidatesInput(t *testing.T) {
	_, err := GenerateChapter(context.Background(), mock.New(llm.ChatResponse{}), ChapterInput{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestGenerateChapterParsesResponse(t *testing.T) {
	sample := `{
		"abstract": "本章界定时间的本体地位。",
		"key_concepts": ["可能性基底", "收敛", "观察"],
		"learning_objectives": ["理解时间不是流动的", "区分时间与变化"],
		"suggested_examples": ["Zeno 悖论", "量子测量实验"]
	}`
	p := mock.New(llm.ChatResponse{Content: sample})
	out, err := GenerateChapter(context.Background(), p, ChapterInput{
		ArchetypeID:  "ontology-epistemology-practice",
		PartIndex:    1,
		PartTitle:    "第一部 本体",
		PartRole:     "ontology",
		ChapterIndex: 1,
		ChapterTitle: "第一章 可能性基底",
		Topic:        "时间的实在",
		Audience:     "educated-general",
		Depth:        "advanced",
		Goal:         "understanding",
		Length:       "long",
		Language:     "zh",
	})
	if err != nil {
		t.Fatalf("GenerateChapter: %v", err)
	}
	if out.Abstract != "本章界定时间的本体地位。" {
		t.Errorf("abstract: got %q, want %q", out.Abstract, "本章界定时间的本体地位。")
	}
	if len(out.KeyConcepts) != 3 {
		t.Errorf("concepts: got %d, want 3", len(out.KeyConcepts))
	}
	if out.Status != book.StatusScaffolded {
		t.Errorf("status: got %q, want %q", out.Status, book.StatusScaffolded)
	}
}

func TestGenerateChapterRejectsMalformedJSON(t *testing.T) {
	p := mock.New(llm.ChatResponse{Content: "not json"})
	_, err := GenerateChapter(context.Background(), p, ChapterInput{
		ArchetypeID:  "ontology-epistemology-practice",
		PartRole:     "ontology",
		ChapterTitle: "X",
		Topic:        "X",
		Language:     "zh",
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
}
