package llmfactory

import (
	"fmt"

	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/gemini"
	"github.com/iannil/jianwu/internal/provider/llm/glm"
	"github.com/iannil/jianwu/internal/provider/llm/ollama"
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
	case "ollama":
		return ollama.New(ollama.Config{})
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
	case "ollama":
		return ollama.New(ollama.Config{})
	default:
		return nil, fmt.Errorf("unknown LLM provider: %q", ref.Provider)
	}
}

// NewProvider constructs a provider that implements both Chatter and Embedder.
// This is useful when you need a single provider instance for both interfaces,
// such as when wrapping with RetryWrapper which requires ChatterEmbedder.
func NewProvider(ref config.ModelRef, secrets *config.Secrets) (llm.ChatterEmbedder, error) {
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
	case "ollama":
		return ollama.New(ollama.Config{})
	default:
		return nil, fmt.Errorf("unknown LLM provider: %q", ref.Provider)
	}
}
