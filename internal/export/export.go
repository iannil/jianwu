// Package export assembles a book into delivery formats. The first target
// is EPUB3 (ADR 29 step 2): pure Go, no external toolchain, deterministic
// output (identical book state → identical bytes) so the release layer can
// hash artifacts. The legacy md/hugo/pdf targets remain in cli/server.
package export

import (
	"embed"

	"github.com/iannil/jianwu/internal/book"
)

//go:embed style.css
var assetsFS embed.FS

// styleCSS returns the embedded reading stylesheet.
func styleCSS() []byte {
	b, err := assetsFS.ReadFile("style.css")
	if err != nil {
		return []byte("")
	}
	return b
}

// BookInput is the assembled state of one book for format builders.
type BookInput struct {
	Meta     *book.Meta
	Outline  *book.Outline
	Chapters []ChapterDoc
}
