package llmfactory

import (
	"fmt"

	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/deepseek"
	"github.com/iannil/jianwu/internal/provider/llm/gemini"
	"github.com/iannil/jianwu/internal/provider/llm/glm"
	"github.com/iannil/jianwu/internal/provider/llm/kimi"
	"github.com/iannil/jianwu/internal/provider/llm/ollama"
)

// NewChatter constructs a Chatter with the configured default model.
func NewChatter(ref config.ModelRef, secrets *config.Secrets) (llm.Chatter, error) {
	return NewProvider(ref, secrets)
}

// NewEmbedder constructs an Embedder with the configured default model.
func NewEmbedder(ref config.ModelRef, secrets *config.Secrets) (llm.Embedder, error) {
	return NewProvider(ref, secrets)
}

// NewProvider constructs a provider that implements both Chatter and Embedder.
// This is useful when you need a single provider instance for both interfaces,
// such as when wrapping with RetryWrapper which requires ChatterEmbedder.
func NewProvider(ref config.ModelRef, secrets *config.Secrets) (llm.ChatterEmbedder, error) {
	provider, err := newProvider(ref, secrets)
	if err != nil {
		return nil, err
	}
	return &modelProvider{inner: provider, model: ref.Model}, nil
}

func newProvider(ref config.ModelRef, secrets *config.Secrets) (llm.ChatterEmbedder, error) {
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
	case "kimi":
		if secrets.KimiAPIKey == "" {
			return nil, fmt.Errorf("kimi provider requires KIMI_API_KEY")
		}
		return kimi.New(kimi.Config{APIKey: secrets.KimiAPIKey})
	case "deepseek":
		if secrets.DeepSeekAPIKey == "" {
			return nil, fmt.Errorf("deepseek provider requires DEEPSEEK_API_KEY")
		}
		return deepseek.New(deepseek.Config{APIKey: secrets.DeepSeekAPIKey})
	case "ollama":
		return ollama.New(ollama.Config{})
	default:
		return nil, fmt.Errorf("unknown LLM provider: %q", ref.Provider)
	}
}
