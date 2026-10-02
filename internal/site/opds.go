package site

import (
	"strings"

	"github.com/iannil/jianwu/internal/export"
)

// opdsEntry is one book in the acquisition feed.
type opdsEntry struct {
	Book *ShelfBook
}

// opdsFeed renders the OPDS 1.x acquisition catalog (Atom). Acquisition
// links point at the copied EPUB artifacts; books without one keep an
// alternate HTML link so the entry is still usable.
func opdsFeed(entries []opdsEntry) string {
	updated := "1970-01-01T00:00:00Z"
	for _, e := range entries {
		if u := e.Book.Manifest.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"); u > updated {
			updated = u
		}
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom" xmlns:opds="http://opds-spec.org/2010/catalog">` + "\n")
	b.WriteString("<id>urn:jianwu:catalog</id>\n")
	b.WriteString("<title>肩吾书架</title>\n")
	b.WriteString("<updated>" + updated + "</updated>\n")
	b.WriteString(`<link rel="self" href="opds.xml" type="application/atom+xml;profile=opds-catalog;kind=acquisition" />` + "\n")
	for _, e := range entries {
		bk := e.Book
		b.WriteString("<entry>\n")
		b.WriteString("<id>urn:uuid:" + export.Esc(bk.Manifest.BookID) + "</id>\n")
		b.WriteString("<title>" + export.Esc(bk.Meta.Title) + "</title>\n")
		if bk.Meta.Author != "" {
			b.WriteString("<author><name>" + export.Esc(bk.Meta.Author) + "</name></author>\n")
		}
		b.WriteString("<updated>" + bk.Manifest.CreatedAt.UTC().Format("2006-01-02T15:04:05Z") + "</updated>\n")
		if bk.Meta.Subtitle != "" {
			b.WriteString(`<summary type="text">` + export.Esc(bk.Meta.Subtitle) + "</summary>\n")
		}
		if bk.Manifest.EPUB != nil {
			b.WriteString(`<link rel="http://opds-spec.org/acquisition" href="epub/` + export.Esc(bk.Slug) +
				`.epub" type="application/epub+zip" />` + "\n")
		}
		b.WriteString(`<link rel="alternate" href="` + export.Esc(bk.Slug) + `/index.html" type="text/html" />` + "\n")
		b.WriteString("</entry>\n")
	}
	b.WriteString("</feed>\n")
	return b.String()
}
