package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesFullWorkspace(t *testing.T) {
	root := t.TempDir()

	if err := Init(root, InitOpts{}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	for _, p := range []string{
		MarkerName,
		MarkerName + "/" + ConfigFileName,
		"books",
		"exports",
		"archive",
	} {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
}

func TestInitBareOmitsBooksDirs(t *testing.T) {
	root := t.TempDir()

	if err := Init(root, InitOpts{Bare: true}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// .jianwu/ must still exist
	if _, err := os.Stat(filepath.Join(root, MarkerName)); err != nil {
		t.Errorf(".jianwu missing: %v", err)
	}
	// books/ etc. must NOT exist
	for _, p := range []string{"books", "exports", "archive"} {
		if _, err := os.Stat(filepath.Join(root, p)); err == nil {
			t.Errorf("%s/ should not exist with --bare", p)
		}
	}
}

func TestInitExistingReturnsError(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, InitOpts{}); err != nil {
		t.Fatal(err)
	}
	err := Init(root, InitOpts{})
	if err == nil {
		t.Error("expected error on re-init, got nil")
	}
}
