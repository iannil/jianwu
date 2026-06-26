package workspace

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/storage"
)

// Workspace is a loaded workspace root + its resolved config.
type Workspace struct {
	Root   string
	Config *config.Config
}

// Load validates the workspace at wsRoot and returns it with config resolved.
func Load(wsRoot string) (*Workspace, error) {
	marker := filepath.Join(wsRoot, MarkerName)
	if info, err := storage.OS.Stat(marker); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrWorkspaceNotFound, wsRoot)
	}

	schemaBytes, err := storage.OS.ReadFile(filepath.Join(marker, SchemaVersionFileName))
	if err != nil {
		return nil, fmt.Errorf("read schema_version: %w", err)
	}
	schema := strings.TrimSpace(string(schemaBytes))
	if schema != CurrentSchemaVersion {
		return nil, fmt.Errorf(
			"workspace schema_version %q does not match supported version %q: run `jianwu migrate` (planned for v0.2)",
			schema, CurrentSchemaVersion,
		)
	}

	cfg, err := config.Load(wsRoot)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	return &Workspace{Root: wsRoot, Config: cfg}, nil
}
