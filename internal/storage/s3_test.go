package storage

import (
	"errors"
	"testing"
)

func TestNewS3StorageRequiresBucket(t *testing.T) {
	_, err := NewS3Storage("", "prefix/")
	if err == nil {
		t.Fatal("expected error for empty bucket")
	}
}

func TestNewS3StorageValid(t *testing.T) {
	s, err := NewS3Storage("my-bucket", "prefix/")
	if err != nil {
		t.Fatalf("NewS3Storage: %v", err)
	}
	if s.Bucket() != "my-bucket" {
		t.Errorf("Bucket = %q, want %q", s.Bucket(), "my-bucket")
	}
	if s.KeyPrefix() != "prefix/" {
		t.Errorf("KeyPrefix = %q, want %q", s.KeyPrefix(), "prefix/")
	}
}

func TestNewS3StorageEmptyPrefix(t *testing.T) {
	s, err := NewS3Storage("bucket", "")
	if err != nil {
		t.Fatalf("NewS3Storage: %v", err)
	}
	if s.KeyPrefix() != "" {
		t.Errorf("expected empty prefix, got %q", s.KeyPrefix())
	}
}

func TestS3StorageReturnsNotImplemented(t *testing.T) {
	s, err := NewS3Storage("bucket", "")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		fn   func() error
	}{
		{"ReadFile", func() error { _, err := s.ReadFile("/test"); return err }},
		{"WriteFile", func() error { return s.WriteFile("/test", []byte("x"), 0o644) }},
		{"MkdirAll", func() error { return s.MkdirAll("/dir", 0o755) }},
		{"RemoveAll", func() error { return s.RemoveAll("/dir") }},
		{"Rename", func() error { return s.Rename("/a", "/b") }},
		{"Stat", func() error { _, err := s.Stat("/test"); return err }},
		{"ReadDir", func() error { _, err := s.ReadDir("/"); return err }},
	}

	for _, tc := range tests {
		err := tc.fn()
		if !errors.Is(err, ErrS3NotImplemented) {
			t.Errorf("%s: expected ErrS3NotImplemented, got %v", tc.name, err)
		}
	}
}

func TestS3StorageImplementsInterface(t *testing.T) {
	// compile-time check via assignment
	var s Storage
	var err error
	s, err = NewS3Storage("bucket", "")
	if err != nil {
		t.Fatal(err)
	}
	_ = s
}

func TestS3StoragePathToKey(t *testing.T) {
	s, err := NewS3Storage("bucket", "prefix/")
	if err != nil {
		t.Fatal(err)
	}
	key := s.pathToKey("books/my-book/ch01.md")
	if key != "prefix/books/my-book/ch01.md" {
		t.Errorf("pathToKey = %q, want %q", key, "prefix/books/my-book/ch01.md")
	}
}

func TestS3StoragePathToKeyNoPrefix(t *testing.T) {
	s, err := NewS3Storage("bucket", "")
	if err != nil {
		t.Fatal(err)
	}
	key := s.pathToKey("books/my-book/ch01.md")
	if key != "books/my-book/ch01.md" {
		t.Errorf("pathToKey = %q, want %q", key, "books/my-book/ch01.md")
	}
}
