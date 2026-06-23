package cli

import (
	"fmt"
	"os/user"
	"path/filepath"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/workspace"
)

// bookCtx is the resolved on-disk context for one book.
type bookCtx struct {
	WSRoot  string
	BookDir string
	Meta    *book.Meta
	Outline *book.Outline
}

// loadBook resolves a slug to its workspace + meta + outline.
// Shared by expand/review/finalize/export/status. Errors are *InfoError.
func loadBook(slug string) (*bookCtx, error) {
	wsRoot, err := workspace.FindWorkspace(".")
	if err != nil {
		return nil, &InfoError{Err: err, Code: ExitCodeWorkspaceNotFound}
	}
	bookDir := filepath.Join(wsRoot, "books", slug)
	meta, err := book.LoadMeta(filepath.Join(bookDir, "meta.json"))
	if err != nil {
		return nil, &InfoError{Err: fmt.Errorf("load meta for %q: %w", slug, err), Code: ExitCodeGeneric}
	}
	outline, err := book.LoadOutline(filepath.Join(bookDir, "outline.json"))
	if err != nil {
		return nil, &InfoError{Err: fmt.Errorf("load outline for %q: %w", slug, err), Code: ExitCodeGeneric}
	}
	return &bookCtx{WSRoot: wsRoot, BookDir: bookDir, Meta: meta, Outline: outline}, nil
}

// mirrorChapterStatus updates only the status field of a chapter's .md frontmatter,
// preserving the body (read-modify-write). outline.json remains the query source of truth.
func mirrorChapterStatus(bookDir string, partIdx, chIdx int, status string) error {
	path := book.ChapterPath(bookDir, partIdx, chIdx)
	fm, body, err := book.ReadChapter(path)
	if err != nil {
		return fmt.Errorf("read chapter %02d-%02d: %w", partIdx, chIdx, err)
	}
	fm.Status = status
	if _, err := book.WriteChapter(bookDir, partIdx, chIdx, *fm, body); err != nil {
		return fmt.Errorf("write chapter %02d-%02d: %w", partIdx, chIdx, err)
	}
	return nil
}

// osUsername returns the current OS username, or "" if it cannot be determined.
func osUsername() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return u.Username
}
