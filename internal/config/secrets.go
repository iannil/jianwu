package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iannil/jianwu/internal/storage"
	"gopkg.in/yaml.v3"
)

// Env var names for API keys.
const (
	GeminiAPIKeyEnv = "GEMINI_API_KEY"
	GLMAPIKeyEnv    = "GLM_API_KEY"
	BraveAPIKeyEnv  = "BRAVE_API_KEY"
	SerperAPIKeyEnv = "SERPER_API_KEY"
	JinaAPIKeyEnv   = "JINA_API_KEY"
)

// Secrets holds resolved API keys. ENV > file precedence is applied per field.
type Secrets struct {
	GeminiAPIKey string `yaml:"gemini_api_key"`
	GLMAPIKey    string `yaml:"glm_api_key"`
	BraveAPIKey  string `yaml:"brave_api_key"`
	SerperAPIKey string `yaml:"serper_api_key"`
	JinaAPIKey   string `yaml:"jina_api_key"`
}

// SecretsProvider resolves API keys. The default implementation reads from
// ENV and ~/.config/jianwu/secrets.yaml. Inject a custom provider for
// per-tenant keys or test mocks.
type SecretsProvider interface {
	// LoadSecrets returns the global secrets.
	LoadSecrets() (*Secrets, error)
	// LoadSecretsFor returns secrets scoped to the given tenant.
	// The default implementation ignores tenantID; inject a provider
	// that uses it for per-tenant key isolation.
	LoadSecretsFor(tenantID string) (*Secrets, error)
}

// defaultSecretsProvider is the built-in SecretsProvider.
type defaultSecretsProvider struct{}

func (defaultSecretsProvider) LoadSecrets() (*Secrets, error) {
	return loadSecrets()
}

func (defaultSecretsProvider) LoadSecretsFor(_ string) (*Secrets, error) {
	return loadSecrets()
}

// secretsProvider is the package-level provider. Override with SetSecretsProvider.
var secretsProvider SecretsProvider = defaultSecretsProvider{}

// SetSecretsProvider replaces the global secrets provider.
// Used by tests and by SaaS tenants to provide per-tenant keys.
func SetSecretsProvider(p SecretsProvider) {
	secretsProvider = p
}

// LoadSecrets resolves API keys using the configured SecretsProvider.
func LoadSecrets() (*Secrets, error) {
	return secretsProvider.LoadSecrets()
}

// LoadSecretsFor resolves API keys for a specific tenant.
// Falls back to global keys when no per-tenant provider is configured.
func LoadSecretsFor(tenantID string) (*Secrets, error) {
	return secretsProvider.LoadSecretsFor(tenantID)
}

// loadSecrets implements the default resolution: ENV first, then file.
func loadSecrets() (*Secrets, error) {
	s := &Secrets{}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve HOME: %w", err)
	}
	path := filepath.Join(home, ".config", "jianwu", "secrets.yaml")

	if info, err := storage.OS.Stat(path); err == nil {
		// File exists: enforce strict permissions.
		perm := info.Mode().Perm()
		if perm > 0o600 {
			return nil, fmt.Errorf(
				"secrets file %s has permissions %o; expected 0600 or stricter (run: chmod 600 %s)",
				path, perm, path,
			)
		}
		data, err := storage.OS.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read secrets: %w", err)
		}
		if err := yaml.Unmarshal(data, s); err != nil {
			return nil, fmt.Errorf("parse secrets: %w", err)
		}
	}

	// ENV overrides file per field.
	if v := os.Getenv(GeminiAPIKeyEnv); v != "" {
		s.GeminiAPIKey = v
	}
	if v := os.Getenv(GLMAPIKeyEnv); v != "" {
		s.GLMAPIKey = v
	}
	if v := os.Getenv(BraveAPIKeyEnv); v != "" {
		s.BraveAPIKey = v
	}
	if v := os.Getenv(SerperAPIKeyEnv); v != "" {
		s.SerperAPIKey = v
	}
	if v := os.Getenv(JinaAPIKeyEnv); v != "" {
		s.JinaAPIKey = v
	}

	return s, nil
}
