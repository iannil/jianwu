package corpus

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

// Load parses all embedded builtin corpus JSON files keyed by book slug.
func Load() (map[string]*Book, error) {
	entries, err := builtinFS.ReadDir("builtin")
	if err != nil {
		return nil, fmt.Errorf("read builtin dir: %w", err)
	}
	out := make(map[string]*Book)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := fs.ReadFile(builtinFS, "builtin/"+e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		var b Book
		if err := json.Unmarshal(data, &b); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		if b.Slug == "" {
			return nil, fmt.Errorf("book in %s has empty slug", e.Name())
		}
		out[b.Slug] = &b
	}
	return out, nil
}
