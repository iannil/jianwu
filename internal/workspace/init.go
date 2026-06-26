package workspace

import (
	"fmt"
	"path/filepath"

	"github.com/iannil/jianwu/internal/storage"
)

// Init creates a workspace at the given path.
// Default (non-bare) layout: .jianwu/{config.yaml, schema_version} + books/ + exports/ + archive/.
// Bare layout: only .jianwu/ with config.yaml + schema_version.
// Returns an error if a workspace already exists at the path.
func Init(path string, opts InitOpts) error {
	marker := filepath.Join(path, MarkerName)
	if _, err := storage.OS.Stat(marker); err == nil {
		return fmt.Errorf("workspace already exists at %s", path)
	}

	if err := storage.OS.MkdirAll(marker, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", marker, err)
	}

	if err := storage.OS.WriteFile(
		filepath.Join(marker, SchemaVersionFileName),
		[]byte(CurrentSchemaVersion+"\n"),
		0o644,
	); err != nil {
		return fmt.Errorf("write schema_version: %w", err)
	}

	cfg := defaultWorkspaceConfig()
	if err := storage.OS.WriteFile(
		filepath.Join(marker, ConfigFileName),
		[]byte(cfg),
		0o644,
	); err != nil {
		return fmt.Errorf("write config.yaml: %w", err)
	}

	if opts.Bare {
		return nil
	}

	for _, sub := range []string{"books", "exports", "archive"} {
		if err := storage.OS.MkdirAll(filepath.Join(path, sub), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", sub, err)
		}
	}
	return nil
}

// defaultWorkspaceConfig returns the template written into config.yaml on init.
// Kept in workspace package (not config package) to avoid an import cycle:
// config package loads workspaces, workspace writes the initial template.
func defaultWorkspaceConfig() string {
	return `# jianwu workspace configuration
# Global config: ~/.config/jianwu/config.yaml (overrides here)
schema_version: 1

# Global LLM settings (per-stage overrides in models.<stage>.timeout)
# llm:
#   timeout: 90  # default per-chat timeout in seconds; 0 = no timeout

models:
  intake:       { provider: glm,    model: glm-4.6 }
  outline:
    provider: gemini
    model: gemini-2.5-pro
    # fallback: { provider: glm, model: glm-4.6 }
  scaffolding:  { provider: gemini, model: gemini-2.5-flash }
  expand:       { provider: glm,    model: glm-4.6 }
  # Per-stage fallback: uncomment fallback: under any stage to auto-switch
  # when the primary provider fails (retries exhausted). To skip fallback for a
  # stage, omit the field entirely. See docs/PROJECT_STATUS.md §v0.1.4.

search:
  primary: brave
  fallback: serper
  reader: jina

archetypes:
  library: [user, builtin]

style:
  guide: [user, builtin]
  samples: [builtin]

scaffolding:
  concurrency: 5

logging:
  level: warn
`
}
