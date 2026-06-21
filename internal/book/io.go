package book

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadMeta reads and parses meta.json.
func LoadMeta(path string) (*Meta, error) {
	data, err := os.ReadFile(path)
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
	data, err := os.ReadFile(path)
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir for %s: %w", path, err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
