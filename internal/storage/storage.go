// Package storage defines a Storage interface for all file I/O in jianwu.
// This allows replacing the OS filesystem with in-memory or test implementations.
package storage

import (
	"os"
	"sort"
	"strings"
	"time"
)

// Storage abstracts filesystem operations used across jianwu.
// Operations mirror os.* functions for straightforward wrapping.
type Storage interface {
	// ReadFile returns the contents of the file at path.
	ReadFile(path string) ([]byte, error)
	// WriteFile writes data to path, creating parent directories as needed.
	// perm is used only when creating new files.
	WriteFile(path string, data []byte, perm os.FileMode) error
	// MkdirAll creates a directory and all parents.
	MkdirAll(path string, perm os.FileMode) error
	// RemoveAll removes path and any children.
	RemoveAll(path string) error
	// Rename atomically moves a file from oldPath to newPath (same filesystem).
	Rename(oldPath, newPath string) error
	// Stat returns file info. os.IsNotExist(err) works on the error.
	Stat(path string) (os.FileInfo, error)
	// ReadDir returns the names of entries in directory.
	ReadDir(name string) ([]os.DirEntry, error)
}

// OS is the default Storage backed by the OS filesystem.
var OS Storage = osStorage{}

type osStorage struct{}

func (osStorage) ReadFile(path string) ([]byte, error)          { return os.ReadFile(path) }
func (osStorage) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}
func (osStorage) MkdirAll(path string, perm os.FileMode) error  { return os.MkdirAll(path, perm) }
func (osStorage) RemoveAll(path string) error                   { return os.RemoveAll(path) }
func (osStorage) Rename(oldPath, newPath string) error          { return os.Rename(oldPath, newPath) }
func (osStorage) Stat(path string) (os.FileInfo, error)         { return os.Stat(path) }
func (osStorage) ReadDir(name string) ([]os.DirEntry, error)   { return os.ReadDir(name) }

// MemStorage is an in-memory Storage for tests.
type MemStorage struct {
	files   map[string][]byte
	modTime time.Time // shared timestamp for all entries
}

func NewMemStorage() *MemStorage {
	return &MemStorage{files: map[string][]byte{}, modTime: time.Now()}
}

func (m *MemStorage) ReadFile(path string) ([]byte, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return data, nil
}
func (m *MemStorage) WriteFile(path string, data []byte, _ os.FileMode) error {
	m.files[path] = data
	return nil
}
func (m *MemStorage) MkdirAll(_ string, _ os.FileMode) error { return nil }
func (m *MemStorage) RemoveAll(path string) error {
	prefix := path
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	for k := range m.files {
		if k == path || strings.HasPrefix(k, prefix) {
			delete(m.files, k)
		}
	}
	return nil
}
func (m *MemStorage) Rename(oldPath, newPath string) error {
	data, ok := m.files[oldPath]
	if !ok {
		return os.ErrNotExist
	}
	m.files[newPath] = data
	delete(m.files, oldPath)
	return nil
}
func (m *MemStorage) Stat(path string) (os.FileInfo, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &memFileInfo{name: path, size: int64(len(data)), modTime: m.modTime}, nil
}
func (m *MemStorage) ReadDir(name string) ([]os.DirEntry, error) {
	prefix := name
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	var entries []os.DirEntry
	seen := map[string]bool{}
	for k := range m.files {
		if k == name {
			continue
		}
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		rest := k[len(prefix):]
		// Only collect the top-level entry under this prefix.
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			rest = rest[:slash]
		}
		if seen[rest] {
			continue
		}
		seen[rest] = true
		entryPath := prefix + rest
		data, ok := m.files[entryPath]
		isDir := !ok
		var size int64
		if ok {
			size = int64(len(data))
		}
		entries = append(entries, &memDirEntry{name: rest, size: size, modTime: m.modTime, isDir: isDir})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

// memFileInfo implements os.FileInfo for MemStorage.
type memFileInfo struct {
	name    string
	size    int64
	modTime time.Time
}

func (f *memFileInfo) Name() string       { return f.name }
func (f *memFileInfo) Size() int64        { return f.size }
func (f *memFileInfo) Mode() os.FileMode  { return 0o644 }
func (f *memFileInfo) ModTime() time.Time { return f.modTime }
func (f *memFileInfo) IsDir() bool        { return false }
func (f *memFileInfo) Sys() any           { return nil }

// memDirEntry implements os.DirEntry for MemStorage.
type memDirEntry struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
}

func (e *memDirEntry) Name() string               { return e.name }
func (e *memDirEntry) IsDir() bool                 { return e.isDir }
func (e *memDirEntry) Type() os.FileMode           { return e.Mode().Type() }
func (e *memDirEntry) Info() (os.FileInfo, error)  { return &memFileInfo{name: e.name, size: e.size, modTime: e.modTime}, nil }
func (e *memDirEntry) Mode() os.FileMode {
	if e.isDir {
		return os.ModeDir | 0o755
	}
	return 0o644
}
