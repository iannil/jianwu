package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallWritesSkill(t *testing.T) {
	dir := t.TempDir()
	path, err := Install(dir)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	want := filepath.Join(dir, "jianwu", "SKILL.md")
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	raw, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, sub := range []string{"name: jianwu", "MCP", "review 是人工批准", "发布硬门"} {
		if !strings.Contains(s, sub) {
			t.Errorf("skill missing %q", sub)
		}
	}
}

func TestContentMatchesInstall(t *testing.T) {
	if len(Content()) == 0 {
		t.Fatal("embedded skill is empty")
	}
}
