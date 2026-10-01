package config

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

func TestSetSecretValuesWriteMergeClear(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GLM_API_KEY", "")

	// First write creates the file with 0600 and trims the value.
	if err := SetSecretValues(map[string]string{"gemini_api_key": " key-a "}); err != nil {
		t.Fatalf("SetSecretValues: %v", err)
	}
	path := filepath.Join(tmpHome, ".config", "jianwu", "secrets.yaml")
	if info, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("perm = %o, want 600", perm)
	}
	s, err := LoadSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if s.GeminiAPIKey != "key-a" {
		t.Fatalf("GeminiAPIKey = %q, want key-a", s.GeminiAPIKey)
	}

	// Second write merges (preserves the first key).
	if err := SetSecretValues(map[string]string{"glm_api_key": "key-b"}); err != nil {
		t.Fatalf("SetSecretValues merge: %v", err)
	}
	s, err = LoadSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if s.GeminiAPIKey != "key-a" || s.GLMAPIKey != "key-b" {
		t.Fatalf("after merge: gemini=%q glm=%q, want key-a/key-b", s.GeminiAPIKey, s.GLMAPIKey)
	}

	// Empty value removes the key; other keys survive.
	if err := SetSecretValues(map[string]string{"gemini_api_key": ""}); err != nil {
		t.Fatalf("SetSecretValues clear: %v", err)
	}
	s, err = LoadSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if s.GeminiAPIKey != "" || s.GLMAPIKey != "key-b" {
		t.Fatalf("after clear: gemini=%q glm=%q, want empty/key-b", s.GeminiAPIKey, s.GLMAPIKey)
	}

	// Unknown field is rejected without touching the file.
	if err := SetSecretValues(map[string]string{"not_a_field": "x"}); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestSetSecretValuesEnforcesPermAndPreservesComments(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("GEMINI_API_KEY", "")

	secretsDir := filepath.Join(tmpHome, ".config", "jianwu")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(secretsDir, "secrets.yaml")
	original := "# my keys\n# keep me\nbrave_api_key: brave-old\n"
	// Loose permissions: rewriting must tighten them to 0600.
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SetSecretValues(map[string]string{"brave_api_key": "brave-new"}); err != nil {
		t.Fatalf("SetSecretValues: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("perm = %o, want 600 after rewrite", perm)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := string(data)
	for _, want := range []string{"# my keys", "# keep me", "brave_api_key: brave-new"} {
		if !strings.Contains(updated, want) {
			t.Errorf("updated secrets missing %q:\n%s", want, updated)
		}
	}
}

// mockSecretsProvider returns fixed keys for testing.
type mockSecretsProvider struct {
	secrets  *Secrets
	tenantDB map[string]*Secrets
}

func (m *mockSecretsProvider) LoadSecrets() (*Secrets, error) {
	return m.secrets, nil
}

func (m *mockSecretsProvider) LoadSecretsFor(tenantID string) (*Secrets, error) {
	if s, ok := m.tenantDB[tenantID]; ok {
		return s, nil
	}
	return m.secrets, nil
}

func TestSetSecretsProviderInjection(t *testing.T) {
	mock := &mockSecretsProvider{
		secrets: &Secrets{GeminiAPIKey: "mock-global-key"},
	}
	SetSecretsProvider(mock)
	defer SetSecretsProvider(defaultSecretsProvider{})

	s, err := LoadSecrets()
	if err != nil {
		t.Fatalf("LoadSecrets: %v", err)
	}
	if s.GeminiAPIKey != "mock-global-key" {
		t.Errorf("got %q, want %q", s.GeminiAPIKey, "mock-global-key")
	}
}

func TestLoadSecretsForTenant(t *testing.T) {
	mock := &mockSecretsProvider{
		secrets: &Secrets{GeminiAPIKey: "global"},
		tenantDB: map[string]*Secrets{
			"tenant-a": {GeminiAPIKey: "tenant-a-key"},
		},
	}
	SetSecretsProvider(mock)
	defer SetSecretsProvider(defaultSecretsProvider{})

	// Tenant-specific
	sa, err := LoadSecretsFor("tenant-a")
	if err != nil {
		t.Fatalf("LoadSecretsFor: %v", err)
	}
	if sa.GeminiAPIKey != "tenant-a-key" {
		t.Errorf("tenant-a: got %q, want %q", sa.GeminiAPIKey, "tenant-a-key")
	}

	// Unknown tenant falls back to global
	sb, err := LoadSecretsFor("unknown")
	if err != nil {
		t.Fatalf("LoadSecretsFor: %v", err)
	}
	if sb.GeminiAPIKey != "global" {
		t.Errorf("unknown tenant: got %q, want %q", sb.GeminiAPIKey, "global")
	}
}

func TestLoadSecretsWarnsOnEnvFileDivergence(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	secretsDir := filepath.Join(tmpHome, ".config", "jianwu")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fileContent := "deepseek_api_key: file-key\nserper_api_key: same-key\n"
	if err := os.WriteFile(filepath.Join(secretsDir, "secrets.yaml"), []byte(fileContent), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DEEPSEEK_API_KEY", "env-key")    // diverges → warn
	t.Setenv("SERPER_API_KEY", "same-key")     // identical → silent
	t.Setenv("JINA_API_KEY", "env-only-value") // env only → silent

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	s, err := LoadSecrets()
	if err != nil {
		t.Fatalf("LoadSecrets: %v", err)
	}
	if s.DeepSeekAPIKey != "env-key" {
		t.Errorf("DeepSeekAPIKey = %q, want env-key (ENV wins)", s.DeepSeekAPIKey)
	}
	logs := buf.String()
	if !strings.Contains(logs, "DEEPSEEK_API_KEY") {
		t.Errorf("missing divergence warning for DEEPSEEK_API_KEY, log: %s", logs)
	}
	if strings.Contains(logs, "SERPER_API_KEY") || strings.Contains(logs, "JINA_API_KEY") {
		t.Errorf("unexpected warning for matching/env-only keys, log: %s", logs)
	}
}
