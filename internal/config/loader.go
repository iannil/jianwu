package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iannil/jianwu/internal/storage"
	"gopkg.in/yaml.v3"
)

// Load resolves the config layers (excluding env/CLI which the CLI layer
// applies later). Layer precedence (low to high):
//  1. BuiltinDefaults
//  2. global: ~/.config/jianwu/config.yaml (if exists)
//  3. workspace: <wsRoot>/.jianwu/config.yaml (if exists)
func Load(wsRoot string) (*Config, error) {
	cfg := BuiltinDefaults()

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve HOME: %w", err)
	}
	globalPath := filepath.Join(home, ".config", "jianwu", "config.yaml")
	if err := overlayYAML(cfg, globalPath); err != nil {
		return nil, fmt.Errorf("global config: %w", err)
	}

	wsPath := filepath.Join(wsRoot, ".jianwu", "config.yaml")
	if err := overlayYAML(cfg, wsPath); err != nil {
		return nil, fmt.Errorf("workspace config: %w", err)
	}

	return cfg, nil
}

// overlayYAML reads path (if it exists) and merges non-zero fields into cfg.
// Strategy: read file → unmarshal into a fresh Config → copy non-zero fields.
// Merging is field-by-field for struct fields (mergeConfig, mergeModelRef)
// and wholesale-replace for slices (Archetypes.Library, Style.Guide/Samples).
func overlayYAML(cfg *Config, path string) error {
	data, err := storage.OS.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var overlay Config
	if err := yaml.Unmarshal(data, &overlay); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	mergeConfig(cfg, &overlay)
	return nil
}

// mergeConfig copies non-zero fields from src into dst (in place).
// Each top-level Config field is checked individually; sub-structs like ModelRef
// are merged field-by-field via mergeModelRef (not wholesale replaced).
// Slices (Archetypes.Library, Style.Guide/Samples) are replaced wholesale.
func mergeConfig(dst, src *Config) {
	if src.LLM.TimeoutSeconds != 0 {
		dst.LLM.TimeoutSeconds = src.LLM.TimeoutSeconds
	}
	mergeModelRef(&dst.Models.Intake, &src.Models.Intake)
	mergeModelRef(&dst.Models.Outline, &src.Models.Outline)
	mergeModelRef(&dst.Models.Scaffolding, &src.Models.Scaffolding)
	mergeModelRef(&dst.Models.Expand, &src.Models.Expand)
	if src.Search.Primary != "" {
		dst.Search.Primary = src.Search.Primary
	}
	if src.Search.Fallback != "" {
		dst.Search.Fallback = src.Search.Fallback
	}
	if src.Search.Reader != "" {
		dst.Search.Reader = src.Search.Reader
	}
	if len(src.Archetypes.Library) > 0 {
		dst.Archetypes.Library = src.Archetypes.Library
	}
	if len(src.Style.Guide) > 0 {
		dst.Style.Guide = src.Style.Guide
	}
	if len(src.Style.Samples) > 0 {
		dst.Style.Samples = src.Style.Samples
	}
	if src.Scaffolding.Concurrency != 0 {
		dst.Scaffolding.Concurrency = src.Scaffolding.Concurrency
	}
	if src.Logging.Level != "" {
		dst.Logging.Level = src.Logging.Level
	}
}

func mergeModelRef(dst, src *ModelRef) {
	if src.Provider != "" {
		dst.Provider = src.Provider
	}
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.Fallback != nil {
		if dst.Fallback == nil {
			cp := *src.Fallback
			dst.Fallback = &cp
		} else {
			mergeModelRef(dst.Fallback, src.Fallback)
		}
	}
}
