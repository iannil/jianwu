package site

import (
	"fmt"
	"strings"

	"github.com/iannil/jianwu/internal/export"
)

// opdsEntry is one book in the acquisition feed.
type opdsEntry struct {
	Book *ShelfBook
}

// opdsFeed renders the OPDS 1.x acquisition catalog (Atom). Acquisition
// links point at the copied EPUB artifacts; books without one keep an
// alternate HTML link so the entry is still usable. baseURL, when set,
// prefixes link hrefs.
func opdsFeed(entries []opdsEntry, baseURL string) string {
	updated := "1970-01-01T00:00:00Z"
	for _, e := range entries {
		if u := e.Book.Manifest.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"); u > updated {
			updated = u
		}
	}
	base := strings.TrimSuffix(baseURL, "/")
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom" xmlns:opds="http://opds-spec.org/2010/catalog">` + "\n")
	b.WriteString("<id>urn:jianwu:catalog</id>\n")
	b.WriteString("<title>肩吾书架</title>\n")
	b.WriteString("<updated>" + updated + "</updated>\n")
	b.WriteString(`<link rel="self" href="` + base + `/opds.xml" type="application/atom+xml;profile=opds-catalog;kind=acquisition" />` + "\n")
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
			b.WriteString(`<link rel="http://opds-spec.org/acquisition" href="` + base + `/epub/` + export.Esc(bk.Slug) +
				`.epub" type="application/epub+zip" />` + "\n")
		}
		b.WriteString(`<link rel="alternate" href="` + base + `/` + export.Esc(bk.Slug) + `/index.html" type="text/html" />` + "\n")
		b.WriteString("</entry>\n")
	}
	b.WriteString("</feed>\n")
	return b.String()
}

// rssFeed renders the RSS 2.0 release feed: one item per book (latest
// release), EPUB attached as an enclosure. Feeds are for humans and agents
// alike — the item description carries the same disclosure counts as the
// catalog page.
func rssFeed(books []ShelfBook, baseURL string) string {
	base := strings.TrimSuffix(baseURL, "/")
	lastBuild := "Thu, 01 Jan 1970 00:00:00 +0000"
	for i := range books {
		if d := books[i].Manifest.CreatedAt.UTC().Format("Mon, 02 Jan 2006 15:04:05 -0700"); d > lastBuild {
			lastBuild = d
		}
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(`<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">` + "\n<channel>\n")
	b.WriteString("<title>肩吾书架</title>\n")
	b.WriteString("<link>" + base + "/index.html</link>\n")
	b.WriteString("<description>本地发布 · 可溯源的 AI 辅助图书</description>\n")
	b.WriteString("<language>zh-cn</language>\n")
	b.WriteString("<lastBuildDate>" + lastBuild + "</lastBuildDate>\n")
	b.WriteString(`<atom:link href="` + base + `/rss.xml" rel="self" type="application/rss+xml" />` + "\n")
	for i := range books {
		bk := &books[i]
		b.WriteString("<item>\n")
		b.WriteString("<title>" + export.Esc(bk.Meta.Title) + " v" + export.Esc(bk.Manifest.Version) + "</title>\n")
		b.WriteString("<link>" + base + "/" + export.Esc(bk.Slug) + "/index.html</link>\n")
		b.WriteString("<guid>urn:uuid:" + export.Esc(bk.Manifest.BookID) + "-" + export.Esc(bk.Manifest.Version) + "</guid>\n")
		b.WriteString("<pubDate>" + bk.Manifest.CreatedAt.UTC().Format("Mon, 02 Jan 2006 15:04:05 -0700") + "</pubDate>\n")
		desc := bk.Meta.Subtitle
		if desc != "" {
			desc += "；"
		}
		desc += fmt.Sprintf("%d 章 · %d 条未核验论断 · 许可 %s",
			bk.Manifest.Content.ChaptersTotal, bk.Manifest.Content.ClaimsUnverified, bk.Meta.License)
		b.WriteString("<description>" + export.Esc(desc) + "</description>\n")
		if bk.Manifest.EPUB != nil {
			b.WriteString(`<enclosure url="` + base + "/epub/" + export.Esc(bk.Slug) +
				`.epub" length="` + fmt.Sprint(bk.Manifest.EPUB.Bytes) + `" type="application/epub+zip" />` + "\n")
		}
		b.WriteString("</item>\n")
	}
	b.WriteString("</channel>\n</rss>\n")
	return b.String()
}
