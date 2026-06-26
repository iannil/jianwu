package cli

import (
	"context"
	"testing"

	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
	"github.com/iannil/jianwu/internal/provider/reader"
	"github.com/iannil/jianwu/internal/provider/search"
)

func TestBuildChatterIntake(t *testing.T) {
	cfg := &config.Config{
		Models: config.Models{
			Intake: config.ModelRef{Provider: "gemini", Model: "gemini-2.5-pro"},
		},
	}
	secrets := &config.Secrets{GeminiAPIKey: "fake-key"}
	c, err := buildChatter(cfg, secrets, "intake")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("nil chatter")
	}
}

func TestBuildChatterMissingKey(t *testing.T) {
	cfg := &config.Config{
		Models: config.Models{
			Outline: config.ModelRef{Provider: "gemini", Model: "gemini-2.5-pro"},
		},
	}
	_, err := buildChatter(cfg, &config.Secrets{}, "outline")
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestBuildChatterUnknownStage(t *testing.T) {
	_, err := buildChatter(&config.Config{}, &config.Secrets{}, "bogus")
	if err == nil {
		t.Error("expected error for unknown stage")
	}
}

func TestBuildEmbedder(t *testing.T) {
	cfg := &config.Config{
		Models: config.Models{
			Scaffolding: config.ModelRef{Provider: "glm", Model: "glm-4.6"},
		},
	}
	secrets := &config.Secrets{GLMAPIKey: "fake"}
	e, err := buildEmbedder(cfg, secrets, "scaffolding")
	if err != nil {
		t.Fatal(err)
	}
	if e == nil {
		t.Fatal("nil embedder")
	}
}

func TestBuildToolRegistryAssemblesAllProviders(t *testing.T) {
	deps := &ProviderDeps{
		Chatter:  mock.New(llm.ChatResponse{Content: "x"}),
		Searcher: &stubSearcher{},
		Reader:   &stubReader{},
		Embedder: &stubEmbedder{},
	}
	cfg := &config.Config{
		Search: config.Search{Primary: "test-search", Reader: "test-reader"},
	}
	registry, err := buildToolRegistry(deps, cfg)
	if err != nil {
		t.Fatalf("buildToolRegistry: %v", err)
	}
	if registry == nil {
		t.Fatal("registry is nil")
	}
	// ToolRegistry has exported Searcher/Reader/Embedder fields (see expand/tools.go).
	if registry.Searcher == nil {
		t.Error("Searcher not wired")
	}
	if registry.Reader == nil {
		t.Error("Reader not wired")
	}
	if registry.Embedder == nil {
		t.Error("Embedder not wired")
	}
	if registry.SearchProviderName != "test-search" {
		t.Errorf("SearchProviderName = %q, want %q", registry.SearchProviderName, "test-search")
	}
	if registry.ReaderProviderName != "test-reader" {
		t.Errorf("ReaderProviderName = %q, want %q", registry.ReaderProviderName, "test-reader")
	}
}

// stubSearcher/Reader/Embedder defined at bottom of file or in a shared test helper.
type stubSearcher struct{}

func (s *stubSearcher) Search(ctx context.Context, query string, opts search.SearchOpts) ([]search.SearchResult, error) {
	return nil, nil
}

func TestBuildChatterWithFallback(t *testing.T) {
	cfg := &config.Config{
		Models: config.Models{
			Outline: config.ModelRef{
				Provider: "gemini",
				Model:    "gemini-2.5-pro",
				Fallback: &config.ModelRef{Provider: "glm", Model: "glm-4.6"},
			},
		},
	}
	secrets := &config.Secrets{GeminiAPIKey: "gk", GLMAPIKey: "gk"}
	c, err := buildChatter(cfg, secrets, "outline")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("nil chatter")
	}
	_, ok := c.(*llm.FallbackWrapper)
	if !ok {
		t.Errorf("expected *llm.FallbackWrapper, got %T", c)
	}
}

func TestBuildChatterFallbackSelf(t *testing.T) {
	cfg := &config.Config{
		Models: config.Models{
			Outline: config.ModelRef{
				Provider: "gemini",
				Model:    "gemini-2.5-pro",
				Fallback: &config.ModelRef{Provider: "gemini", Model: "gemini-2.5-pro"},
			},
		},
	}
	secrets := &config.Secrets{GeminiAPIKey: "gk"}
	c, err := buildChatter(cfg, secrets, "outline")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("nil chatter")
	}
	_, ok := c.(*llm.RetryWrapper)
	if !ok {
		t.Errorf("expected *llm.RetryWrapper (no fallback), got %T", c)
	}
}

func TestBuildChatterNoFallback(t *testing.T) {
	// No fallback configured — should still return RetryWrapper (unchanged behaviour).
	cfg := &config.Config{
		Models: config.Models{
			Intake: config.ModelRef{Provider: "gemini", Model: "gemini-2.5-flash"},
		},
	}
	secrets := &config.Secrets{GeminiAPIKey: "gk"}
	c, err := buildChatter(cfg, secrets, "intake")
	if err != nil {
		t.Fatal(err)
	}
	_, ok := c.(*llm.RetryWrapper)
	if !ok {
		t.Errorf("expected *llm.RetryWrapper, got %T", c)
	}
}

type stubReader struct{}

func (r *stubReader) Read(ctx context.Context, url string) (reader.Content, error) {
	return reader.Content{}, nil
}

type stubEmbedder struct{}

func (e *stubEmbedder) Embed(ctx context.Context, req llm.EmbedRequest) (*llm.EmbedResponse, error) {
	return &llm.EmbedResponse{}, nil
}
