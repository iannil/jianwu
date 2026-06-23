package llmfactory

import (
	"testing"

	"github.com/iannil/jianwu/internal/config"
)

func TestNewChatterGemini(t *testing.T) {
	secrets := &config.Secrets{GeminiAPIKey: "fake"}
	_, err := NewChatter(config.ModelRef{Provider: "gemini", Model: "gemini-2.5-pro"}, secrets)
	if err != nil {
		t.Fatalf("gemini: %v", err)
	}
}

func TestNewChatterGLM(t *testing.T) {
	secrets := &config.Secrets{GLMAPIKey: "fake"}
	_, err := NewChatter(config.ModelRef{Provider: "glm", Model: "glm-4.6"}, secrets)
	if err != nil {
		t.Fatalf("glm: %v", err)
	}
}

func TestNewChatterUnknownProviderErrors(t *testing.T) {
	_, err := NewChatter(config.ModelRef{Provider: "unknown", Model: "x"}, &config.Secrets{})
	if err == nil {
		t.Error("expected error")
	}
}

func TestNewChatterMissingKeyErrors(t *testing.T) {
	_, err := NewChatter(config.ModelRef{Provider: "gemini", Model: "x"}, &config.Secrets{})
	if err == nil {
		t.Error("expected error for missing Gemini key")
	}
}

func TestNewEmbedderGemini(t *testing.T) {
	secrets := &config.Secrets{GeminiAPIKey: "fake"}
	_, err := NewEmbedder(config.ModelRef{Provider: "gemini", Model: "gemini-2.5-pro"}, secrets)
	if err != nil {
		t.Fatalf("gemini embedder: %v", err)
	}
}

func TestNewEmbedderGLM(t *testing.T) {
	secrets := &config.Secrets{GLMAPIKey: "fake"}
	_, err := NewEmbedder(config.ModelRef{Provider: "glm", Model: "glm-4.6"}, secrets)
	if err != nil {
		t.Fatalf("glm embedder: %v", err)
	}
}

func TestNewEmbedderUnknownProviderErrors(t *testing.T) {
	_, err := NewEmbedder(config.ModelRef{Provider: "unknown", Model: "x"}, &config.Secrets{})
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestNewEmbedderMissingKeyErrors(t *testing.T) {
	_, err := NewEmbedder(config.ModelRef{Provider: "glm", Model: "x"}, &config.Secrets{})
	if err == nil {
		t.Error("expected error for missing GLM key")
	}
}
