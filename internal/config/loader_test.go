package config

import (
	"os"
	"path/filepath"
	"testing"
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
schema_version: 1
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
