package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReturnsConfig(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, InitOpts{}); err != nil {
		t.Fatal(err)
	}

	ws, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if ws.Root != root {
		t.Errorf("Root: got %q want %q", ws.Root, root)
	}
	if ws.Config == nil {
		t.Error("Config is nil")
	}
}

func TestLoadChecksSchemaVersion(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, InitOpts{}); err != nil {
		t.Fatal(err)
	}
	// Corrupt schema_version
	if err := overwriteFile(filepath.Join(root, MarkerName, SchemaVersionFileName), "99"); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root)
	if err == nil {
		t.Error("expected schema mismatch error, got nil")
	}
}

func overwriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func TestLoadReturnsErrorWhenMarkerNotFound(t *testing.T) {
	root := t.TempDir()
	_, err := Load(root)
	if err == nil {
		t.Error("expected error for missing .jianwu directory, got nil")
	}
	if !errors.Is(err, ErrWorkspaceNotFound) {
		t.Errorf("got %v, want ErrWorkspaceNotFound", err)
	}
}

func TestLoadReturnsErrorWhenSchemaVersionMissing(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, InitOpts{}); err != nil {
		t.Fatal(err)
	}
	// Remove schema_version file
	if err := os.Remove(filepath.Join(root, MarkerName, SchemaVersionFileName)); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root)
	if err == nil {
		t.Error("expected error for missing schema_version file, got nil")
	}
}
