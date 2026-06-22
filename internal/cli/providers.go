package cli

import (
	"fmt"

	"github.com/zhurong/jianwu/internal/config"
	"github.com/zhurong/jianwu/internal/provider/llm"
	"github.com/zhurong/jianwu/internal/provider/llmfactory"
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
