package config

import (
	"fmt"
	"os"
	"path/filepath"

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

// LoadSecrets resolves API keys from ENV first, then ~/.config/jianwu/secrets.yaml.
// Returns an error if the file exists with permissions looser than 0600.
func LoadSecrets() (*Secrets, error) {
	s := &Secrets{}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve HOME: %w", err)
	}
	path := filepath.Join(home, ".config", "jianwu", "secrets.yaml")

	if info, err := os.Stat(path); err == nil {
		// File exists: enforce strict permissions.
		perm := info.Mode().Perm()
		if perm > 0o600 {
			return nil, fmt.Errorf(
				"secrets file %s has permissions %o; expected 0600 or stricter (run: chmod 600 %s)",
				path, perm, path,
			)
		}
		data, err := os.ReadFile(path)
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
