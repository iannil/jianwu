package cli

import (
	"testing"

	"github.com/zhurong/jianwu/internal/config"
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
