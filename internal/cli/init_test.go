package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesWorkspaceInCwd(t *testing.T) {
	dir := t.TempDir()
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"init", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".jianwu")); err != nil {
		t.Errorf(".jianwu not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "books")); err != nil {
		t.Errorf("books/ not created: %v", err)
	}
}

func TestInitBareFlag(t *testing.T) {
	dir := t.TempDir()
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"init", "--bare", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".jianwu")); err != nil {
		t.Errorf(".jianwu not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "books")); err == nil {
		t.Error("books/ should not exist with --bare")
	}
}

func TestInitDefaultsToCwd(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"init"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".jianwu")); err != nil {
		t.Errorf(".jianwu not created in cwd: %v", err)
	}
}
