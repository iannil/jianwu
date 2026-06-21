package reader

import (
	"context"
	"errors"
)

// Reader fetches a URL and returns clean markdown content.
type Reader interface {
	Read(ctx context.Context, url string) (Content, error)
}

// Content is the result of a Read call.
type Content struct {
	URL      string
	Title    string // extracted from page if available
	Markdown string // cleaned content
}

// ErrReader is the sentinel for reader errors.
var ErrReader = errors.New("reader error")
