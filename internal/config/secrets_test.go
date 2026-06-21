package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSecretsEnvOverridesFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Write file with file-gemini
	secretsDir := filepath.Join(tmpHome, ".config", "jianwu")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fileContent := "gemini_api_key: file-gemini\nglm_api_key: file-glm\n"
	if err := os.WriteFile(filepath.Join(secretsDir, "secrets.yaml"), []byte(fileContent), 0o600); err != nil {
		t.Fatal(err)
	}

	// ENV overrides file for Gemini
	t.Setenv("GEMINI_API_KEY", "env-gemini")

	s, err := LoadSecrets()
	if err != nil {
		t.Fatalf("LoadSecrets: %v", err)
	}
	if s.GeminiAPIKey != "env-gemini" {
		t.Errorf("GeminiAPIKey: got %q want %q", s.GeminiAPIKey, "env-gemini")
	}
	if s.GLMAPIKey != "file-glm" {
		t.Errorf("GLMAPIKey: got %q want %q (file fallback)", s.GLMAPIKey, "file-glm")
	}
}

func TestLoadSecretsReturnsEmptyIfNothingConfigured(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	// Clear any inherited env
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GLM_API_KEY", "")

	s, err := LoadSecrets()
	if err != nil {
		t.Fatalf("LoadSecrets: %v", err)
	}
	if s.GeminiAPIKey != "" {
		t.Errorf("expected empty Gemini key, got %q", s.GeminiAPIKey)
	}
}

func TestLoadSecretsWarnsOnLooseFilePermissions(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	secretsDir := filepath.Join(tmpHome, ".config", "jianwu")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// World-readable: 0644 — too loose
	if err := os.WriteFile(filepath.Join(secretsDir, "secrets.yaml"), []byte("gemini_api_key: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadSecrets()
	if err == nil {
		t.Error("expected warning/error for loose permissions, got nil")
	}
}
