package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobalWorkspaceRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	SetGlobalConfigPath(path)
	t.Cleanup(func() { SetGlobalConfigPath("") })

	// Missing file → empty.
	ws, err := GlobalWorkspace()
	if err != nil {
		t.Fatalf("GlobalWorkspace on missing file: %v", err)
	}
	if ws != "" {
		t.Fatalf("workspace = %q, want empty", ws)
	}

	// Set creates the file with the key.
	if err := SetGlobalWorkspace("/tmp/books-a"); err != nil {
		t.Fatalf("SetGlobalWorkspace: %v", err)
	}
	ws, err = GlobalWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if ws != "/tmp/books-a" {
		t.Fatalf("workspace = %q, want /tmp/books-a", ws)
	}

	// Updating preserves other keys and comments.
	original := "# my config comment\nmodels:\n  outline:\n    provider: gemini\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetGlobalWorkspace("/tmp/books-b"); err != nil {
		t.Fatalf("SetGlobalWorkspace over existing: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := string(data)
	for _, want := range []string{"# my config comment", "provider: gemini", "workspace: /tmp/books-b"} {
		if !strings.Contains(updated, want) {
			t.Errorf("updated config missing %q:\n%s", want, updated)
		}
	}
}

func TestSetGlobalWorkspaceRejectsNonMapping(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	SetGlobalConfigPath(path)
	t.Cleanup(func() { SetGlobalConfigPath("") })

	if err := os.WriteFile(path, []byte("- a\n- b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetGlobalWorkspace("/tmp/x"); err == nil {
		t.Fatal("expected error for non-mapping top level")
	}
}
