package llmfactory

import (
	"strings"
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

func TestNewChatterKimi(t *testing.T) {
	cases := []struct {
		name    string
		secrets *config.Secrets
		wantErr string
	}{
		{"with key", &config.Secrets{KimiAPIKey: "fake"}, ""},
		{"missing key", &config.Secrets{}, "KIMI_API_KEY"},
	}
	for _, tc := range cases {
		_, err := NewChatter(config.ModelRef{Provider: "kimi", Model: "kimi-k3"}, tc.secrets)
		if tc.wantErr == "" && err != nil {
			t.Errorf("%s: kimi: %v", tc.name, err)
		}
		if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
			t.Errorf("%s: want error containing %q, got %v", tc.name, tc.wantErr, err)
		}
	}
}

func TestNewChatterDeepSeek(t *testing.T) {
	cases := []struct {
		name    string
		secrets *config.Secrets
		wantErr string
	}{
		{"with key", &config.Secrets{DeepSeekAPIKey: "fake"}, ""},
		{"missing key", &config.Secrets{}, "DEEPSEEK_API_KEY"},
	}
	for _, tc := range cases {
		_, err := NewChatter(config.ModelRef{Provider: "deepseek", Model: "deepseek-chat"}, tc.secrets)
		if tc.wantErr == "" && err != nil {
			t.Errorf("%s: deepseek: %v", tc.name, err)
		}
		if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
			t.Errorf("%s: want error containing %q, got %v", tc.name, tc.wantErr, err)
		}
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
