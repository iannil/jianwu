// Package storage defines a Storage interface for all file I/O in jianwu.
// This allows replacing the OS filesystem with in-memory or test implementations.
package storage

import "os"

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
	files map[string][]byte
}

func NewMemStorage() *MemStorage { return &MemStorage{files: map[string][]byte{}} }

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
	for k := range m.files {
		if len(k) >= len(path) && k[:len(path)] == path {
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
	_, ok := m.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return nil, nil // minimal stub
}
func (m *MemStorage) ReadDir(name string) ([]os.DirEntry, error) {
	return nil, nil // not implemented for mem
}
