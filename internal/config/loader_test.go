package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadReturnsDefaultsWhenNoGlobalOrWorkspace(t *testing.T) {
	// Use temp HOME so no global config exists
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	wsRoot := t.TempDir()
	// No .jianwu/config.yaml in workspace
	if err := os.MkdirAll(filepath.Join(wsRoot, ".jianwu"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(wsRoot)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Models.Outline.Provider != "gemini" {
		t.Errorf("Outline.Provider: got %q want %q", cfg.Models.Outline.Provider, "gemini")
	}
	if cfg.Scaffolding.Concurrency != 5 {
		t.Errorf("Concurrency: got %d want 5", cfg.Scaffolding.Concurrency)
	}
}

func TestLoadWorkspaceOverridesDefaults(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	wsRoot := t.TempDir()
	wsConfig := `
models:
  outline: { provider: glm, model: glm-4.6 }
scaffolding:
  concurrency: 10
`
	if err := os.MkdirAll(filepath.Join(wsRoot, ".jianwu"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsRoot, ".jianwu", "config.yaml"), []byte(wsConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(wsRoot)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Models.Outline.Provider != "glm" {
		t.Errorf("Outline.Provider: got %q want %q (workspace override)", cfg.Models.Outline.Provider, "glm")
	}
	if cfg.Models.Intake.Provider != "glm" {
		t.Errorf("Intake.Provider: got %q want %q (default retained)", cfg.Models.Intake.Provider, "glm")
	}
	if cfg.Scaffolding.Concurrency != 10 {
		t.Errorf("Concurrency: got %d want 10 (override)", cfg.Scaffolding.Concurrency)
	}
}

func TestLoadGlobalOverridesDefaults(t *testing.T) {
	tmpHome := t.TempDir()
	globalConfig := `
models:
  expand: { provider: gemini, model: gemini-2.5-pro }
`
	cfgDir := filepath.Join(tmpHome, ".config", "jianwu")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(globalConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmpHome)

	wsRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wsRoot, ".jianwu"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(wsRoot)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Models.Expand.Provider != "gemini" {
		t.Errorf("Expand.Provider: got %q want %q (global)", cfg.Models.Expand.Provider, "gemini")
	}
}

func TestLoadWorkspaceOverridesGlobal(t *testing.T) {
	tmpHome := t.TempDir()
	globalConfig := `
models:
  outline: { provider: glm, model: glm-4.6 }
`
	cfgDir := filepath.Join(tmpHome, ".config", "jianwu")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(globalConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmpHome)

	wsRoot := t.TempDir()
	wsConfig := `
models:
  outline: { provider: gemini, model: gemini-2.5-pro }
`
	if err := os.MkdirAll(filepath.Join(wsRoot, ".jianwu"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsRoot, ".jianwu", "config.yaml"), []byte(wsConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(wsRoot)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Models.Outline.Provider != "gemini" {
		t.Errorf("workspace should override global; got %q want %q", cfg.Models.Outline.Provider, "gemini")
	}
}

func TestModelRefFallbackParsing(t *testing.T) {
	y := `
models:
  intake: { provider: glm, model: glm-4.6 }
  outline:
    provider: gemini
    model: gemini-2.5-pro
    fallback: { provider: glm, model: glm-4.6 }
  scaffolding: { provider: gemini, model: gemini-2.5-flash }
  expand: { provider: glm, model: glm-4.6 }
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(y), &cfg); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Models.Intake.Fallback != nil {
		t.Errorf("Intake.Fallback should be nil, got %+v", *cfg.Models.Intake.Fallback)
	}
	if cfg.Models.Outline.Fallback == nil {
		t.Fatal("Outline.Fallback should be non-nil")
	}
	if cfg.Models.Outline.Fallback.Provider != "glm" {
		t.Errorf("Outline.Fallback.Provider = %q, want %q", cfg.Models.Outline.Fallback.Provider, "glm")
	}
	if cfg.Models.Outline.Fallback.Model != "glm-4.6" {
		t.Errorf("Outline.Fallback.Model = %q, want %q", cfg.Models.Outline.Fallback.Model, "glm-4.6")
	}
	if cfg.Models.Scaffolding.Fallback != nil {
		t.Errorf("Scaffolding.Fallback should be nil")
	}
}

func TestLLMTimeoutDefaults(t *testing.T) {
	var cfg Config
	yamlBody := `llm:
  timeout: 0`
	if err := yaml.Unmarshal([]byte(yamlBody), &cfg); err != nil {
		t.Fatalf("parse: %v", err)
	}
	// TimeoutSeconds is 0 when not set (defaults applied by BuiltinDefaults)
	_ = cfg
}

func TestLLMTimeoutParsing(t *testing.T) {
	y := `
llm:
  timeout: 120
models:
  intake: { provider: glm, model: glm-4.6, timeout: 300 }
  outline: { provider: gemini, model: gemini-2.5-pro }
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(y), &cfg); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.LLM.TimeoutSeconds != 120 {
		t.Errorf("global LLM timeout = %d, want 120", cfg.LLM.TimeoutSeconds)
	}
	if cfg.Models.Intake.TimeoutSeconds != 300 {
		t.Errorf("Intake timeout = %d, want 300", cfg.Models.Intake.TimeoutSeconds)
	}
	if cfg.Models.Outline.TimeoutSeconds != 0 {
		t.Errorf("Outline timeout should be 0 (not set), got %d", cfg.Models.Outline.TimeoutSeconds)
	}
}
