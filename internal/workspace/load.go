package workspace

import (
	"fmt"
	"path/filepath"

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

	cfg, err := config.Load(wsRoot)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	return &Workspace{Root: wsRoot, Config: cfg}, nil
}
