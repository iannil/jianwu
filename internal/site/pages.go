package site

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/iannil/jianwu/internal/export"
)

//go:embed site.css
var assetsFS embed.FS

// siteCSS returns the embedded site stylesheet.
func siteCSS() []byte {
	b, err := assetsFS.ReadFile("site.css")
	if err != nil {
		return []byte("")
	}
	return b
}

// chapterHref returns the site path of one chapter page.
func chapterHref(part, chapter int) string {
	return fmt.Sprintf("ch-%02d-%02d.html", part, chapter)
}

// collectRelease assembles the release's chapter docs (content/ snapshot,
// per-chapter footnote renumbering, normalized access dates).
func collectRelease(b *ShelfBook) ([]export.ChapterDoc, error) {
	return export.CollectDir(filepath.Join(b.Dir, "content"), b.Outline)
}

// htmlPage wraps body content in an HTML5 page with the site chrome.
// cssHref is the stylesheet path relative to the page's depth.
func htmlPage(title, cssHref, body string) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n")
	b.WriteString(`<html lang="zh">` + "\n<head>\n<meta charset=\"utf-8\" />\n")
	b.WriteString("<title>" + export.Esc(title) + "</title>\n")
	b.WriteString(`<link rel="stylesheet" href="` + cssHref + `" />` + "\n</head>\n<body>\n")
	b.WriteString(body)
	b.WriteString("\n</body>\n</html>\n")
	return b.String()
}

// indexPage renders the shelf catalog.
func indexPage(books []ShelfBook) string {
	var b strings.Builder
	b.WriteString(`<header class="site-head"><h1><a href="index.html">肩吾书架</a></h1>` +
		`<p class="muted">本地发布 · 可溯源的 AI 辅助图书 · <a href="opds.xml">OPDS 订阅</a></p></header>` + "\n")
	if len(books) == 0 {
		b.WriteString(`<p class="empty">还没有已发布的图书。用 <code>jianwu publish</code> 发布第一本。</p>` + "\n")
		return htmlPage("肩吾书架", "site.css", b.String())
	}
	b.WriteString("<ul class=\"catalog\">\n")
	for i := range books {
		bk := &books[i]
		st := bk.Manifest.Content
		b.WriteString(`<li><a class="book-link" href="` + export.Esc(bk.Slug) + `/index.html">` + export.Esc(bk.Meta.Title) + "</a>")
		if bk.Meta.Subtitle != "" {
			b.WriteString(`<span class="muted"> — ` + export.Esc(bk.Meta.Subtitle) + "</span>")
		}
		b.WriteString(`<div class="muted">` + export.Esc(bk.Meta.Author) +
			" · v" + export.Esc(bk.Manifest.Version) + " · " + bk.Manifest.CreatedAt.Format("2006-01-02") +
			fmt.Sprintf(" · %d 章 · %d 条未核验论断", st.ChaptersTotal, st.ClaimsUnverified) + `</div></li>` + "\n")
	}
	b.WriteString("</ul>\n")
	return htmlPage("肩吾书架", "site.css", b.String())
}

// bookPage renders the book landing page: colophon, versions, TOC, download.
func bookPage(b *ShelfBook) string {
	var bld strings.Builder
	bld.WriteString(`<header class="site-head"><h1><a href="../index.html">← 书架</a></h1></header>` + "\n")
	bld.WriteString(`<article class="book">` + "\n<h1>" + export.Esc(b.Meta.Title) + "</h1>\n")
	if b.Meta.Subtitle != "" {
		bld.WriteString(`<p class="subtitle">` + export.Esc(b.Meta.Subtitle) + "</p>\n")
	}
	bld.WriteString(`<p class="muted">` + export.Esc(b.Meta.Author) + " · v" + export.Esc(b.Manifest.Version) + "</p>\n")
	if b.Manifest.EPUB != nil {
		bld.WriteString(`<p><a class="dl" href="../epub/` + export.Esc(b.Slug) + `.epub">下载 EPUB</a> ` +
			`<span class="muted">(` + fmt.Sprint(b.Manifest.EPUB.Bytes) + " bytes · sha256 " + export.Esc(shortSHA(b.Manifest.EPUB.SHA256)) + ")</span></p>\n")
	}
	bld.WriteString(`<p class="colophon muted">许可：` + export.Esc(b.Meta.License) +
		" · 本书由 jianwu 辅助生成，全部章节经人工审阅后定稿 · 未核验论断见各章「来源与核验」</p>\n")
	if len(b.Versions) > 1 {
		bld.WriteString(`<p class="muted">历史版本：` + export.Esc(strings.Join(b.Versions[1:], "、")) + "</p>\n")
	}
	bld.WriteString("<nav class=\"toc\">\n<h2>目录</h2>\n")
	for pi := range b.Outline.Parts {
		p := &b.Outline.Parts[pi]
		bld.WriteString("<h3>" + export.Esc(p.Title) + "</h3>\n<ol>\n")
		for _, c := range p.Chapters {
			bld.WriteString(`<li><a href="` + chapterHref(p.Index, c.Index) + `">` + export.Esc(c.Title) + "</a></li>\n")
		}
		bld.WriteString("</ol>\n")
	}
	bld.WriteString("</nav>\n</article>\n")
	return htmlPage(b.Meta.Title, "../site.css", bld.String())
}

// chapterPage renders one reading page with prev/next nav.
func chapterPage(b *ShelfBook, doc export.ChapterDoc, prev, next string) string {
	var bld strings.Builder
	bld.WriteString(`<header class="site-head"><h1><a href="index.html">` + export.Esc(b.Meta.Title) + `</a></h1></header>` + "\n")
	bld.WriteString(`<article class="chapter">` + "\n<h1>" + export.Esc(doc.Title) + "</h1>\n")
	if doc.Missing {
		bld.WriteString(`<p class="empty">（本章未随本版本发布）</p>` + "\n")
	} else {
		body, err := export.RenderXHTML(doc.BodyMD)
		if err != nil {
			bld.WriteString(`<p class="empty">（本章渲染失败）</p>` + "\n")
		} else {
			bld.WriteString(body)
			bld.WriteString(export.SourcesXHTML(doc.Meta))
		}
	}
	bld.WriteString(`<nav class="pagenav">` + "\n")
	if prev != "" {
		bld.WriteString(`<a class="prev" href="` + prev + `">← 上一章</a>`)
	}
	if next != "" {
		bld.WriteString(`<a class="next" href="` + next + `">下一章 →</a>`)
	}
	bld.WriteString("\n</nav>\n</article>\n")
	return htmlPage(doc.Title+" · "+b.Meta.Title, "../site.css", bld.String())
}

// shortSHA truncates a hex digest for display.
func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
