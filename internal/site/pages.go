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
// cssHref is the stylesheet path relative to the page's depth; desc feeds
// <meta name="description"> so shared links carry a meaningful summary.
func htmlPage(title, desc, cssHref, body string) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n")
	b.WriteString(`<html lang="zh-CN">` + "\n<head>\n<meta charset=\"utf-8\" />\n")
	b.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1" />` + "\n")
	b.WriteString(`<meta name="description" content="` + export.Esc(desc) + `" />` + "\n")
	b.WriteString(`<link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 64 64'><rect width='64' height='64' rx='14' fill='%231c1c1c'/><text x='32' y='45' font-size='36' text-anchor='middle' fill='%23fafafa' font-family='serif' font-weight='600'>肩</text></svg>">` + "\n")
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
		return htmlPage("肩吾书架", "本地发布的可溯源 AI 辅助图书：书架、逐章阅读、来源核验披露与 EPUB 下载。", "site.css", b.String())
	}
	b.WriteString("<ul class=\"catalog\">\n")
	for i := range books {
		bk := &books[i]
		st := bk.Manifest.Content
		cover := ""
		if rel := bk.CoverRel(); rel != "" {
			cover = `<a href="` + export.Esc(bk.Slug) + `/index.html"><img class="shelf-cover" src="` + export.Esc(bk.Slug) + `/` + export.Esc(rel) + `" alt="《` + export.Esc(bk.Meta.Title) + `》封面" /></a>`
		}
		b.WriteString(`<li>` + cover + `<a class="book-link" href="` + export.Esc(bk.Slug) + `/index.html">` + export.Esc(bk.Meta.Title) + "</a>")
		if bk.Meta.Subtitle != "" {
			b.WriteString(`<span class="muted"> — ` + export.Esc(bk.Meta.Subtitle) + "</span>")
		}
		b.WriteString(`<div class="muted">` + export.Esc(bk.Meta.Author) +
			" · v" + export.Esc(bk.Manifest.Version) + " · " + bk.Manifest.CreatedAt.Format("2006-01-02") +
			fmt.Sprintf(" · %d 章 · %d 条未核验论断", st.ChaptersTotal, st.ClaimsUnverified) + `</div></li>` + "\n")
	}
	b.WriteString("</ul>\n")
	return htmlPage("肩吾书架", "本地发布的可溯源 AI 辅助图书：书架、逐章阅读、来源核验披露与 EPUB 下载。", "site.css", b.String())
}

// bookPage renders the book landing page: colophon, versions, TOC, download.
func bookPage(b *ShelfBook) string {
	var bld strings.Builder
	bld.WriteString(`<header class="site-head"><h1><a href="../index.html">← 书架</a></h1></header>` + "\n")
	bld.WriteString(`<article class="book">` + "\n")
	if rel := b.CoverRel(); rel != "" {
		bld.WriteString(`<img class="cover" src="` + export.Esc(rel) + `" alt="《` + export.Esc(b.Meta.Title) + `》封面" />` + "\n")
	}
	bld.WriteString("<h1>" + export.Esc(b.Meta.Title) + "</h1>\n")
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
	desc := b.Meta.Title
	if b.Meta.Subtitle != "" {
		desc += "——" + b.Meta.Subtitle
	}
	desc += "。逐章阅读，来源核验披露，提供 EPUB 下载。"
	return htmlPage(b.Meta.Title, desc, "../site.css", bld.String())
}

// chapterPage renders one reading page with prev/next nav.
func chapterPage(b *ShelfBook, doc export.ChapterDoc, prev, next string) string {
	var bld strings.Builder
	bld.WriteString(`<header class="site-head"><h1><a href="index.html">` + export.Esc(b.Meta.Title) + `</a></h1></header>` + "\n")
	bld.WriteString(`<article class="chapter">` + "\n<h1>" + export.Esc(doc.Title) + "</h1>\n")
	// 顶部章导航：读长文途中换章/回目录不必滚动到页底。
	bld.WriteString(`<nav class="chapnav" aria-label="章节导航">` + "\n")
	if prev != "" {
		bld.WriteString(`<a class="prev" href="` + prev + `">← 上一章</a>`)
	} else {
		bld.WriteString(`<span class="prev"></span>`)
	}
	bld.WriteString(`<a class="toc" href="index.html">目录</a>`)
	if next != "" {
		bld.WriteString(`<a class="next" href="` + next + `">下一章 →</a>`)
	} else {
		bld.WriteString(`<span class="next"></span>`)
	}
	bld.WriteString("\n</nav>\n")
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
	return htmlPage(doc.Title+" · "+b.Meta.Title, "《"+b.Meta.Title+"》章节："+doc.Title+"。附来源与核验披露。", "../site.css", bld.String())
}

// shortSHA truncates a hex digest for display.
func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
