package cli

import (
	"fmt"
	"log/slog"

	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/engine/expand"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llmfactory"
	"github.com/iannil/jianwu/internal/provider/reader"
	"github.com/iannil/jianwu/internal/provider/readerfactory"
	"github.com/iannil/jianwu/internal/provider/search"
	"github.com/iannil/jianwu/internal/provider/searchfactory"
)

// buildChatter constructs a Chatter for the given stage, wrapped in Retry + Fallback per Q7.
// stage is one of "intake", "outline", "scaffolding", "expand".
// If the ModelRef has a Fallback configured (non-nil), primary and fallback are each
// wrapped in RetryWrapper, then combined into a FallbackWrapper.
// If primary == fallback (identical provider+model), a warning is logged and no fallback is applied.
func buildChatter(cfg *config.Config, secrets *config.Secrets, stage string) (llm.Chatter, error) {
	primary, err := stageModel(cfg, stage)
	if err != nil {
		return nil, err
	}
	p, err := llmfactory.NewProvider(primary, secrets)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", stage, err)
	}
	var wrapped llm.ChatterEmbedder = llm.NewRetryWrapper(p)

	if primary.Fallback != nil {
		// Skip if fallback is identical to primary.
		if primary.Fallback.Provider == primary.Provider && primary.Fallback.Model == primary.Model {
			slog.Warn("fallback skipped: primary and fallback are identical",
				"stage", stage,
				"provider", primary.Provider,
				"model", primary.Model)
		} else {
			fb, err := llmfactory.NewProvider(*primary.Fallback, secrets)
			if err != nil {
				return nil, fmt.Errorf("%s fallback: %w", stage, err)
			}
			fbWrapped := llm.NewRetryWrapper(fb)
			wrapped = &llm.FallbackWrapper{Primary: wrapped, Fallback: fbWrapped}
			slog.Warn("fallback configured",
				"stage", stage,
				"primary_provider", primary.Provider,
				"primary_model", primary.Model,
				"fallback_provider", primary.Fallback.Provider,
				"fallback_model", primary.Fallback.Model)
		}
	}

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
// single struct rather than 4 separate hooks, prefiguring the eventual DI
// refactoring of providerDepsHook.
type ProviderDeps struct {
	Chatter  llm.Chatter
	Searcher search.Searcher
	Reader   reader.Reader
	Embedder llm.Embedder
}

// buildProviderDeps assembles providers from config + secrets using factories.
func buildProviderDeps(cfg *config.Config, secrets *config.Secrets) (*ProviderDeps, error) {
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

// buildToolRegistry assembles an expand.ToolRegistry from ProviderDeps.
// Sets provider names from config for citation metadata (avoids hardcoded strings).
func buildToolRegistry(deps *ProviderDeps, cfg *config.Config) (*expand.ToolRegistry, error) {
	if deps == nil {
		return nil, fmt.Errorf("deps is nil")
	}
	reg := expand.NewToolRegistry(deps.Searcher, deps.Reader, deps.Embedder)
	reg.SearchProviderName = cfg.Search.Primary
	reg.ReaderProviderName = cfg.Search.Reader
	return reg, nil
}
