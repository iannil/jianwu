package site

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/release"
	"github.com/iannil/jianwu/internal/storage"
)

// publishFixture creates a final licensed book and publishes it via the real
// release pipeline. withEPUB controls whether the artifact hook is wired.
func publishFixture(t *testing.T, wsRoot, slug string, withEPUB bool) {
	t.Helper()
	bookDir := filepath.Join(wsRoot, "books", slug)
	if err := os.MkdirAll(filepath.Join(bookDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	reviewed := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	meta := &book.Meta{
		ID: "id-" + slug, Slug: slug, Title: "书名" + slug, Subtitle: "副题" + slug,
		Author: "作者", License: "CC BY-SA 4.0", Archetype: "a", Language: "zh",
		Status: book.BookStatusFinal, UpdatedAt: time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC),
	}
	outline := &book.Outline{Parts: []book.OutlinePart{{
		Index: 1, Title: "第一部分", Intro: "介绍",
		Chapters: []book.OutlineChapter{
			{Index: 1, Title: "第一章", Status: book.StatusFinal, WordCount: 50,
				ReviewedAt: &reviewed, ReviewedBy: "reviewer",
				Claims:    []book.Claim{{Text: "已核验", CitationIDs: []string{"1"}}},
				Citations: []book.Citation{{ID: "1", URL: "https://example.com/a", Title: "来源A", AccessedAt: reviewed}},
				Verdicts:  []book.ClaimVerdict{{ClaimText: "已核验", Verified: true, CitationID: "1", Reasoning: "直接支持"}},
			},
			{Index: 2, Title: "第二章", Status: book.StatusFinal, WordCount: 40, ReviewedAt: &reviewed, ReviewedBy: "reviewer"},
		},
	}}}
	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"), meta); err != nil {
		t.Fatal(err)
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		idx   int
		title string
	}{
		{1, "第一章"}, {2, "第二章"},
	} {
		// 正文以 "## 标题" 开头，与引擎草稿的真实输出一致（site 必须剥掉这个重复标题）。
		if _, err := book.WriteChapter(bookDir, 1, c.idx, book.ChapterFrontmatter{
			Title: c.title, PartIndex: 1, ChapterIndex: c.idx, Status: book.StatusFinal,
		}, "## "+c.title+"\n\n正文内容"+c.title+"。[^1]\n\n[^1]: [来源A](https://example.com/a)\n"); err != nil {
			t.Fatal(err)
		}
	}
	opts := release.Options{JianwuVersion: "test"}
	if withEPUB {
		opts.BuildEPUB = func() ([]byte, error) { return []byte("PK-epub-" + slug), nil }
	}
	// 约定封面：所有 fixture 书带 cover.png，验证封面随 release 快照进入阅读站。
	if err := os.WriteFile(filepath.Join(bookDir, "cover.png"), []byte("PNG-cover-"+slug), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := release.Publish(storage.OS, release.Input{BookDir: bookDir, Meta: meta, Outline: outline}, opts)
	if err != nil {
		t.Fatalf("publish fixture %s: %v", slug, err)
	}
	if res.Version != "1.0" {
		t.Fatalf("fixture version = %s", res.Version)
	}
}

func TestScan_ShelfIsPublishedBooksOnly(t *testing.T) {
	ws := t.TempDir()
	publishFixture(t, ws, "alpha", true)
	// A working draft never reaches the shelf.
	draftDir := filepath.Join(ws, "books", "drafty")
	if err := os.MkdirAll(draftDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = book.SaveMeta(filepath.Join(draftDir, "meta.json"), &book.Meta{Slug: "drafty", Status: book.BookStatusDraft})

	books, skipped, err := Scan(ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 || books[0].Slug != "alpha" {
		t.Fatalf("books = %+v, want only alpha", books)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %v", skipped)
	}
	if books[0].Manifest.EPUB == nil {
		t.Error("fixture release should carry an epub artifact")
	}
}

func TestGenerate_ProducesCompleteSite(t *testing.T) {
	ws := t.TempDir()
	publishFixture(t, ws, "alpha", true)
	publishFixture(t, ws, "beta", false) // no artifact
	out := filepath.Join(ws, "site")

	res, err := Generate(ws, out)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if res.Books != 2 || res.EPUBs != 1 {
		t.Errorf("result = %+v", res)
	}

	read := func(rel string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(raw)
	}

	// Catalog lists both books, unverified-claim counts included.
	idx := read("index.html")
	for _, want := range []string{"书名alpha", "书名beta", "OPDS"} {
		if !strings.Contains(idx, want) {
			t.Errorf("index missing %q", want)
		}
	}
	// 书架缩略封面与书页大图均来自 release 快照。
	if !strings.Contains(idx, `<img class="shelf-cover" src="alpha/cover.png"`) {
		t.Error("index missing shelf cover thumbnail")
	}
	if _, err := os.Stat(filepath.Join(out, "alpha", "cover.png")); err != nil {
		t.Errorf("cover.png not shipped next to book page: %v", err)
	}
	// Root page css path must not escape the site dir.
	if !strings.Contains(idx, `href="site.css"`) {
		t.Error("index stylesheet path must be site.css (not ../site.css)")
	}

	// Book page: colophon + epub download only for alpha.
	alphaPage := read("alpha/index.html")
	for _, want := range []string{"书名alpha", "CC BY-SA 4.0", "下载 EPUB", "第一章", `<img class="cover" src="cover.png"`} {
		if !strings.Contains(alphaPage, want) {
			t.Errorf("alpha book page missing %q", want)
		}
	}
	// 原始字符串里的 "\n" 会成为字面反斜杠 n 渲染进页面——任何页面都不得出现。
	for _, page := range []string{"index.html", "alpha/index.html", "alpha/ch-01-01.html"} {
		if html := read(page); strings.Contains(html, `\n`) {
			t.Errorf("%s contains a literal backslash-n", page)
		}
	}
	if strings.Contains(read("beta/index.html"), "下载 EPUB") {
		t.Error("beta has no artifact; download link must be absent")
	}

	// Chapter page: rendered body, footnote aside, sources section, nav.
	ch := read("alpha/ch-01-01.html")
	for _, want := range []string{
		"正文内容第一章", `epub:type="noteref"`, "来源与核验", "✓ 来源支持",
		`href="ch-01-02.html"`, "下一章",
	} {
		if !strings.Contains(ch, want) {
			t.Errorf("chapter page missing %q", want)
		}
	}
	last := read("alpha/ch-01-02.html")
	if strings.Contains(last, "下一章") {
		t.Error("last chapter must not have a next link")
	}

	// EPUB copied byte-for-byte.
	epub := read("epub/alpha.epub")
	if epub != "PK-epub-alpha" {
		t.Errorf("epub copy = %q", epub)
	}

	// OPDS feed parses and its acquisition link resolves.
	feed := read("opds.xml")
	if err := xml.Unmarshal([]byte(feed), new(any)); err != nil {
		t.Fatalf("opds not well-formed: %v\n%s", err, feed)
	}
	if !strings.Contains(feed, `rel="http://opds-spec.org/acquisition"`) ||
		!strings.Contains(feed, "书名alpha") || !strings.Contains(feed, "urn:uuid:id-alpha") {
		t.Errorf("opds feed incomplete:\n%s", feed)
	}
	if !strings.Contains(feed, "书名beta") || !strings.Contains(feed, `rel="alternate" href="/beta/index.html"`) {
		t.Errorf("artifact-less book must still be listed with an alternate link:\n%s", feed)
	}

	// RSS feed parses, carries the disclosure and the EPUB enclosure.
	rss := read("rss.xml")
	if err := xml.Unmarshal([]byte(rss), new(any)); err != nil {
		t.Fatalf("rss not well-formed: %v\n%s", err, rss)
	}
	for _, want := range []string{
		"<title>书名alpha v1.0</title>",
		`<enclosure url="/epub/alpha.epub" length="13" type="application/epub+zip" />`,
		"未核验论断",
		"<guid>urn:uuid:id-alpha-1.0</guid>",
	} {
		if !strings.Contains(rss, want) {
			t.Errorf("rss missing %q:\n%s", want, rss)
		}
	}
	if !strings.Contains(rss, "<pubDate>") {
		t.Error("rss items must carry pubDate")
	}
}

// TestGenerate_PageHeadAndFeedMetadata pins the page chrome (viewport, favicon,
// language) and the RSS channel metadata added for readers/validators.
func TestGenerate_PageHeadAndFeedMetadata(t *testing.T) {
	ws := t.TempDir()
	publishFixture(t, ws, "alpha", true)
	out := filepath.Join(ws, "site")
	if _, err := Generate(ws, out); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	read := func(rel string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(raw)
	}

	// Every generated page shares htmlPage: check one shallow and one deep page.
	for _, page := range []string{"index.html", "alpha/ch-01-01.html"} {
		html := read(page)
		for _, want := range []string{
			`<html lang="zh-CN">`,
			`<meta name="viewport" content="width=device-width, initial-scale=1" />`,
			`rel="icon"`,
		} {
			if !strings.Contains(html, want) {
				t.Errorf("%s missing %q", page, want)
			}
		}
	}

	rss := read("rss.xml")
	for _, want := range []string{
		"<language>zh-cn</language>",
		"<lastBuildDate>",
		`<atom:link href="/rss.xml" rel="self" type="application/rss+xml" />`,
	} {
		if !strings.Contains(rss, want) {
			t.Errorf("rss missing %q:\n%s", want, rss)
		}
	}
	if err := xml.Unmarshal([]byte(rss), new(any)); err != nil {
		t.Fatalf("rss not well-formed: %v\n%s", err, rss)
	}
}

// 章节阅读页：重复标题被剥离、顶部章导航与 meta description 存在。
func TestGenerate_ChapterPageReadingAids(t *testing.T) {
	ws := t.TempDir()
	publishFixture(t, ws, "alpha", true)
	out := filepath.Join(ws, "site")
	if _, err := Generate(ws, out); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	read := func(rel string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(raw)
	}

	first := read("alpha/ch-01-01.html")
	// 模板渲染 <h1>第一章</h1>；正文自带的 "## 第一章" 必须被剥掉，不得再出现 <h2>。
	if !strings.Contains(first, "<h1>第一章</h1>") {
		t.Errorf("chapter page missing template h1:\n%s", first)
	}
	if strings.Contains(first, "<h2>第一章</h2>") {
		t.Errorf("chapter page carries the duplicate body heading:\n%s", first)
	}
	// 首章：prev 为空占位、目录直达、next 指向下一章。
	for _, want := range []string{
		`<nav class="chapnav" aria-label="章节导航">`,
		`<span class="prev"></span>`,
		`<a class="toc" href="index.html">目录</a>`,
		`<a class="next" href="ch-01-02.html">`,
		`<meta name="description" content="《书名alpha》章节：第一章。附来源与核验披露。" />`,
	} {
		if !strings.Contains(first, want) {
			t.Errorf("first chapter page missing %q", want)
		}
	}

	last := read("alpha/ch-01-02.html")
	if !strings.Contains(last, `<a class="prev" href="ch-01-01.html">`) {
		t.Errorf("second chapter page missing prev link")
	}
	if !strings.Contains(last, `<span class="next"></span>`) {
		t.Errorf("last chapter page missing next placeholder")
	}
}

func TestGenerateAt_BaseURLMakesAbsoluteLinks(t *testing.T) {
	ws := t.TempDir()
	publishFixture(t, ws, "alpha", true)
	out := filepath.Join(ws, "site")
	if _, err := GenerateAt(ws, out, "https://books.example.com/"); err != nil {
		t.Fatal(err)
	}
	rss, _ := os.ReadFile(filepath.Join(out, "rss.xml"))
	if !strings.Contains(string(rss), `url="https://books.example.com/epub/alpha.epub"`) {
		t.Errorf("rss enclosure not absolute:\n%s", rss)
	}
	feed, _ := os.ReadFile(filepath.Join(out, "opds.xml"))
	if !strings.Contains(string(feed), `href="https://books.example.com/epub/alpha.epub"`) {
		t.Errorf("opds acquisition not absolute:\n%s", feed)
	}
}

func TestGenerate_DeterministicAndSkipsBrokenReleases(t *testing.T) {
	ws := t.TempDir()
	publishFixture(t, ws, "alpha", true)
	// A corrupted release dir (no manifest) must be skipped, not fatal.
	if err := os.MkdirAll(filepath.Join(ws, "books", "broken", "releases", "9.9"), 0o755); err != nil {
		t.Fatal(err)
	}
	out1 := filepath.Join(ws, "site1")
	res, err := Generate(ws, out1)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0], "broken") {
		t.Errorf("skipped = %v, want broken@9.9", res.Skipped)
	}
	if res.Books != 1 {
		t.Errorf("books = %d, want 1", res.Books)
	}

	// Determinism: regenerating elsewhere yields identical files.
	out2 := filepath.Join(ws, "site2")
	if _, err := Generate(ws, out2); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"index.html", "opds.xml", "rss.xml", "alpha/index.html", "alpha/ch-01-01.html", "site.css"} {
		a, _ := os.ReadFile(filepath.Join(out1, rel))
		b, _ := os.ReadFile(filepath.Join(out2, rel))
		if string(a) != string(b) {
			t.Errorf("%s differs between generations", rel)
		}
	}
}

func TestGenerate_EmptyWorkspace(t *testing.T) {
	ws := t.TempDir()
	out := filepath.Join(ws, "site")
	res, err := Generate(ws, out)
	if err != nil {
		t.Fatalf("Generate on empty workspace: %v", err)
	}
	if res.Books != 0 {
		t.Errorf("books = %d, want 0", res.Books)
	}
	idx, _ := os.ReadFile(filepath.Join(out, "index.html"))
	if !strings.Contains(string(idx), "还没有已发布的图书") {
		t.Errorf("empty catalog page wrong:\n%s", idx)
	}
}
