package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/storage"
	"gopkg.in/yaml.v3"
)

// Provider name sets accepted by the config API. Keep in sync with the
// factory switches (llmfactory / searchfactory / readerfactory).
var (
	llmProviders    = []string{"gemini", "glm", "ollama"}
	searchProviders = []string{"brave", "serper"}
	readerProviders = []string{"jina"}
	loggingLevels   = []string{"debug", "info", "warn", "error"}
	sourceNames     = []string{"user", "builtin"} // archetypes/style library entries
)

// handleConfig returns the resolved (non-secret) workspace configuration.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	cfg, err := s.workspaceConfig()
	if err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, configView(cfg))
}

// handleConfigSave validates a full configuration object and writes it to the
// workspace config file (<root>/.jianwu/config.yaml), mirroring the CLI
// `jianwu config set` write-back. Empty provider/model means "use defaults";
// env vars and CLI flags still override the file layer on the next load.
func (s *Server) handleConfigSave(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	var in configUpdate
	if err := decodeBody(r, &in); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg, err := in.validate()
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		failf(w, http.StatusBadRequest, "marshal config: %v", err)
		return
	}
	path := filepath.Join(s.root(), ".jianwu", "config.yaml")
	if err := storage.OS.WriteFile(path, data, 0o644); err != nil {
		failErr(w, err)
		return
	}
	saved, err := s.workspaceConfig()
	if err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, configView(saved))
}

// modelRefUpdate is the JSON body shape of one stage model reference.
type modelRefUpdate struct {
	Provider string          `json:"provider"`
	Model    string          `json:"model"`
	Fallback *modelRefUpdate `json:"fallback"`
	Timeout  int             `json:"timeout"`
}

type configUpdate struct {
	LLM struct {
		Timeout int `json:"timeout"`
	} `json:"llm"`
	Models struct {
		Intake      modelRefUpdate `json:"intake"`
		Outline     modelRefUpdate `json:"outline"`
		Scaffolding modelRefUpdate `json:"scaffolding"`
		Expand      modelRefUpdate `json:"expand"`
	} `json:"models"`
	Search struct {
		Primary  string `json:"primary"`
		Fallback string `json:"fallback"`
		Reader   string `json:"reader"`
	} `json:"search"`
	Archetypes struct {
		Library []string `json:"library"`
	} `json:"archetypes"`
	Style struct {
		Guide   []string `json:"guide"`
		Samples []string `json:"samples"`
	} `json:"style"`
	Scaffolding struct {
		Concurrency int `json:"concurrency"`
	} `json:"scaffolding"`
	Logging struct {
		Level string `json:"level"`
	} `json:"logging"`
}

// validate checks the update and converts it to a Config ready for
// marshaling. Every field must pass validation; empty strings are legal and
// mean "not set" (lower layers apply).
func (u *configUpdate) validate() (*config.Config, error) {
	cfg := &config.Config{}
	if u.LLM.Timeout < 0 {
		return nil, errf("llm.timeout 不能为负数")
	}
	cfg.LLM.TimeoutSeconds = u.LLM.Timeout

	var err error
	if cfg.Models.Intake, err = u.Models.Intake.validate(); err != nil {
		return nil, errf("models.intake: %v", err)
	}
	if cfg.Models.Outline, err = u.Models.Outline.validate(); err != nil {
		return nil, errf("models.outline: %v", err)
	}
	if cfg.Models.Scaffolding, err = u.Models.Scaffolding.validate(); err != nil {
		return nil, errf("models.scaffolding: %v", err)
	}
	if cfg.Models.Expand, err = u.Models.Expand.validate(); err != nil {
		return nil, errf("models.expand: %v", err)
	}

	if u.Search.Primary != "" && !slices.Contains(searchProviders, u.Search.Primary) {
		return nil, errf("search.primary %q 无效（可选：%s）", u.Search.Primary, strings.Join(searchProviders, ", "))
	}
	if u.Search.Fallback != "" && !slices.Contains(searchProviders, u.Search.Fallback) {
		return nil, errf("search.fallback %q 无效（可选：%s）", u.Search.Fallback, strings.Join(searchProviders, ", "))
	}
	if u.Search.Reader != "" && !slices.Contains(readerProviders, u.Search.Reader) {
		return nil, errf("search.reader %q 无效（可选：%s）", u.Search.Reader, strings.Join(readerProviders, ", "))
	}
	cfg.Search = config.Search{
		Primary: u.Search.Primary, Fallback: u.Search.Fallback, Reader: u.Search.Reader,
	}

	if cfg.Archetypes.Library, err = validateSources(u.Archetypes.Library, "archetypes.library"); err != nil {
		return nil, err
	}
	if cfg.Style.Guide, err = validateSources(u.Style.Guide, "style.guide"); err != nil {
		return nil, err
	}
	if cfg.Style.Samples, err = validateSources(u.Style.Samples, "style.samples"); err != nil {
		return nil, err
	}

	if u.Scaffolding.Concurrency < 0 || u.Scaffolding.Concurrency > 64 {
		return nil, errf("scaffolding.concurrency 需在 1–64 之间（0 表示使用默认值）")
	}
	cfg.Scaffolding.Concurrency = u.Scaffolding.Concurrency

	if u.Logging.Level != "" && !slices.Contains(loggingLevels, u.Logging.Level) {
		return nil, errf("logging.level %q 无效（可选：%s）", u.Logging.Level, strings.Join(loggingLevels, ", "))
	}
	cfg.Logging.Level = u.Logging.Level
	return cfg, nil
}

