package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iannil/jianwu/internal/config"
	"gopkg.in/yaml.v3"
)

// --- helpers ---

// findSecretField locates one field view in a /secrets response.
func findSecretField(t *testing.T, view map[string]any, field string) map[string]any {
	t.Helper()
	fields, ok := view["fields"].([]any)
	if !ok {
		t.Fatalf("secrets view has no fields: %v", view)
	}
	for _, f := range fields {
		m, ok := f.(map[string]any)
		if ok && m["field"] == field {
			return m
		}
	}
	t.Fatalf("field %q not in secrets view: %v", field, view)
	return nil
}

// --- config save ---

func TestConfigSaveRoundTrip(t *testing.T) {
	srv, _ := newTestEnv(t)
	body := map[string]any{
		"llm": map[string]any{"timeout": 120},
		"models": map[string]any{
			"expand": map[string]any{
				"provider": "gemini", "model": "gemini-2.5-flash", "timeout": 300,
				"fallback": map[string]any{"provider": "glm", "model": "glm-4.6"},
			},
			"intake": map[string]any{"provider": "glm", "model": "glm-4.6"},
		},
		"search":      map[string]any{"primary": "serper", "fallback": "brave", "reader": "jina"},
		"archetypes":  map[string]any{"library": []string{"builtin", "user"}},
		"style":       map[string]any{"guide": []string{"user"}, "samples": []string{"builtin"}},
		"scaffolding": map[string]any{"concurrency": 3},
		"logging":     map[string]any{"level": "info"},
	}
	rec, got := do(t, srv, "POST", "/api/v1/config", body)
	wantCode(t, rec, http.StatusOK)

	expand := got["models"].(map[string]any)["expand"].(map[string]any)
	if expand["provider"] != "gemini" || expand["model"] != "gemini-2.5-flash" {
		t.Fatalf("expand = %v, want gemini/gemini-2.5-flash", expand)
	}
	if expand["timeout"] != float64(300) {
		t.Fatalf("expand timeout = %v, want 300", expand["timeout"])
	}
	fb, ok := expand["fallback"].(map[string]any)
	if !ok || fb["provider"] != "glm" || fb["model"] != "glm-4.6" {
		t.Fatalf("expand fallback = %v, want glm/glm-4.6", expand["fallback"])
	}
	if got["search"].(map[string]any)["primary"] != "serper" {
		t.Fatalf("search = %v", got["search"])
	}
	if got["scaffolding"].(map[string]any)["concurrency"] != float64(3) {
		t.Fatalf("concurrency = %v", got["scaffolding"])
	}
	lib := got["archetypes"].(map[string]any)["library"].([]any)
	if len(lib) != 2 || lib[0] != "builtin" {
		t.Fatalf("archetypes.library = %v", lib)
	}

	// GET reflects the saved values (workspace reloaded from disk).
	rec, cfg := do(t, srv, "GET", "/api/v1/config", nil)
	wantCode(t, rec, http.StatusOK)
	if cfg["models"].(map[string]any)["expand"].(map[string]any)["provider"] != "gemini" {
		t.Fatalf("GET after save: %v", cfg["models"])
	}

	// The workspace config.yaml on disk is valid YAML with the values.
	data, err := os.ReadFile(filepath.Join(srv.WSRoot(), ".jianwu", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed config.Config
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse saved config.yaml: %v", err)
	}
	if parsed.Models.Expand.Provider != "gemini" || parsed.Models.Expand.TimeoutSeconds != 300 {
		t.Fatalf("parsed expand = %+v", parsed.Models.Expand)
	}
	if parsed.Models.Expand.Fallback == nil || parsed.Models.Expand.Fallback.Provider != "glm" {
		t.Fatalf("parsed expand fallback = %+v", parsed.Models.Expand.Fallback)
	}
	if parsed.Scaffolding.Concurrency != 3 || parsed.Logging.Level != "info" {
		t.Fatalf("parsed scaffolding/logging = %+v / %+v", parsed.Scaffolding, parsed.Logging)
	}
}

func TestConfigSaveDropsSelfFallback(t *testing.T) {
	srv, _ := newTestEnv(t)
	body := map[string]any{
		"models": map[string]any{
			"expand": map[string]any{
				"provider": "glm", "model": "glm-4.6",
				"fallback": map[string]any{"provider": "glm", "model": "glm-4.6"},
			},
		},
	}
	rec, got := do(t, srv, "POST", "/api/v1/config", body)
	wantCode(t, rec, http.StatusOK)
	expand := got["models"].(map[string]any)["expand"].(map[string]any)
	if expand["fallback"] != nil {
		t.Fatalf("self fallback should be dropped, got %v", expand["fallback"])
	}
}

func TestConfigSaveValidation(t *testing.T) {
	srv, _ := newTestEnv(t)
	cases := []struct {
		name string
		body map[string]any
	}{
		{"bad llm provider", map[string]any{"models": map[string]any{
			"expand": map[string]any{"provider": "nope", "model": "x"}}}},
		{"provider without model", map[string]any{"models": map[string]any{
			"expand": map[string]any{"provider": "glm"}}}},
		{"negative stage timeout", map[string]any{"models": map[string]any{
			"expand": map[string]any{"provider": "glm", "model": "m", "timeout": -1}}}},
		{"bad fallback provider", map[string]any{"models": map[string]any{
			"expand": map[string]any{"provider": "glm", "model": "m",
				"fallback": map[string]any{"provider": "nope", "model": "x"}}}}},
		{"bad search primary", map[string]any{"search": map[string]any{"primary": "google"}}},
		{"bad reader", map[string]any{"search": map[string]any{"reader": "browser"}}},
		{"bad logging level", map[string]any{"logging": map[string]any{"level": "loud"}}},
		{"negative concurrency", map[string]any{"scaffolding": map[string]any{"concurrency": -2}}},
		{"huge concurrency", map[string]any{"scaffolding": map[string]any{"concurrency": 100}}},
		{"bad source entry", map[string]any{"archetypes": map[string]any{"library": []string{"web"}}}},
		{"negative llm timeout", map[string]any{"llm": map[string]any{"timeout": -5}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, resp := do(t, srv, "POST", "/api/v1/config", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
			}
			if resp["error"] == nil || resp["error"] == "" {
				t.Fatalf("expected error message, got %v", resp)
			}
		})
	}
}

func TestConfigSaveRequiresWorkspace(t *testing.T) {
	srv, _ := newTestEnvAt(t, t.TempDir()) // uninitialized root
	rec, _ := do(t, srv, "POST", "/api/v1/config", map[string]any{})
	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("status = %d, want 428", rec.Code)
	}
}

