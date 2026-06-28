package corpus

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/iannil/jianwu/internal/storage"
	"github.com/iannil/jianwu/internal/workspace"
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

// LoadWithWorkspace loads corpus books layered: workspace overrides + builtin fallback.
// Workspace corpus files live in <wsRoot>/.jianwu/corpus/<slug>.json and
// override builtin books with the same slug. Non-existent workspace corpus directory
// is silently ignored.
func LoadWithWorkspace(wsRoot string) (map[string]*Book, error) {
	out := make(map[string]*Book)

	// Load workspace corpus first (lowest priority, will be overridden by builtin? No —
	// workspace overrides mean user-synced data should WIN over builtin. So load
	// builtin first, then overlay workspace on top.)
	builtin, err := Load()
	if err != nil {
		return nil, fmt.Errorf("load builtin corpus: %w", err)
	}
	for k, v := range builtin {
		out[k] = v
	}

	// Load workspace corpus (overrides builtin)
	corpusDir := filepath.Join(wsRoot, workspace.MarkerName, workspace.CorpusDirName)
	entries, err := storage.OS.ReadDir(corpusDir)
	if err != nil {
		// Directory doesn't exist — that's fine, just use builtin.
		return out, nil
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := storage.OS.ReadFile(filepath.Join(corpusDir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read workspace corpus %s: %w", e.Name(), err)
		}
		var b Book
		if err := json.Unmarshal(data, &b); err != nil {
			return nil, fmt.Errorf("parse workspace corpus %s: %w", e.Name(), err)
		}
		if b.Slug == "" {
			return nil, fmt.Errorf("workspace corpus book in %s has empty slug", e.Name())
		}
		out[b.Slug] = &b
	}

	return out, nil
}
