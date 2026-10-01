package server

import (
	"context"
	"fmt"
	"time"

	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/engine/expand"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llmfactory"
	"github.com/iannil/jianwu/internal/provider/readerfactory"
	"github.com/iannil/jianwu/internal/provider/searchfactory"
)

// depsNeed selects which providers a handler actually uses, so e.g. the
// grill interview never requires search API keys.
type depsNeed struct {
	chatter, searcher, reader, embedder bool
}

var (
	needChatterOnly   = depsNeed{chatter: true}
	needChatterReader = depsNeed{chatter: true, reader: true}
	needExpand        = depsNeed{chatter: true, searcher: true, reader: true, embedder: true}
	needCollect       = depsNeed{chatter: true, searcher: true, reader: true}
)

// resolveDeps returns providers for engine calls: injected deps when present,
// otherwise built fresh from workspace config + secrets (picks up config edits).
// stage selects which stage model to use for the Chatter ("intake", "outline",
// "scaffolding", "expand"); need filters which providers are built.
func (s *Server) resolveDeps(stage string, need depsNeed) (*Deps, error) {
	if s.inject != nil && s.inject.Chatter != nil {
		return s.inject, nil
	}
	ws, err := s.loadWorkspace()
	if err != nil {
		return nil, err
	}
	secrets, err := config.LoadSecrets()
	if err != nil {
		return nil, fmt.Errorf("load secrets: %w", err)
	}
	deps := &Deps{}
	if s.inject != nil {
		deps.Searcher, deps.Reader, deps.Embedder = s.inject.Searcher, s.inject.Reader, s.inject.Embedder
	}
	if need.chatter && deps.Chatter == nil {
		chatter, err := s.buildChatter(ws.Config, secrets, stage)
		if err != nil {
			return nil, err
		}
		deps.Chatter = chatter
	}
	if need.searcher && deps.Searcher == nil {
		searcher, err := searchfactory.New(ws.Config.Search.Primary, secrets)
		if err != nil {
			return nil, fmt.Errorf("search primary: %w", err)
		}
		deps.Searcher = searcher
	}
	if need.reader && deps.Reader == nil {
		rd, err := readerfactory.New(ws.Config.Search.Reader, secrets)
		if err != nil {
			return nil, fmt.Errorf("reader: %w", err)
		}
		deps.Reader = rd
	}
	if need.embedder && deps.Embedder == nil {
		// A dedicated models.embedder config wins over the stage model, so
		// chat and embedding can use different providers (e.g. deepseek + glm).
		ref := ws.Config.Models.Scaffolding
		if ws.Config.Models.Embedder != nil {
			ref = *ws.Config.Models.Embedder
		}
		embedder, err := llmfactory.NewEmbedder(ref, secrets)
		if err != nil {
			return nil, fmt.Errorf("embedder: %w", err)
		}
		deps.Embedder = embedder
	}
	return deps, nil
}

// buildChatter constructs a Chatter for the given stage, wrapped in
// Retry + Fallback (mirrors the CLI assembly, per Q7).
func (s *Server) buildChatter(cfg *config.Config, secrets *config.Secrets, stage string) (llm.Chatter, error) {
	primary, err := stageModel(cfg, stage)
	if err != nil {
		return nil, err
	}
	p, err := llmfactory.NewProvider(primary, secrets)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", stage, err)
	}
	var wrapped llm.ChatterEmbedder = llm.NewRetryWrapper(p)
	if primary.Fallback != nil && !(primary.Fallback.Provider == primary.Provider && primary.Fallback.Model == primary.Model) {
		fb, err := llmfactory.NewProvider(*primary.Fallback, secrets)
		if err != nil {
			return nil, fmt.Errorf("%s fallback: %w", stage, err)
		}
		wrapped = &llm.FallbackWrapper{Primary: wrapped, Fallback: llm.NewRetryWrapper(fb)}
	}
	return wrapped, nil
}

// stageModel returns the ModelRef for the given pipeline stage.
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

// jobCtx returns a context for a job: Ctrl+C is meaningless server-side, but
// per-stage timeouts from config still apply. stage "" means no stage timeout.
func stageTimeoutCtx(ctx context.Context, cfg *config.Config, stage string) (context.Context, context.CancelFunc) {
	if cfg == nil {
		return ctx, func() {}
	}
	timeout := cfg.LLM.TimeoutSeconds
	if stage != "" {
		if m, err := stageModel(cfg, stage); err == nil && m.TimeoutSeconds > 0 {
			timeout = m.TimeoutSeconds
		}
	}
	if timeout > 0 {
		return context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	}
	return ctx, func() {}
}

// buildRegistry assembles an expand.ToolRegistry from deps + config.
func (s *Server) buildRegistry(deps *Deps, cfg *config.Config) *expand.ToolRegistry {
	reg := expand.NewToolRegistry(deps.Searcher, deps.Reader, deps.Embedder)
	reg.SearchProviderName = cfg.Search.Primary
	reg.ReaderProviderName = cfg.Search.Reader
	reg.SetCorpusIndexPath(expand.CorpusIndexPathForWorkspace(s.root()))
	return reg
}
