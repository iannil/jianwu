package book

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/iannil/jianwu/internal/storage"
)

// DefaultStorage is the filesystem backend used by all book IO functions.
// Replace in tests with storage.NewMemStorage() to avoid real filesystem access.
var DefaultStorage storage.Storage = storage.OS

// LoadMeta reads and parses meta.json.
func LoadMeta(path string) (*Meta, error) {
	data, err := DefaultStorage.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read meta %s: %w", path, err)
	}
	var m Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse meta %s: %w", path, err)
	}
	return &m, nil
}

// SaveMeta writes meta.json with 2-space indent.
func SaveMeta(path string, m *Meta) error {
	return writeJSON(path, m)
}

// LoadOutline reads and parses outline.json.
func LoadOutline(path string) (*Outline, error) {
	data, err := DefaultStorage.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read outline %s: %w", path, err)
	}
	var o Outline
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, fmt.Errorf("parse outline %s: %w", path, err)
	}
	return &o, nil
}

// SaveOutline writes outline.json with 2-space indent.
func SaveOutline(path string, o *Outline) error {
	return writeJSON(path, o)
}

func writeJSON(path string, v any) error {
	if err := DefaultStorage.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir for %s: %w", path, err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')
	if err := DefaultStorage.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
