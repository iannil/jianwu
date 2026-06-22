package scaffolding

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/zhurong/jianwu/internal/book"
	"github.com/zhurong/jianwu/internal/provider/llm"
	"github.com/zhurong/jianwu/internal/provider/llm/gemini"
	"github.com/zhurong/jianwu/internal/provider/llm/glm"
)

func TestGenerateChapterLiveGemini(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Skip("GEMINI_API_KEY not set")
	}
	p, err := gemini.New(gemini.Config{APIKey: key})
	if err != nil {
		t.Fatal(err)
	}
	runLiveChapter(t, p)
}

func TestGenerateChapterLiveGLM(t *testing.T) {
	key := os.Getenv("GLM_API_KEY")
	if key == "" {
		t.Skip("GLM_API_KEY not set")
	}
	p, err := glm.New(glm.Config{APIKey: key})
	if err != nil {
		t.Fatal(err)
	}
	runLiveChapter(t, p)
}

func runLiveChapter(t *testing.T, chatter llm.Chatter) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	out, err := GenerateChapter(ctx, chatter, ChapterInput{
		ArchetypeID:  "ontology-epistemology-practice",
		PartIndex:    1,
		PartTitle:    "第一部 本体",
		PartRole:     "ontology",
		ChapterIndex: 1,
		ChapterTitle: "第一章 可能性基底",
		Topic:        "人工智能时代的真实与虚幻",
		Audience:     "educated-general",
		Depth:        "intermediate",
		Goal:         "understanding",
		Length:       "medium",
		Language:     "zh",
	})
	if err != nil {
		t.Fatalf("GenerateChapter: %v", err)
	}
	t.Logf("abstract: %s", out.Abstract)
	t.Logf("key_concepts: %v", out.KeyConcepts)
	if out.Status != book.StatusScaffolded {
		t.Errorf("status: %q", out.Status)
	}
}
