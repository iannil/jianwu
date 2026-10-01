package corpus

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iannil/jianwu/internal/storage"
	"github.com/iannil/jianwu/internal/workspace"
)

// CorpusDir returns the workspace corpus directory (<wsRoot>/.jianwu/corpus).
func CorpusDir(wsRoot string) string {
	return filepath.Join(wsRoot, workspace.MarkerName, workspace.CorpusDirName)
}

// BookPath returns the corpus file path for one book slug.
func BookPath(wsRoot, slug string) string {
	return filepath.Join(CorpusDir(wsRoot), slug+".json")
}

// Load parses all workspace corpus JSON files keyed by book slug.
// There is no builtin corpus: the workspace corpus is the only source, and it
// is populated by `corpus sync`, `corpus collect`, or hand-authored files.
// A missing corpus directory yields an empty map, not an error.
func Load(wsRoot string) (map[string]*Book, error) {
	out := make(map[string]*Book)
	if wsRoot == "" {
		return out, nil
	}

	corpusDir := CorpusDir(wsRoot)
	entries, err := storage.OS.ReadDir(corpusDir)
	if err != nil {
		// Directory doesn't exist — no corpus collected yet.
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

// List loads the workspace corpus and returns books sorted by slug, for
// callers that need deterministic order (e.g. prompt rendering).
func List(wsRoot string) ([]*Book, error) {
	m, err := Load(wsRoot)
	if err != nil {
		return nil, err
	}
	slugs := make([]string, 0, len(m))
	for slug := range m {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	out := make([]*Book, 0, len(slugs))
	for _, slug := range slugs {
		out = append(out, m[slug])
	}
	return out, nil
}

// SaveBook marshals a corpus book and writes it into the workspace corpus
// directory, creating the directory if needed. Overwrites existing files with
// the same slug.
func SaveBook(wsRoot string, b *Book) error {
	if b.Slug == "" {
		return fmt.Errorf("corpus book has empty slug")
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal corpus book %s: %w", b.Slug, err)
	}
	dir := CorpusDir(wsRoot)
	if err := storage.OS.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create corpus dir: %w", err)
	}
	if err := storage.OS.WriteFile(BookPath(wsRoot, b.Slug), data, 0o644); err != nil {
		return fmt.Errorf("write corpus book %s: %w", b.Slug, err)
	}
	return nil
}
