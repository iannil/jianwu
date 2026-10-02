package export

import (
	"fmt"
	"strings"

	"github.com/iannil/jianwu/internal/book"
)

// docLang picks the content language with zh fallback.
func docLang(in BookInput) string {
	if in.Meta != nil && in.Meta.Language != "" {
		return in.Meta.Language
	}
	return "zh"
}

// xhtmlDoc wraps body content in a full XHTML document.
func xhtmlDoc(title, lang, body string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString("<!DOCTYPE html>\n")
	b.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="` + lang + `" lang="` + lang + `">` + "\n")
	b.WriteString("<head>\n<title>" + Esc(title) + "</title>\n")
	b.WriteString(`<link rel="stylesheet" type="text/css" href="style.css" />` + "\n</head>\n<body>\n")
	b.WriteString(body)
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

// titleXHTML renders the title page with the AI disclosure colophon.
func titleXHTML(in BookInput, version string) string {
	lang := docLang(in)
	var b strings.Builder
	b.WriteString(`<section class="titlepage" epub:type="titlepage">` + "\n")
	b.WriteString(`<h1 class="book-title">` + Esc(in.Meta.Title) + "</h1>\n")
	if in.Meta.Subtitle != "" {
		b.WriteString(`<p class="book-subtitle">` + Esc(in.Meta.Subtitle) + "</p>\n")
	}
	author := in.Meta.Author
	if author == "" {
		author = "jianwu 生成（作者未署名）"
	}
	b.WriteString(`<p class="book-author">` + Esc(author) + "</p>\n")
	var col strings.Builder
	if in.Meta.License != "" {
		col.WriteString("许可：" + Esc(in.Meta.License) + "<br />")
	}
	col.WriteString("本书由 jianwu " + Esc(version) + " 辅助生成，全部章节经人工审阅后定稿。")
	b.WriteString(`<p class="colophon">` + col.String() + "</p>\n</section>\n")
	return xhtmlDoc(in.Meta.Title, lang, b.String())
}

// partXHTML renders a part divider page.
func partXHTML(in BookInput, p book.OutlinePart) string {
	lang := docLang(in)
	var b strings.Builder
	b.WriteString(`<section class="partpage" epub:type="part">` + "\n")
	b.WriteString(`<h1 class="part-title">` + Esc(p.Title) + "</h1>\n")
	if p.Intro != "" {
		b.WriteString(`<div class="part-intro">` + Esc(p.Intro) + "</div>\n")
	}
	b.WriteString("</section>\n")
	return xhtmlDoc(p.Title, lang, b.String())
}

// chapterXHTML renders one chapter: title, body, footnotes, sources.
func chapterXHTML(in BookInput, doc ChapterDoc) (string, error) {
	lang := docLang(in)
	var b strings.Builder
	b.WriteString(`<section class="chapter" epub:type="chapter">` + "\n")
	b.WriteString(`<h1 class="ch-title">` + Esc(doc.Title) + "</h1>\n")
	if doc.Missing {
		b.WriteString(`<p class="missing">（本章尚未展开）</p>\n`)
	} else {
		body, err := RenderXHTML(doc.BodyMD)
		if err != nil {
			return "", fmt.Errorf("chapter %02d-%02d: %w", doc.PartIndex, doc.ChapterIndex, err)
		}
		b.WriteString(body)
		b.WriteString(SourcesXHTML(doc.Meta))
	}
	b.WriteString("</section>\n")
	return xhtmlDoc(doc.Title, lang, b.String()), nil
}

// coverXHTML renders the cover page wrapping the cover image.
func coverXHTML(in BookInput, imgName string) string {
	var b strings.Builder
	b.WriteString(`<section class="coverpage" epub:type="cover">` + "\n")
	b.WriteString(`<img src="` + Esc(imgName) + `" alt="封面" />\n</section>\n`)
	return xhtmlDoc(in.Meta.Title, docLang(in), b.String())
}

// containerXML is the fixed OCF container pointing at the package doc.
const containerXML = `<?xml version="1.0" encoding="utf-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`
