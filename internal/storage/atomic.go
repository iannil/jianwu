package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// WriteFileAtomic replaces one file using a temporary sibling and Rename.
// It preserves the previous file on write/rename failure. This is not a
// transaction across files or a guarantee of durability after power loss.
func WriteFileAtomic(s Storage, path string, data []byte, perm os.FileMode) error {
	if err := s.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent: %w", err)
	}
	temp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+"."+uuid.NewString()+".tmp")
	defer s.RemoveAll(temp)
	if err := s.WriteFile(temp, data, perm); err != nil {
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := s.Rename(temp, path); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}
	return nil
}
