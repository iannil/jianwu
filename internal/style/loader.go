package style

import (
	"fmt"
	"path"
	"strings"
)

// LoadGuide returns the full text of style-guide.md.
func LoadGuide() (string, error) {
	return string(guideFS), nil
}

// LoadSamples returns few-shot sample markdown keyed by archetype ID
// (the basename of each samples/<id>.md file).
func LoadSamples() (map[string]string, error) {
	entries, err := samplesFS.ReadDir("samples")
	if err != nil {
		return nil, fmt.Errorf("read samples dir: %w", err)
	}
	out := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := samplesFS.ReadFile(path.Join("samples", e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		id := strings.TrimSuffix(e.Name(), ".md")
		out[id] = string(data)
	}
	return out, nil
}
