package llmfactory

import (
	"fmt"

	"github.com/zhurong/jianwu/internal/config"
	"github.com/zhurong/jianwu/internal/provider/llm"
	"github.com/zhurong/jianwu/internal/provider/llm/gemini"
	"github.com/zhurong/jianwu/internal/provider/llm/glm"
)

// NewChatter constructs a Chatter for the given provider/model.
// For S2: returns the bare provider (no retry/fallback wrapping yet).
// Engine layer in S3+ will wrap with RetryWrapper and FallbackWrapper per config.
func NewChatter(ref config.ModelRef, secrets *config.Secrets) (llm.Chatter, error) {
	switch ref.Provider {
	case "gemini":
		if secrets.GeminiAPIKey == "" {
			return nil, fmt.Errorf("gemini provider requires GEMINI_API_KEY")
		}
		return gemini.New(gemini.Config{APIKey: secrets.GeminiAPIKey})
	case "glm":
		if secrets.GLMAPIKey == "" {
			return nil, fmt.Errorf("glm provider requires GLM_API_KEY")
		}
		return glm.New(glm.Config{APIKey: secrets.GLMAPIKey})
	default:
		return nil, fmt.Errorf("unknown LLM provider: %q", ref.Provider)
	}
}

// NewEmbedder constructs an Embedder. Same switch as Chatter since both providers
// implement both interfaces.
func NewEmbedder(ref config.ModelRef, secrets *config.Secrets) (llm.Embedder, error) {
	switch ref.Provider {
	case "gemini":
		if secrets.GeminiAPIKey == "" {
			return nil, fmt.Errorf("gemini provider requires GEMINI_API_KEY")
		}
		return gemini.New(gemini.Config{APIKey: secrets.GeminiAPIKey})
	case "glm":
		if secrets.GLMAPIKey == "" {
			return nil, fmt.Errorf("glm provider requires GLM_API_KEY")
		}
		return glm.New(glm.Config{APIKey: secrets.GLMAPIKey})
	default:
		return nil, fmt.Errorf("unknown LLM provider: %q", ref.Provider)
	}
}
