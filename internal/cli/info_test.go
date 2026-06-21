package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInfoFromInsideWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := runInit(root, false); err != nil {
		t.Fatal(err)
	}
	// Make a subdir to test walk-up
	sub := filepath.Join(root, "books", "mybook")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"info"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	s := out.String()
	if !strings.Contains(s, "Workspace:") {
		t.Errorf("output missing 'Workspace:': %q", s)
	}
	if !strings.Contains(s, root) {
		t.Errorf("output missing root path %q: %q", root, s)
	}
	if !strings.Contains(s, "Models:") {
		t.Errorf("output missing 'Models:': %q", s)
	}
}

func TestInfoOutsideWorkspaceReturnsExit3(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"info"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// The CLI main should map workspace errors to ExitCodeWorkspaceNotFound;
	// here we just check the error is non-nil and recognizable.
	if !strings.Contains(err.Error(), "workspace") {
		t.Errorf("error should mention workspace, got: %v", err)
	}
}

// runInit is a test helper that creates a workspace at root.
func runInit(root string, bare bool) error {
	cmd := NewRootCmd()
	args := []string{"init", root}
	if bare {
		args = []string{"init", "--bare", root}
	}
	cmd.SetArgs(args)
	return cmd.Execute()
}
