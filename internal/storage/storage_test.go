package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMemStorageRoundTrip(t *testing.T) {
	m := NewMemStorage()

	// WriteFile + ReadFile
	data := []byte("hello world")
	if err := m.WriteFile("/tmp/test.txt", data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := m.ReadFile("/tmp/test.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("ReadFile: got %q want %q", string(got), string(data))
	}
}

func TestMemStorageReadMissing(t *testing.T) {
	m := NewMemStorage()
	_, err := m.ReadFile("/nonexistent")
	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist, got %v", err)
	}
}

func TestMemStorageStat(t *testing.T) {
	m := NewMemStorage()
	if err := m.WriteFile("/tmp/a.txt", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	fi, err := m.Stat("/tmp/a.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi.Name() != "/tmp/a.txt" {
		t.Errorf("Name: got %q want %q", fi.Name(), "/tmp/a.txt")
	}
	if fi.Size() != 5 {
		t.Errorf("Size: got %d want 5", fi.Size())
	}
	if fi.IsDir() {
		t.Error("expected IsDir() == false")
	}
	if fi.Mode() != 0o644 {
		t.Errorf("Mode: got %o want 644", fi.Mode())
	}
	if fi.ModTime().IsZero() {
		t.Error("ModTime should not be zero")
	}
}

func TestMemStorageStatMissing(t *testing.T) {
	m := NewMemStorage()
	_, err := m.Stat("/does/not/exist")
	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist, got %v", err)
	}
}

func TestMemStorageRename(t *testing.T) {
	m := NewMemStorage()
	m.WriteFile("/tmp/old.txt", []byte("data"), 0o644)

	if err := m.Rename("/tmp/old.txt", "/tmp/new.txt"); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	// Old should be gone.
	if _, err := m.ReadFile("/tmp/old.txt"); !os.IsNotExist(err) {
		t.Error("old file should not exist after rename")
	}

	// New should have data.
	data, err := m.ReadFile("/tmp/new.txt")
	if err != nil {
		t.Fatalf("ReadFile new: %v", err)
	}
	if string(data) != "data" {
		t.Errorf("got %q want %q", string(data), "data")
	}
}

func TestMemStorageRenameMissing(t *testing.T) {
	m := NewMemStorage()
	err := m.Rename("/nothing", "/stillnothing")
	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist, got %v", err)
	}
}

func TestMemStorageRemoveAll(t *testing.T) {
	m := NewMemStorage()
	m.WriteFile("/books/a/ch01.md", []byte("c1"), 0o644)
	m.WriteFile("/books/a/ch02.md", []byte("c2"), 0o644)
	m.WriteFile("/books/b/ch01.md", []byte("c3"), 0o644)
	m.WriteFile("/config.yaml", []byte("cfg"), 0o644)

	if err := m.RemoveAll("/books/a"); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	// /books/a files should be gone.
	if _, err := m.ReadFile("/books/a/ch01.md"); !os.IsNotExist(err) {
		t.Error("/books/a/ch01.md should be gone")
	}
	// /books/b should remain.
	if _, err := m.ReadFile("/books/b/ch01.md"); err != nil {
		t.Error("/books/b/ch01.md should remain")
	}
	// /config.yaml should remain.
	if _, err := m.ReadFile("/config.yaml"); err != nil {
		t.Error("/config.yaml should remain")
	}
}

func TestMemStorageRemoveAllRoot(t *testing.T) {
	m := NewMemStorage()
	m.WriteFile("/books/a/ch01.md", []byte("c1"), 0o644)
	m.WriteFile("/config.yaml", []byte("cfg"), 0o644)

	if err := m.RemoveAll(""); err != nil {
		t.Fatalf("RemoveAll empty: %v", err)
	}
	if len(m.files) != 0 {
		t.Errorf("expected empty after RemoveAll(\"\"), got %d files", len(m.files))
	}
}

func TestMemStorageReadDir(t *testing.T) {
	m := NewMemStorage()
	m.WriteFile("/root/a.txt", []byte("a"), 0o644)
	m.WriteFile("/root/sub/b.txt", []byte("b"), 0o644)
	m.WriteFile("/root/sub/c.txt", []byte("c"), 0o644)
	m.WriteFile("/other/d.txt", []byte("d"), 0o644)

	entries, err := m.ReadDir("/root")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	// Should see: a.txt (file), sub (directory, implied by /root/sub/b.txt)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(entries), names(entries))
	}

	if entries[0].Name() != "a.txt" || entries[1].Name() != "sub" {
		t.Errorf("expected [a.txt sub], got %v", names(entries))
	}
	if entries[0].IsDir() {
		t.Error("a.txt should not be a dir")
	}
	if !entries[1].IsDir() {
		t.Error("sub should be a dir")
	}

	// Info() should work.
	info, err := entries[0].Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Size() != 1 {
		t.Errorf("a.txt size: got %d want 1", info.Size())
	}
}

func TestMemStorageReadDirEmpty(t *testing.T) {
	m := NewMemStorage()
	entries, err := m.ReadDir("/empty")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestMemStorageMkdirAll(t *testing.T) {
	m := NewMemStorage()
	if err := m.MkdirAll("/a/b/c", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// MkdirAll is a no-op for MemStorage — just shouldn't error.
}

// OS storage tests (require real filesystem).
func TestOSStorageRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	if err := OS.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, err := OS.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q want %q", string(data), "hello")
	}
}

func TestOSStorageStat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stat.txt")
	OS.WriteFile(path, []byte("data"), 0o644)

	fi, err := OS.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi.Name() != "stat.txt" {
		t.Errorf("Name: got %q want %q", fi.Name(), "stat.txt")
	}
	if fi.Size() != 4 {
		t.Errorf("Size: got %d want 4", fi.Size())
	}
}

func TestOSStorageRename(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	newPath := filepath.Join(dir, "new.txt")

	OS.WriteFile(oldPath, []byte("data"), 0o644)
	if err := OS.Rename(oldPath, newPath); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if _, err := OS.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old file should be gone after rename")
	}
	if _, err := OS.Stat(newPath); err != nil {
		t.Error("new file should exist after rename")
	}
}

func TestOSStorageReadDir(t *testing.T) {
	dir := t.TempDir()
	OS.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)

	entries, err := OS.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestOSStorageReadDirRoot(t *testing.T) {
	_, err := OS.ReadDir("/")
	if err != nil {
		t.Fatalf("ReadDir /: %v", err)
	}
}

func names(entries []os.DirEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name()
	}
	return out
}
