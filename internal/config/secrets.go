package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/iannil/jianwu/internal/storage"
	"gopkg.in/yaml.v3"
)

// Env var names for API keys.
const (
	GeminiAPIKeyEnv   = "GEMINI_API_KEY"
	GLMAPIKeyEnv      = "GLM_API_KEY"
	KimiAPIKeyEnv     = "KIMI_API_KEY"
	DeepSeekAPIKeyEnv = "DEEPSEEK_API_KEY"
	BraveAPIKeyEnv    = "BRAVE_API_KEY"
	SerperAPIKeyEnv   = "SERPER_API_KEY"
	JinaAPIKeyEnv     = "JINA_API_KEY"
)

// Secrets holds resolved API keys. ENV > file precedence is applied per field.
type Secrets struct {
	GeminiAPIKey   string `yaml:"gemini_api_key"`
	GLMAPIKey      string `yaml:"glm_api_key"`
	KimiAPIKey     string `yaml:"kimi_api_key"`
	DeepSeekAPIKey string `yaml:"deepseek_api_key"`
	BraveAPIKey    string `yaml:"brave_api_key"`
	SerperAPIKey   string `yaml:"serper_api_key"`
	JinaAPIKey     string `yaml:"jina_api_key"`
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

// SecretFieldNames lists the editable secrets.yaml field names, in display
// order. SetSecretValues accepts exactly these keys.
var SecretFieldNames = []string{
	"gemini_api_key", "glm_api_key", "kimi_api_key", "deepseek_api_key",
	"brave_api_key", "serper_api_key", "jina_api_key",
}

// SecretEnvVars maps secrets.yaml field names to their ENV overrides.
var SecretEnvVars = map[string]string{
	"gemini_api_key":   GeminiAPIKeyEnv,
	"glm_api_key":      GLMAPIKeyEnv,
	"kimi_api_key":     KimiAPIKeyEnv,
	"deepseek_api_key": DeepSeekAPIKeyEnv,
	"brave_api_key":    BraveAPIKeyEnv,
	"serper_api_key":   SerperAPIKeyEnv,
	"jina_api_key":     JinaAPIKeyEnv,
}

// SecretsFilePath returns the global secrets file location.
func SecretsFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve HOME: %w", err)
	}
	return filepath.Join(home, ".config", "jianwu", "secrets.yaml"), nil
}

// SetSecretValues surgically updates the global secrets file
// (~/.config/jianwu/secrets.yaml). Keys must be SecretFieldNames; a non-empty
// value sets the key, an empty value removes it from the file. Other keys and
// comments are preserved (same yaml.Node edit strategy as SetGlobalWorkspace).
// The file is (re)written with 0600 permissions.
func SetSecretValues(updates map[string]string) error {
	for k := range updates {
		if !slices.Contains(SecretFieldNames, k) {
			return fmt.Errorf("unknown secret field %q (want one of %s)", k, strings.Join(SecretFieldNames, ", "))
		}
	}
	path, err := SecretsFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	var doc yaml.Node
	data, err := os.ReadFile(path)
	if err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read secrets: %w", err)
	}

	mapping, err := rootMappingNode(&doc)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	// Iterate SecretFieldNames (not the map) for deterministic output order.
	// Values are trimmed; keys never legitimately start/end with whitespace.
	for _, k := range SecretFieldNames {
		v, ok := updates[k]
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if v == "" {
			deleteMappingKey(mapping, k)
		} else {
			setMappingString(mapping, k, v)
		}
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal secrets: %w", err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("write secrets: %w", err)
	}
	// WriteFile keeps existing looser permissions; enforce 0600 explicitly.
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("chmod secrets: %w", err)
	}
	return nil
}

// deleteMappingKey removes key from a mapping node when present.
func deleteMappingKey(m *yaml.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}

// loadSecrets implements the default resolution: ENV first, then file.
func loadSecrets() (*Secrets, error) {
	s := &Secrets{}

	path, err := SecretsFilePath()
	if err != nil {
		return nil, err
	}

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

	// ENV overrides file per field. A stale exported ENV silently beating a
	// freshly updated secrets.yaml is a support trap, so warn on divergence
	// (precedence is unchanged).
	type envOverride struct {
		env, field string
		fileValue  *string
	}
	for _, o := range []envOverride{
		{GeminiAPIKeyEnv, "gemini_api_key", &s.GeminiAPIKey},
		{GLMAPIKeyEnv, "glm_api_key", &s.GLMAPIKey},
		{KimiAPIKeyEnv, "kimi_api_key", &s.KimiAPIKey},
		{DeepSeekAPIKeyEnv, "deepseek_api_key", &s.DeepSeekAPIKey},
		{BraveAPIKeyEnv, "brave_api_key", &s.BraveAPIKey},
		{SerperAPIKeyEnv, "serper_api_key", &s.SerperAPIKey},
		{JinaAPIKeyEnv, "jina_api_key", &s.JinaAPIKey},
	} {
		v := os.Getenv(o.env)
		if v == "" {
			continue
		}
		if *o.fileValue != "" && *o.fileValue != v {
			slog.Warn("secrets: ENV value differs from secrets.yaml; ENV wins (unset the env var to use the file value)",
				"env", o.env, "field", o.field)
		}
		*o.fileValue = v
	}

	return s, nil
}
