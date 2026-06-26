package book

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ChapterFrontmatter is the YAML frontmatter schema for chapters/NN-MM.md.
// Per decision Q2=B: middle-weight schema, self-contained enough that
// reading the file alone tells you its provenance.
type ChapterFrontmatter struct {
	Title                 string            `yaml:"title"`
	PartIndex             int               `yaml:"part_id"`
	ChapterIndex          int               `yaml:"chapter_id"`
	Status                string            `yaml:"status"`
	WordCount             int               `yaml:"word_count"`
	GeneratedAt           time.Time         `yaml:"generated_at"`
	Model                 string            `yaml:"model"`
	EngineVersion         string            `yaml:"engine_version"`
	Citations             []ChapterCitation `yaml:"citations,omitempty"`
	UnverifiedClaimsCount int               `yaml:"unverified_claims_count"`
}

// ChapterCitation is the citation entry embedded in chapter frontmatter.
type ChapterCitation struct {
	ID    string `yaml:"id"`
	URL   string `yaml:"url"`
	Title string `yaml:"title,omitempty"`
	Site  string `yaml:"site,omitempty"`
}

// ChapterPath returns the canonical path for a chapter file: <bookDir>/chapters/NN-MM.md
// partIdx and chIdx are 1-based; zero-padded to 2 digits.
func ChapterPath(bookDir string, partIdx, chIdx int) string {
	return filepath.Join(bookDir, "chapters", fmt.Sprintf("%02d-%02d.md", partIdx, chIdx))
}

// WriteChapter writes a chapter file with YAML frontmatter + markdown body.
// Creates chapters/ subdir if missing. Returns the absolute path written.
func WriteChapter(bookDir string, partIdx, chIdx int, fm ChapterFrontmatter, markdown string) (string, error) {
	chaptersDir := filepath.Join(bookDir, "chapters")
	if err := DefaultStorage.MkdirAll(chaptersDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir chapters: %w", err)
	}

	yamlBytes, err := yaml.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("marshal frontmatter: %w", err)
	}

	path := ChapterPath(bookDir, partIdx, chIdx)
	content := fmt.Sprintf("---\n%s---\n\n%s\n", string(yamlBytes), markdown)
	if err := DefaultStorage.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write chapter: %w", err)
	}
	return path, nil
}

// ReadChapter reads a chapter file, returning the frontmatter + body.
// Returns os.IsNotExist error if file missing.
func ReadChapter(path string) (*ChapterFrontmatter, string, error) {
	raw, err := DefaultStorage.ReadFile(path)
	if err != nil {
		return nil, "", err
	}

	content := string(raw)
	if !strings.HasPrefix(content, "---\n") {
		return nil, "", fmt.Errorf("missing frontmatter delimiter")
	}

	// Find the closing --- delimiter.
	rest := content[4:]
	endIdx := strings.Index(rest, "\n---\n")
	if endIdx < 0 {
		return nil, "", fmt.Errorf("malformed frontmatter: no closing delimiter")
	}

	yamlPart := rest[:endIdx]
	body := strings.TrimSpace(rest[endIdx+5:])

	var fm ChapterFrontmatter
	if err := yaml.Unmarshal([]byte(yamlPart), &fm); err != nil {
		return nil, "", fmt.Errorf("unmarshal frontmatter: %w", err)
	}
	return &fm, body, nil
}
