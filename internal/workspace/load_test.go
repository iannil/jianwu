package workspace

import (
	"errors"
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
