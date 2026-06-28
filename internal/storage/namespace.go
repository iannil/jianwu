package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Namespace wraps a Storage to add a prefix to all paths, providing
// per-tenant namespace isolation. All read/write/stat/list/remove
// operations are scoped to the namespace prefix.
//
// The prefix is expected to have a trailing separator (e.g. "tenants/acme/").
// If it doesn't, one is added automatically.
//
// Use NewNamespace to construct.
type Namespace struct {
	impl   Storage
	prefix string
}

// NewNamespace creates a Namespace that prepends prefix to every path.
// prefix is typically "tenants/<tenantID>/" for per-tenant isolation.
// An empty prefix creates a no-op passthrough.
func NewNamespace(impl Storage, prefix string) *Namespace {
	p := prefix
	if p != "" && !strings.HasSuffix(p, string(filepath.Separator)) {
		p += string(filepath.Separator)
	}
	return &Namespace{impl: impl, prefix: p}
}

// Prefix returns the namespace prefix (with trailing separator).
func (ns *Namespace) Prefix() string { return ns.prefix }

// nsPath prepends the namespace prefix to path, normalizing "." to empty.
func (ns *Namespace) nsPath(path string) string {
	if path == "." || path == "./" {
		path = ""
	}
	return ns.prefix + path
}

// ReadFile reads a file within the namespace.
func (ns *Namespace) ReadFile(path string) ([]byte, error) {
	return ns.impl.ReadFile(ns.nsPath(path))
}

// WriteFile writes a file within the namespace, creating parent dirs.
func (ns *Namespace) WriteFile(path string, data []byte, perm os.FileMode) error {
	full := ns.nsPath(path)
	// Ensure parent directory exists.
	parent := filepath.Dir(full)
	if err := ns.impl.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("mkdir parent for %s: %w", path, err)
	}
	return ns.impl.WriteFile(full, data, perm)
}

// MkdirAll creates a directory within the namespace.
func (ns *Namespace) MkdirAll(path string, perm os.FileMode) error {
	return ns.impl.MkdirAll(ns.nsPath(path), perm)
}

// RemoveAll removes a path and its children within the namespace.
func (ns *Namespace) RemoveAll(path string) error {
	return ns.impl.RemoveAll(ns.nsPath(path))
}

// Rename moves a file within the namespace.
func (ns *Namespace) Rename(oldPath, newPath string) error {
	return ns.impl.Rename(ns.nsPath(oldPath), ns.nsPath(newPath))
}

// Stat returns file info within the namespace.
func (ns *Namespace) Stat(path string) (os.FileInfo, error) {
	return ns.impl.Stat(ns.nsPath(path))
}

// ReadDir lists directory entries within the namespace.
func (ns *Namespace) ReadDir(name string) ([]os.DirEntry, error) {
	return ns.impl.ReadDir(ns.nsPath(name))
}

// compile-time interface check
var _ Storage = (*Namespace)(nil)
