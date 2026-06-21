package workspace

import (
	"os"
	"path/filepath"
)

// FindWorkspace walks up from startPath looking for a directory containing
// a .jianwu/ subdirectory. Returns the absolute path of the workspace root
// or ErrWorkspaceNotFound.
func FindWorkspace(startPath string) (string, error) {
	abs, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}
	dir := abs
	for {
		if isWorkspace(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// reached filesystem root
			return "", ErrWorkspaceNotFound
		}
		dir = parent
	}
}

func isWorkspace(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, MarkerName))
	return err == nil && info.IsDir()
}
