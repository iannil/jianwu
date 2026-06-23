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

func TestProviderDepsHookIsConsultedWhenSet(t *testing.T) {
	original := providerDepsHook
	defer func() { providerDepsHook = original }()

	called := false
	providerDepsHook = func(cfg *config.Config, secrets *config.Secrets) (*ProviderDeps, error) {
		called = true
		return &ProviderDeps{Chatter: mock.New(llm.ChatResponse{Content: "x"})}, nil
	}

	deps, err := buildProviderDeps(&config.Config{}, &config.Secrets{})
	if err != nil {
		t.Fatalf("buildProviderDeps: %v", err)
	}
	if !called {
		t.Error("providerDepsHook was not consulted")
	}
	if deps == nil {
		t.Fatal("deps is nil")
	}
}

func TestProviderDepsHookFallsBackToRealAssemblyWhenNil(t *testing.T) {
	// Can't fully test real assembly without API keys, but can verify the hook
	// variable starts as the real builder (not nil).
	if providerDepsHook == nil {
		t.Fatal("providerDepsHook should default to real builder, not nil")
	}
}

func TestBuildToolRegistryAssemblesAllProviders(t *testing.T) {
	deps := &ProviderDeps{
		Chatter:  mock.New(llm.ChatResponse{Content: "x"}),
		Searcher: &stubSearcher{},
		Reader:   &stubReader{},
		Embedder: &stubEmbedder{},
	}
	registry, err := buildToolRegistry(deps)
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
}

// stubSearcher/Reader/Embedder defined at bottom of file or in a shared test helper.
type stubSearcher struct{}

func (s *stubSearcher) Search(ctx context.Context, query string, opts search.SearchOpts) ([]search.SearchResult, error) {
	return nil, nil
}

type stubReader struct{}

func (r *stubReader) Read(ctx context.Context, url string) (reader.Content, error) {
	return reader.Content{}, nil
}

type stubEmbedder struct{}

func (e *stubEmbedder) Embed(ctx context.Context, req llm.EmbedRequest) (*llm.EmbedResponse, error) {
	return &llm.EmbedResponse{}, nil
}
