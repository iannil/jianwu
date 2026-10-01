package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/engine/expand"
	"github.com/iannil/jianwu/internal/storage"
)

// saveExpandedChapter restores pre-write files after an ordinary I/O failure.
// A process crash between files still requires recovery from a workspace backup.
func saveExpandedChapter(out io.Writer, bc *bookCtx, cfg *config.Config, partIdx, chIdx int, result *expand.ExpandOutput, usage book.TokenUsage) error {
	return withBookRollback(bc, partIdx, chIdx, func() error {
		return saveExpandedChapterFiles(out, bc, cfg, partIdx, chIdx, result, usage)
	})
}

// withBookRollback restores the affected chapter and metadata after ordinary I/O errors.
func withBookRollback(bc *bookCtx, partIdx, chIdx int, save func() error) error {
	type backup struct {
		path   string
		data   []byte
		exists bool
	}
	var before []backup
	for _, path := range []string{book.ChapterPath(bc.BookDir, partIdx, chIdx), filepath.Join(bc.BookDir, "outline.json"), filepath.Join(bc.BookDir, "meta.json")} {
		data, err := book.DefaultStorage.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("backup %s: %w", path, err)
		}
		before = append(before, backup{path, data, err == nil})
	}
	ch, err := findChapter(bc.Outline, partIdx, chIdx)
	if err != nil {
		return err
	}
	oldChapter, oldMeta := *ch, *bc.Meta
	if err := save(); err != nil {
		*ch = oldChapter
		*bc.Meta = oldMeta
		for _, b := range before {
			current, readErr := book.DefaultStorage.ReadFile(b.path)
			if b.exists && readErr == nil && bytes.Equal(current, b.data) {
				continue
			}
			var restoreErr error
			if b.exists {
				restoreErr = storage.WriteFileAtomic(book.DefaultStorage, b.path, b.data, 0o644)
			} else {
				restoreErr = book.DefaultStorage.RemoveAll(b.path)
			}
			if restoreErr != nil {
				err = errors.Join(err, fmt.Errorf("restore %s: %w", b.path, restoreErr))
			}
		}
		return err
	}
	return nil
}
