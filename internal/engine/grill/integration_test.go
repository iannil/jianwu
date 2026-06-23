package grill

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/gemini"
	"github.com/iannil/jianwu/internal/provider/llm/glm"
)

func TestRecommendLiveGemini(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Skip("GEMINI_API_KEY not set")
	}
	p, err := gemini.New(gemini.Config{APIKey: key})
	if err != nil {
		t.Fatal(err)
	}
	runLiveRecommend(t, p)
}

func TestRecommendLiveGLM(t *testing.T) {
	key := os.Getenv("GLM_API_KEY")
	if key == "" {
		t.Skip("GLM_API_KEY not set")
	}
	p, err := glm.New(glm.Config{APIKey: key})
	if err != nil {
		t.Fatal(err)
	}
	runLiveRecommend(t, p)
}

func runLiveRecommend(t *testing.T, chatter llm.Chatter) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tree := DefaultTree()
	dim := tree.Find("audience")
	rec, err := Recommend(ctx, chatter, *dim, map[string]string{
		"topic": "人工智能时代的真实与虚幻",
	})
	if err != nil {
		t.Fatalf("Recommend: %v", err)
	}
	t.Logf("recommendation for audience: %s", rec)
}
