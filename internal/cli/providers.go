package cli

import (
	"fmt"

	"github.com/zhurong/jianwu/internal/book"
	"github.com/zhurong/jianwu/internal/config"
	"github.com/zhurong/jianwu/internal/engine/expand"
	"github.com/zhurong/jianwu/internal/provider/llm"
	"github.com/zhurong/jianwu/internal/provider/llmfactory"
	"github.com/zhurong/jianwu/internal/provider/reader"
	"github.com/zhurong/jianwu/internal/provider/readerfactory"
	"github.com/zhurong/jianwu/internal/provider/search"
	"github.com/zhurong/jianwu/internal/provider/searchfactory"
)

// buildChatter constructs a Chatter for the given stage, wrapped in Retry + Fallback per Q7.
// stage is one of "intake", "outline", "scaffolding", "expand".
// For S6, fallback is optional: if cfg.Models[stage] has no fallback configured, returns primary only.
func buildChatter(cfg *config.Config, secrets *config.Secrets, stage string) (llm.Chatter, error) {
	primary, err := stageModel(cfg, stage)
	if err != nil {
		return nil, err
	}
	p, err := llmfactory.NewProvider(primary, secrets)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", stage, err)
	}
	wrapped := llm.NewRetryWrapper(p)
	// Note: in S6 we don't yet wire fallback because Config doesn't carry fallback yet.
	// S6.1 or later can add config.Models[stage].Fallback and wrap with FallbackWrapper.
	return wrapped, nil
}

// buildEmbedder constructs an Embedder for the given stage.
func buildEmbedder(cfg *config.Config, secrets *config.Secrets, stage string) (llm.Embedder, error) {
	primary, err := stageModel(cfg, stage)
	if err != nil {
		return nil, err
	}
	return llmfactory.NewEmbedder(primary, secrets)
}

// stageModel returns the ModelRef for the given stage.
func stageModel(cfg *config.Config, stage string) (config.ModelRef, error) {
	switch stage {
	case "intake":
		return cfg.Models.Intake, nil
	case "outline":
		return cfg.Models.Outline, nil
	case "scaffolding":
		return cfg.Models.Scaffolding, nil
	case "expand":
		return cfg.Models.Expand, nil
	default:
		return config.ModelRef{}, fmt.Errorf("unknown stage: %q", stage)
	}
}

// ProviderDeps bundles the providers needed by expand CLI (and future commands
// that need search/reader/embedder in addition to chatter). Per Q20=B this is a
// single struct rather than 4 separate hooks, prefiguring the v1.1 refactor of
// chatterProviderHook into a CLI struct field.
type ProviderDeps struct {
	Chatter  llm.Chatter
	Searcher search.Searcher
	Reader   reader.Reader
	Embedder llm.Embedder
}

// providerDepsHook allows tests to inject mock provider bundles without going
// through the real factory. WARNING: package-global mutable var, same rules as
// chatterProviderHook — test binaries only, no concurrent mutation.
var providerDepsHook = func(cfg *config.Config, secrets *config.Secrets) (*ProviderDeps, error) {
	return buildProviderDepsReal(cfg, secrets)
}

// buildProviderDeps is the public entry that consults the hook.
func buildProviderDeps(cfg *config.Config, secrets *config.Secrets) (*ProviderDeps, error) {
	return providerDepsHook(cfg, secrets)
}

// buildProviderDepsReal assembles providers from config + secrets using factories.
func buildProviderDepsReal(cfg *config.Config, secrets *config.Secrets) (*ProviderDeps, error) {
	chatter, err := buildChatter(cfg, secrets, "expand")
	if err != nil {
		return nil, fmt.Errorf("expand chatter: %w", err)
	}
	searcher, err := searchfactory.New(cfg.Search.Primary, secrets)
	if err != nil {
		return nil, fmt.Errorf("search primary: %w", err)
	}
	reader, err := readerfactory.New(cfg.Search.Reader, secrets)
	if err != nil {
		return nil, fmt.Errorf("reader: %w", err)
	}
	embedder, err := buildEmbedder(cfg, secrets, "expand")
	if err != nil {
		return nil, fmt.Errorf("expand embedder: %w", err)
	}
	return &ProviderDeps{
		Chatter:  chatter,
		Searcher: searcher,
		Reader:   reader,
		Embedder: embedder,
	}, nil
}

// buildToolRegistry assembles an expand.ToolRegistry from ProviderDeps + outline.
// The outlineFn callback is a stub in v1.0.1 (returns empty string); v1.0.2
// will inject real adjacent-chapter prose per decision Q16=C.
func buildToolRegistry(deps *ProviderDeps, outline *book.Outline) (*expand.ToolRegistry, error) {
	if deps == nil {
		return nil, fmt.Errorf("deps is nil")
	}
	outlineFn := func(partIdx, chIdx int) (string, error) {
		// v1.0.1 stub: return empty; v1.0.2 will read from chapters/ dir
		// or use outline's abstract/key_concepts.
		return "", nil
	}
	return expand.NewToolRegistry(deps.Searcher, deps.Reader, deps.Embedder, outlineFn), nil
}
