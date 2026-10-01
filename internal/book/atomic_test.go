package book

import (
	"errors"
	"os"
	"testing"

	"github.com/iannil/jianwu/internal/storage"
)

type failingStorage struct {
	storage.Storage
	failWrite, failRename bool
}

var errAtomicTest = errors.New("simulated disk failure")

func (s failingStorage) WriteFile(path string, data []byte, mode os.FileMode) error {
	if s.failWrite {
		_ = s.Storage.WriteFile(path, []byte("truncated"), mode)
		return errAtomicTest
	}
	return s.Storage.WriteFile(path, data, mode)
}
func (s failingStorage) Rename(a, b string) error {
	if s.failRename {
		return errAtomicTest
	}
	return s.Storage.Rename(a, b)
}
func TestFailedSavePreservesOriginal(t *testing.T) {
	for _, tc := range []struct {
		name          string
		write, rename bool
	}{{"write", true, false}, {"rename", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			mem := storage.NewMemStorage()
			old := DefaultStorage
			DefaultStorage = mem
			t.Cleanup(func() { DefaultStorage = old })
			if err := SaveMeta("books/meta.json", &Meta{Title: "original"}); err != nil {
				t.Fatal(err)
			}
			DefaultStorage = failingStorage{Storage: mem, failWrite: tc.write, failRename: tc.rename}
			if err := SaveMeta("books/meta.json", &Meta{Title: "replacement"}); !errors.Is(err, errAtomicTest) {
				t.Fatalf("error=%v", err)
			}
			got, err := LoadMeta("books/meta.json")
			if err != nil {
				t.Fatal(err)
			}
			if got.Title != "original" {
				t.Errorf("original lost: %s", got.Title)
			}
			entries, err := mem.ReadDir("books")
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Errorf("temporary file leaked: %d entries", len(entries))
			}
		})
	}
}
