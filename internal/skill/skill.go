// Package skill ships the jianwu agent skill: an installable SKILL.md that
// teaches AI agents the jianwu workflow, its four access modes (MCP / API /
// RSS / CLI) and the human-gate rules (ADR 30).
package skill

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed SKILL.md
var skillFS embed.FS

// Content returns the embedded SKILL.md bytes.
func Content() []byte {
	b, err := skillFS.ReadFile("SKILL.md")
	if err != nil {
		return []byte("")
	}
	return b
}

// DefaultDir is the default agent skills root (the ~/.agents/skills
// convention understood by common agent runtimes).
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".agents/skills"
	}
	return filepath.Join(home, ".agents", "skills")
}

// Install writes the skill to <dir>/jianwu/SKILL.md and returns the path.
func Install(dir string) (string, error) {
	if dir == "" {
		dir = DefaultDir()
	}
	dst := filepath.Join(dir, "jianwu", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("mkdir skill dir: %w", err)
	}
	if err := os.WriteFile(dst, Content(), 0o644); err != nil {
		return "", fmt.Errorf("write skill: %w", err)
	}
	return dst, nil
}