// validate checks one stage model ref. Provider and model must be set together
// or both empty; a fallback identical to the primary is dropped (the assembly
// layer would ignore it anyway).
func (m *modelRefUpdate) validate() (config.ModelRef, error) {
	ref := config.ModelRef{TimeoutSeconds: m.Timeout}
	if m.Timeout < 0 {
		return ref, errf("timeout 不能为负数")
	}
	if (m.Provider == "") != (m.Model == "") {
		return ref, errf("provider 与 model 需同时填写或同时留空")
	}
	if m.Provider != "" {
		if !slices.Contains(llmProviders, m.Provider) {
			return ref, errf("provider %q 无效（可选：%s）", m.Provider, strings.Join(llmProviders, ", "))
		}
	}
	ref.Provider, ref.Model = m.Provider, m.Model
	if m.Fallback != nil && (m.Fallback.Provider != "" || m.Fallback.Model != "") {
		fb, err := m.Fallback.validate()
		if err != nil {
			return ref, errf("fallback: %v", err)
		}
		if fb.Provider != ref.Provider || fb.Model != ref.Model {
			ref.Fallback = &fb
		}
	}
	return ref, nil
}

// validateSources trims, de-duplicates and validates an archetypes/style
// source list; an empty list is legal and means "use defaults".
func validateSources(in []string, key string) ([]string, error) {
	out := make([]string, 0, len(in))
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if !slices.Contains(sourceNames, v) {
			return nil, errf("%s 条目 %q 无效（可选：%s）", key, v, strings.Join(sourceNames, ", "))
		}
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out, nil
}

// errf is a tiny alias keeping the validation blocks readable.
func errf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

// --- secrets (API keys) ---

// handleSecretsGet reports which API keys are configured, masked. Values are
// never returned in full; the source (env overrides file) is included so the
// UI can explain why a file edit would not take effect.
func (s *Server) handleSecretsGet(w http.ResponseWriter, r *http.Request) {
	secrets, err := config.LoadSecrets()
	if err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, secretsView(secrets))
}

// handleSecretsSave applies {"<field>": "<value>"} updates to the global
// secrets file. A non-empty value sets the key; "" clears it from the file
// (an ENV override may still win — the response reflects the new state).
func (s *Server) handleSecretsSave(w http.ResponseWriter, r *http.Request) {
	var updates map[string]string
	if err := decodeBody(r, &updates); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	clean := make(map[string]string, len(updates))
	for k, v := range updates {
		clean[k] = strings.TrimSpace(v)
	}
	if err := config.SetSecretValues(clean); err != nil {
		failErr(w, err)
		return
	}
	secrets, err := config.LoadSecrets()
	if err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, secretsView(secrets))
}

// secretFieldView is the masked projection of one secret field.
type secretFieldView struct {
	Field  string `json:"field"`
	EnvVar string `json:"env_var"`
	Set    bool   `json:"set"`
	Source string `json:"source"` // env | file | unset
	Masked string `json:"masked"`
}

// secretsView builds the masked view for all secret fields.
func secretsView(secrets *config.Secrets) map[string]any {
	values := map[string]string{
		"gemini_api_key": secrets.GeminiAPIKey,
		"glm_api_key":    secrets.GLMAPIKey,
		"brave_api_key":  secrets.BraveAPIKey,
		"serper_api_key": secrets.SerperAPIKey,
		"jina_api_key":   secrets.JinaAPIKey,
	}
	fields := make([]secretFieldView, 0, len(config.SecretFieldNames))
	for _, name := range config.SecretFieldNames {
		v := values[name]
		envVar := config.SecretEnvVars[name]
		f := secretFieldView{Field: name, EnvVar: envVar}
		switch {
		case os.Getenv(envVar) != "":
			f.Set, f.Source = true, "env"
		case v != "":
			f.Set, f.Source = true, "file"
		default:
			f.Source = "unset"
		}
		f.Masked = maskSecret(v)
		fields = append(fields, f)
	}
	return map[string]any{
		"path":   secretsFileDisplayPath(),
		"fields": fields,
	}
}

// maskSecret shows at most the last 4 characters of a key.
func maskSecret(v string) string {
	if v == "" {
		return ""
	}
	if len(v) <= 8 {
		return "••••"
	}
	return "••••" + v[len(v)-4:]
}

// secretsFileDisplayPath resolves the secrets file path for display; errors
// fall back to the literal default location.
func secretsFileDisplayPath() string {
	p, err := config.SecretsFilePath()
	if err != nil {
		return "~/.config/jianwu/secrets.yaml"
	}
	return p
}
