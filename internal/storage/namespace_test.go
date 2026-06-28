package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNamespacePrefixAddedAutomatically(t *testing.T) {
	ns := NewNamespace(NewMemStorage(), "tenants/acme")
	want := "tenants/acme/"
	if ns.Prefix() != want {
		t.Errorf("Prefix() = %q, want %q", ns.Prefix(), want)
	}
}

func TestNamespaceEmptyPrefixPassthrough(t *testing.T) {
	mem := NewMemStorage()
	ns := NewNamespace(mem, "")

	if err := ns.WriteFile("/test.txt", []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := ns.ReadFile("/test.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "data" {
		t.Errorf("got %q want %q", string(data), "data")
	}
}

func TestNamespaceReadFile(t *testing.T) {
	mem := NewMemStorage()
	ns := NewNamespace(mem, "tenant-a/")

	mem.WriteFile("tenant-a/hello.txt", []byte("scoped"), 0o644)
	mem.WriteFile("hello.txt", []byte("unscoped"), 0o644)

	data, err := ns.ReadFile("hello.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "scoped" {
		t.Errorf("got %q want %q", string(data), "scoped")
	}
}

func TestNamespaceWriteFileCreatesParentDirs(t *testing.T) {
	mem := NewMemStorage()
	ns := NewNamespace(mem, "tenant-a/")

	if err := ns.WriteFile("books/deep/ch01.md", []byte("# Chapter 1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	data, err := mem.ReadFile("tenant-a/books/deep/ch01.md")
	if err != nil {
		t.Fatalf("underlying file not found: %v", err)
	}
	if string(data) != "# Chapter 1" {
		t.Errorf("got %q", string(data))
	}
}

func TestNamespaceMkdirAll(t *testing.T) {
	mem := NewMemStorage()
	ns := NewNamespace(mem, "tenant-a/")

	if err := ns.MkdirAll("books/deep", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write into that dir via ns to confirm path mapping
	ns.WriteFile("books/deep/ch01.md", []byte("data"), 0o644)
	if _, err := mem.ReadFile("tenant-a/books/deep/ch01.md"); err != nil {
		t.Error("file should exist under prefixed path")
	}
}

func TestNamespaceRemoveAll(t *testing.T) {
	mem := NewMemStorage()
	ns := NewNamespace(mem, "tenant-a/")

	ns.WriteFile("books/a/ch01.md", []byte("c1"), 0o644)
	ns.WriteFile("books/a/ch02.md", []byte("c2"), 0o644)
	mem.WriteFile("tenant-b/books/a/ch01.md", []byte("other"), 0o644)

	if err := ns.RemoveAll("books/a"); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	if _, err := ns.ReadFile("books/a/ch01.md"); !os.IsNotExist(err) {
		t.Error("removed file should not exist")
	}
	if _, err := mem.ReadFile("tenant-b/books/a/ch01.md"); err != nil {
		t.Error("other tenant's files should remain untouched")
	}
}

func TestNamespaceRename(t *testing.T) {
	mem := NewMemStorage()
	ns := NewNamespace(mem, "tenant-a/")

	ns.WriteFile("old.md", []byte("data"), 0o644)
	if err := ns.Rename("old.md", "new.md"); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	if _, err := ns.ReadFile("old.md"); !os.IsNotExist(err) {
		t.Error("old file should not exist after rename")
	}
	data, err := ns.ReadFile("new.md")
	if err != nil {
		t.Fatalf("ReadFile new: %v", err)
	}
	if string(data) != "data" {
		t.Errorf("got %q want %q", string(data), "data")
	}
}

func TestNamespaceStat(t *testing.T) {
	mem := NewMemStorage()
	ns := NewNamespace(mem, "tenant-a/")

	mem.WriteFile("tenant-a/test.txt", []byte("hello"), 0o644)

	fi, err := ns.Stat("test.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi.Size() != 5 {
		t.Errorf("Size: got %d want 5", fi.Size())
	}
}

func TestNamespaceReadDir(t *testing.T) {
	mem := NewMemStorage()
	ns := NewNamespace(mem, "tenant-a/")

	mem.WriteFile("tenant-a/a.txt", []byte("a"), 0o644)
	mem.WriteFile("tenant-a/sub/b.txt", []byte("b"), 0o644)
	mem.WriteFile("tenant-b/x.txt", []byte("x"), 0o644) // should be invisible

	entries, err := ns.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (a.txt, sub), got %d: %v", len(entries), names(entries))
	}
	if entries[0].Name() != "a.txt" {
		t.Errorf("expected a.txt first, got %s", entries[0].Name())
	}
	if !entries[1].IsDir() {
		t.Errorf("sub should be a directory")
	}
}

func TestNamespaceReadDirDoesNotLeakOtherTenants(t *testing.T) {
	mem := NewMemStorage()
	ns := NewNamespace(mem, "tenant-a/")

	mem.WriteFile("tenant-a/report.md", []byte("a"), 0o644)
	mem.WriteFile("tenant-b/report.md", []byte("b"), 0o644)
	mem.WriteFile("other-tenant/report.md", []byte("c"), 0o644)

	entries, err := ns.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d (tenant leak detected)", len(entries))
	}
}

func TestNamespaceWithOSFileSystem(t *testing.T) {
	dir := t.TempDir()
	prefix := "tenants/acme/"
	ns := NewNamespace(OS, filepath.Join(dir, prefix))

	if err := ns.WriteFile("test.txt", []byte("data"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Verify file exists at the prefixed OS path
	fullPath := filepath.Join(dir, prefix, "test.txt")
	if _, err := os.Stat(fullPath); err != nil {
		t.Errorf("file should exist at %s: %v", fullPath, err)
	}

	// Read back through namespace
	data, err := ns.ReadFile("test.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "data" {
		t.Errorf("got %q want %q", string(data), "data")
	}
}
