package cli

import (
	"fmt"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

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
	wsRoot, err := workspace.FindWorkspace(findWorkspacePath())
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

// findChapter returns a pointer to the chapter at (partIdx, chIdx), or error.
// Index-based iteration so the returned pointer references outline.Parts[i].Chapters[j]
// directly (mutations persist when outline is saved).
func findChapter(outline *book.Outline, partIdx, chIdx int) (*book.OutlineChapter, error) {
	for i := range outline.Parts {
		if outline.Parts[i].Index == partIdx {
			for j := range outline.Parts[i].Chapters {
				if outline.Parts[i].Chapters[j].Index == chIdx {
					return &outline.Parts[i].Chapters[j], nil
				}
			}
			return nil, fmt.Errorf("chapter %d not found in part %d", chIdx, partIdx)
		}
	}
	return nil, fmt.Errorf("part %d not found", partIdx)
}

// findPart returns the part at partIdx, or a zero value if missing.
func findPart(outline *book.Outline, partIdx int) book.OutlinePart {
	for _, p := range outline.Parts {
		if p.Index == partIdx {
			return p
		}
	}
	return book.OutlinePart{}
}

// parseChapterAddr parses a "NN-MM" string into (partIdx, chIdx), both 1-based.
// Accepts zero-padded ("01-01") and bare ("1-1") forms.
// Returns error if format is wrong or any index is 0.
func parseChapterAddr(s string) (int, int, error) {
	if s == "" {
		return 0, 0, fmt.Errorf("empty chapter address")
	}
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid chapter address %q: want NN-MM", s)
	}
	partIdx, err := strconv.Atoi(parts[0])
	if err != nil || partIdx < 1 {
		return 0, 0, fmt.Errorf("invalid part index %q", parts[0])
	}
	chIdx, err := strconv.Atoi(parts[1])
	if err != nil || chIdx < 1 {
		return 0, 0, fmt.Errorf("invalid chapter index %q", parts[1])
	}
	return partIdx, chIdx, nil
}
