package outline

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/gemini"
	"github.com/iannil/jianwu/internal/provider/llm/glm"
)

// TestGenerateLiveGemini exercises the full Generate flow against Gemini.
// Skips if GEMINI_API_KEY is unset.
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

// TestGenerateLiveGLM exercises the full Generate flow against GLM.
// Skips if GLM_API_KEY is unset.
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
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	out, err := Generate(ctx, chatter, Input{
		ArchetypeID: "ontology-epistemology-practice",
		Topic:       "人工智能时代的真实与虚幻",
		Audience:    "educated-general",
		Depth:       "intermediate",
		Goal:        "understanding",
		Length:      "short",
		Language:    "zh",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(out.Parts) == 0 {
		t.Fatal("no parts generated")
	}
	for _, p := range out.Parts {
		t.Logf("Part %d (%s): %s — %d chapters", p.Index, p.Role, p.Title, len(p.Chapters))
		for _, c := range p.Chapters {
			t.Logf("  Ch %d: %s", c.Index, c.Title)
		}
	}
}
