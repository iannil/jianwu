package export

import (
	"fmt"
	"strings"
)

// navXHTML renders the EPUB3 navigation document: a toc (nested parts →
// chapters) plus landmarks.
func navXHTML(in BookInput, docs []ChapterDoc) string {
	lang := docLang(in)
	var b strings.Builder
	b.WriteString(`<nav epub:type="toc" id="toc">` + "\n<h1>目录</h1>\n<ol>\n")
	b.WriteString(`<li><a href="title.xhtml">` + Esc(in.Meta.Title) + "</a></li>\n")
	for pi := range in.Outline.Parts {
		p := &in.Outline.Parts[pi]
		b.WriteString(`<li><a href="` + partHref(p.Index) + `">` + Esc(p.Title) + "</a>\n<ol>\n")
		for _, d := range docs {
			if d.PartIndex == p.Index {
				b.WriteString(`<li><a href="` + chapterHref(d.PartIndex, d.ChapterIndex) + `">` + Esc(d.Title) + "</a></li>\n")
			}
		}
		b.WriteString("</ol></li>\n")
	}
	b.WriteString("</ol>\n</nav>\n")
	// Landmarks: reading order entry points.
	b.WriteString(`<nav epub:type="landmarks" hidden="">` + "\n<ol>\n")
	b.WriteString(`<li><a epub:type="toc" href="#toc">目录</a></li>\n`)
	if len(in.Outline.Parts) > 0 {
		b.WriteString(`<li><a epub:type="bodymatter" href="` + partHref(in.Outline.Parts[0].Index) + `">正文</a></li>\n`)
	}
	b.WriteString("</ol>\n</nav>\n")
	return xhtmlDoc("目录", lang, b.String())
}

// partHref returns the zip path of a part page.
func partHref(part int) string { return fmt.Sprintf("part-%02d.xhtml", part) }

// chapterHref returns the zip path of a chapter page.
func chapterHref(part, chapter int) string { return fmt.Sprintf("ch-%02d-%02d.xhtml", part, chapter) }

// spineItem is one manifest/spine entry.
type spineItem struct {
	ID       string
	Href     string
	Manifest string // extra manifest attributes (properties=...)
}

// contentOPF renders the package document. dcterms:modified comes from
// Meta.UpdatedAt — never the wall clock — so builds are deterministic.
func contentOPF(in BookInput, opts Options, docs []ChapterDoc) string {
	lang := docLang(in)
	author := in.Meta.Author
	if author == "" {
		author = "jianwu 生成（作者未署名）"
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(`<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid" xml:lang="` + lang + `">` + "\n")
	b.WriteString(`<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` + "\n")
	b.WriteString(`<dc:identifier id="bookid">urn:uuid:` + Esc(in.Meta.ID) + "</dc:identifier>\n")
	b.WriteString(`<dc:title id="t1">` + Esc(in.Meta.Title) + "</dc:title>\n")
	b.WriteString(`<meta refines="#t1" property="title-type">main</meta>\n`)
	if in.Meta.Subtitle != "" {
		b.WriteString(`<dc:title id="t2">` + Esc(in.Meta.Subtitle) + "</dc:title>\n")
		b.WriteString(`<meta refines="#t2" property="title-type">subtitle</meta>\n`)
	}
	b.WriteString("<dc:language>" + Esc(lang) + "</dc:language>\n")
	b.WriteString("<dc:creator>" + Esc(author) + "</dc:creator>\n")
	if in.Meta.License != "" {
		b.WriteString("<dc:rights>" + Esc(in.Meta.License) + "</dc:rights>\n")
	}
	// CCYY-MM-DDThh:mm:ssZ, required form.
	b.WriteString(`<meta property="dcterms:modified">` + in.Meta.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z") + "</meta>\n")
	b.WriteString(`<meta name="generator" content="jianwu ` + Esc(opts.JianwuVersion) + `" />\n`)
	b.WriteString("</metadata>\n<manifest>\n")

	b.WriteString(`<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav" />` + "\n")
	b.WriteString(`<item id="css" href="style.css" media-type="text/css" />` + "\n")
	if opts.CoverData != nil {
		b.WriteString(`<item id="cover-image" href="cover.` + coverExt(opts.CoverMedia) + `" media-type="` + Esc(opts.CoverMedia) + `" properties="cover-image" />` + "\n")
		b.WriteString(`<item id="coverpage" href="cover.xhtml" media-type="application/xhtml+xml" />` + "\n")
	}
	b.WriteString(`<item id="titlepage" href="title.xhtml" media-type="application/xhtml+xml" />` + "\n")

	var spine []string
	if opts.CoverData != nil {
		spine = append(spine, "coverpage")
	}
	spine = append(spine, "titlepage")
	// Every outline part gets a page (even when it has no chapters yet), so
	// the nav and spine stay consistent.
	for pi := range in.Outline.Parts {
		p := &in.Outline.Parts[pi]
		id := fmt.Sprintf("p%02d", p.Index)
		b.WriteString(`<item id="` + id + `" href="` + partHref(p.Index) + `" media-type="application/xhtml+xml" />` + "\n")
		spine = append(spine, id)
		for _, d := range docs {
			if d.PartIndex != p.Index {
				continue
			}
			cid := fmt.Sprintf("c%02d%02d", d.PartIndex, d.ChapterIndex)
			b.WriteString(`<item id="` + cid + `" href="` + chapterHref(d.PartIndex, d.ChapterIndex) + `" media-type="application/xhtml+xml" />` + "\n")
			spine = append(spine, cid)
		}
	}

	b.WriteString("</manifest>\n<spine>\n")
	for _, id := range spine {
		b.WriteString(`<itemref idref="` + id + `" linear="yes" />` + "\n")
	}
	b.WriteString("</spine>\n</package>\n")
	return b.String()
}

// coverExt maps a cover media type to its file extension.
func coverExt(media string) string {
	if media == "image/jpeg" {
		return "jpg"
	}
	return "png"
}