// --- secrets ---

func TestSecretsViewMasksValues(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("GEMINI_API_KEY", "env-secret-key-9876")
	srv, _ := newTestEnv(t)

	rec, view := do(t, srv, "GET", "/api/v1/secrets", nil)
	wantCode(t, rec, http.StatusOK)

	// ENV-sourced field: masked, never the full key.
	gemini := findSecretField(t, view, "gemini_api_key")
	if gemini["source"] != "env" || gemini["set"] != true {
		t.Fatalf("gemini = %v, want source=env set=true", gemini)
	}
	if masked, _ := gemini["masked"].(string); masked == "" || !strings.HasPrefix(masked, "••••") || strings.Contains(masked, "env-secret") {
		t.Fatalf("gemini masked = %q, must be masked", masked)
	}

	// Unset fields.
	glm := findSecretField(t, view, "glm_api_key")
	if glm["source"] != "unset" || glm["set"] != false {
		t.Fatalf("glm = %v, want source=unset", glm)
	}
	if view["path"] == nil || view["path"] == "" {
		t.Fatalf("path missing in view: %v", view)
	}
}

func TestSecretsSaveAndClearRoundTrip(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GLM_API_KEY", "")
	srv, _ := newTestEnv(t)

	// Save two keys; the response reflects file-sourced values.
	rec, view := do(t, srv, "POST", "/api/v1/secrets", map[string]any{
		"gemini_api_key": "sk-gemini-abcdef123456", "glm_api_key": "  glm-key-6543210  ",
	})
	wantCode(t, rec, http.StatusOK)
	gemini := findSecretField(t, view, "gemini_api_key")
	if gemini["source"] != "file" {
		t.Fatalf("gemini = %v, want source=file", gemini)
	}
	if masked, _ := gemini["masked"].(string); !strings.HasSuffix(masked, "3456") || strings.Contains(masked, "abcdef") {
		t.Fatalf("gemini masked = %q, want ••••+last4 only", masked)
	}
	// Value is trimmed.
	glm := findSecretField(t, view, "glm_api_key")
	if glm["source"] != "file" {
		t.Fatalf("glm = %v, want source=file", glm)
	}

	// Written file has 0600 and parses via LoadSecrets.
	path := filepath.Join(tmpHome, ".config", "jianwu", "secrets.yaml")
	if info, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("secrets perm = %o, want 600", perm)
	}
	secrets, err := config.LoadSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if secrets.GeminiAPIKey != "sk-gemini-abcdef123456" || secrets.GLMAPIKey != "glm-key-6543210" {
		t.Fatalf("loaded secrets: %+#v", secrets)
	}

	// ENV wins over file for source reporting.
	t.Setenv("GEMINI_API_KEY", "env-wins")
	rec, view = do(t, srv, "GET", "/api/v1/secrets", nil)
	wantCode(t, rec, http.StatusOK)
	if f := findSecretField(t, view, "gemini_api_key"); f["source"] != "env" {
		t.Fatalf("gemini = %v, want source=env", f)
	}

	// Clear GLM from the file; Gemini survives.
	rec, view = do(t, srv, "POST", "/api/v1/secrets", map[string]any{"glm_api_key": ""})
	wantCode(t, rec, http.StatusOK)
	if f := findSecretField(t, view, "glm_api_key"); f["source"] != "unset" {
		t.Fatalf("glm = %v, want unset after clear", f)
	}
	if f := findSecretField(t, view, "brave_api_key"); f["source"] != "unset" {
		t.Fatalf("brave should remain untouched: %v", f)
	}

	// Unknown field → 400.
	rec, _ = do(t, srv, "POST", "/api/v1/secrets", map[string]any{"not_a_field": "x"})
	wantCode(t, rec, http.StatusBadRequest)
}

// JSON round-trip guard for the masked view shape used by the UI.
func TestSecretsViewJSONShape(t *testing.T) {
	srv, _ := newTestEnv(t)
	rec, _ := do(t, srv, "GET", "/api/v1/secrets", nil)
	wantCode(t, rec, http.StatusOK)
	var raw struct {
		Path   string `json:"path"`
		Fields []struct {
			Field  string `json:"field"`
			EnvVar string `json:"env_var"`
			Set    bool   `json:"set"`
			Source string `json:"source"`
			Masked string `json:"masked"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode secrets view: %v", err)
	}
	if len(raw.Fields) != 5 {
		t.Fatalf("fields = %d, want 5", len(raw.Fields))
	}
	if raw.Fields[0].Field != "gemini_api_key" || raw.Fields[0].EnvVar != "GEMINI_API_KEY" {
		t.Fatalf("first field = %+v", raw.Fields[0])
	}
}
