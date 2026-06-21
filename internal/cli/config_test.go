package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestConfigGetReturnsValue(t *testing.T) {
	root := t.TempDir()
	if err := runInit(root, false); err != nil {
		t.Fatal(err)
	}
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"config", "get", "models.outline.provider"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if got != "gemini" {
		t.Errorf("got %q, want %q", got, "gemini")
	}
}

func TestConfigSetWritesToWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := runInit(root, false); err != nil {
		t.Fatal(err)
	}
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"config", "set", "models.outline.provider", "glm"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Verify by re-reading
	cmd2 := NewRootCmd()
	out := &bytes.Buffer{}
	cmd2.SetOut(out)
	cmd2.SetArgs([]string{"config", "get", "models.outline.provider"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if got != "glm" {
		t.Errorf("after set, got %q, want %q", got, "glm")
	}
}

func TestConfigListShowsAllKeys(t *testing.T) {
	root := t.TempDir()
	if err := runInit(root, false); err != nil {
		t.Fatal(err)
	}
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"config", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	s := out.String()
	for _, want := range []string{"models.", "search.", "scaffolding.", "logging."} {
		if !strings.Contains(s, want) {
			t.Errorf("list missing %q: %q", want, s)
		}
	}
}

func TestConfigGetUnknownKeyErrors(t *testing.T) {
	root := t.TempDir()
	if err := runInit(root, false); err != nil {
		t.Fatal(err)
	}
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"config", "get", "nonexistent.key"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for unknown key, got nil")
	}
}
