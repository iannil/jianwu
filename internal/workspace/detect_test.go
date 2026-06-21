package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindWorkspaceInCurrentDir(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, MarkerName), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindWorkspace(root)
	if err != nil {
		t.Fatalf("FindWorkspace: %v", err)
	}
	if got != root {
		t.Errorf("got %q want %q", got, root)
	}
}

func TestFindWorkspaceWalksUp(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, MarkerName), 0o755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "books", "my-book", "chapters")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindWorkspace(deep)
	if err != nil {
		t.Fatalf("FindWorkspace: %v", err)
	}
	if got != root {
		t.Errorf("got %q want %q", got, root)
	}
}

func TestFindWorkspaceReturnsErrorWhenNotFound(t *testing.T) {
	root := t.TempDir()
	_, err := FindWorkspace(root)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != ErrWorkspaceNotFound {
		t.Errorf("got %v, want ErrWorkspaceNotFound", err)
	}
}
