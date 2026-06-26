package archetypes

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Version is the current version of the embedded archetype definitions.
// Increment when YAML files in this package are updated.
const Version = "1"

// Load parses all embedded archetype YAML files keyed by archetype ID.
func Load() (map[string]*Archetype, error) {
	entries, err := fs.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("read embed dir: %w", err)
	}
	out := make(map[string]*Archetype)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := fs.ReadFile(e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		var a Archetype
		if err := yaml.Unmarshal(data, &a); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		if a.ID == "" {
			return nil, fmt.Errorf("archetype in %s has empty id", e.Name())
		}
		out[a.ID] = &a
	}
	return out, nil
}
