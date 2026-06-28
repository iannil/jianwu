package expand

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/gemini"
	"github.com/iannil/jianwu/internal/provider/llm/glm"
)

func TestGenerateLiveGemini(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Skip("GEMINI_API_KEY not set")
	}
	p, err := gemini.New(gemini.Config{APIKey: key})
	if err != nil {
		t.Fatal(err)
	}
	runLiveGenerate(t, p)
}

func TestGenerateLiveGLM(t *testing.T) {
	key := os.Getenv("GLM_API_KEY")
	if key == "" {
		t.Skip("GLM_API_KEY not set")
	}
	p, err := glm.New(glm.Config{APIKey: key})
	if err != nil {
		t.Fatal(err)
	}
	runLiveGenerate(t, p)
}

func runLiveGenerate(t *testing.T, chatter llm.Chatter) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	out, err := Generate(ctx, chatter, nil, ExpandInput{
		ArchetypeID:  "ontology-epistemology-practice",
		Topic:        "人工智能时代的真实与虚幻",
		Audience:     "educated-general",
		Depth:        "intermediate",
		Goal:         "understanding",
		Length:       "short",
		Language:     "zh",
		PartIndex:    1,
		PartTitle:    "第一部 本体",
		PartRole:     "ontology",
		ChapterIndex: 1,
		ChapterTitle: "第一章 引言",
		Abstract:     "本章导引全书主题，建立核心问题意识。",
		KeyConcepts:  []string{"真实", "虚幻", "AI 时代"},
	}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Verify output
	if out.Markdown == "" {
		t.Error("expected non-empty Markdown")
	}
	if out.WordCount <= 0 {
		t.Errorf("expected WordCount > 0, got %d", out.WordCount)
	}

	t.Logf("word count: %d, citations: %d, unverified: %d", out.WordCount, len(out.Citations), len(out.UnverifiedClaims))
	t.Logf("markdown (first 500 chars): %s", truncate(out.Markdown, 500))
}
