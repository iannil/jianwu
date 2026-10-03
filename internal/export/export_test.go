package export

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iannil/jianwu/internal/book"
)

func TestRenderXHTMLFootnotes(t *testing.T) {
	out, err := RenderXHTML("一句[^1]与again[^1]。<img src=\"x\"> raw\n\n[^1]: [来源](https://example.com/a)\n")
	if err != nil {
		t.Fatal(err)
	}
	// First reference is fnref-1; the second occurrence gets a suffix.
	if !strings.Contains(out, `id="fnref-1"`) || !strings.Contains(out, `id="fnref-1x1"`) {
		t.Errorf("unique fnref ids missing:\n%s", out)
	}
	if !strings.Contains(out, `href="#fn-1"`) || !strings.Contains(out, `id="fn-1"`) {
		t.Errorf("noteref ↔ aside link broken:\n%s", out)
	}
	if !strings.Contains(out, `epub:type="footnote"`) || !strings.Contains(out, `epub:type="footnotes"`) {
		t.Errorf("EPUB3 footnote semantics missing:\n%s", out)
	}
	if !strings.Contains(out, `role="doc-noteref"`) || !strings.Contains(out, `role="doc-footnote"`) {
		t.Errorf("ARIA roles missing:\n%s", out)
	}
	// Raw HTML must not pass through unescaped (well-formed XML requirement).
	if strings.Contains(out, "<img") {
		t.Errorf("raw HTML leaked:\n%s", out)
	}
}

func TestRenderXHTMLEscapedAngleBrackets(t *testing.T) {
	out, err := RenderXHTML("a < b && c > d")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "a < b") {
		t.Errorf("unescaped '<' in output:\n%s", out)
	}
}

func TestClaimStatus(t *testing.T) {
	verdicts := []book.ClaimVerdict{
		{ClaimText: "支持", Verified: true, CitationID: "1"},
		{ClaimText: "失败", Verified: false, CitationID: "2", Reasoning: "来源不支持"},
	}
	tests := []struct {
		name string
		in   book.Claim
		want ClaimStatus
	}{
		{name: "supported by matching verdict", in: book.Claim{Text: "支持", CitationIDs: []string{"1"}},
			want: StatusSupported},
		{name: "failed verdict", in: book.Claim{Text: "失败", CitationIDs: []string{"2"}},
			want: StatusFailed},
		{name: "no verdicts at all", in: book.Claim{Text: "新论断", CitationIDs: []string{"3"}},
			want: StatusUnverified},
		{name: "verdict text mismatch falls back to citation id", in: book.Claim{Text: "改写过的论断", CitationIDs: []string{"1"}},
			want: StatusSupported},
		{name: "uncited claim", in: book.Claim{Text: "无引用论断"},
			want: StatusUncited},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := claimStatus(tt.in, verdicts); got != tt.want {
				t.Errorf("claimStatus = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSourcesXHTMLFourStates(t *testing.T) {
	c := book.OutlineChapter{
		Citations: []book.Citation{
			{ID: "1", URL: "https://example.com/a", Title: "来源A", AccessedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		},
		Claims: []book.Claim{
			{Text: "支持<论断>", CitationIDs: []string{"1"}},
			{Text: "失败", CitationIDs: []string{"1"}},
			{Text: "未核验", CitationIDs: []string{"1"}},
			{Text: "无引用"},
		},
		Verdicts: []book.ClaimVerdict{
			{ClaimText: "支持<论断>", Verified: true},
			{ClaimText: "失败", Verified: false, Reasoning: "来源仅部分支持"},
		},
	}
	out := SourcesXHTML(c)
	for _, want := range []string{
		"✓ 来源支持", "✗ 未通过", "未核验", "无引用", "来源A", "访问于 2026-10-01",
		// 涉及来源列：论断与来源编号可对读；无引用论断显示 —。
		`<th>论断</th><th>涉及来源</th><th>状态</th><th>说明</th>`,
		`<td class="cite-ids">[1]</td>`,
		`<td class="cite-ids">—</td>`,
		// 说明列 summary 为 reasoning 首行摘要，不再是重复的"核验说明"链接。
		"<summary>来源仅部分支持</summary>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in sources section:\n%s", want, out)
		}
	}
	// Claim text with angle brackets must be escaped.
	if strings.Contains(out, "支持<论断>") {
		t.Errorf("unescaped claim text:\n%s", out)
	}
}

func TestStripLeadingTitle(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		title string
		want  string
	}{
		{"h2 匹配剥离", "## 第一章\n正文", "第一章", "正文"},
		{"h1 匹配剥离", "# 第一章\n正文", "第一章", "正文"},
		{"闭合式标题剥离", "## 第一章 ##\n正文", "第一章", "正文"},
		{"前后空白容忍", "##  第一章  \n正文", "第一章", "正文"},
		{"单行正文剥离", "## 第一章", "第一章", ""},
		{"标题后空行一并吞掉", "## 第一章\n\n正文", "第一章", "正文"},
		{"标题不同保留原样", "## 别的标题\n正文", "第一章", "## 别的标题\n正文"},
		{"非标题首行保留", "正文开头", "第一章", "正文开头"},
		{"无章节标题保留", "## 第一章\n正文", "", "## 第一章\n正文"},
	}
	for _, tt := range tests {
		if got := StripLeadingTitle(tt.body, tt.title); got != tt.want {
			t.Errorf("%s: StripLeadingTitle(%q, %q) = %q, want %q", tt.name, tt.body, tt.title, got, tt.want)
		}
	}
}

func TestCollectDirStripsDuplicateTitle(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "01-01.md"),
		[]byte("---\ntitle: 第一章\n---\n\n## 第一章\n\n正文内容。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outline := &book.Outline{Parts: []book.OutlinePart{{
		Index: 1, Title: "P",
		Chapters: []book.OutlineChapter{{Index: 1, Title: "第一章"}},
	}}}
	docs, err := CollectDir(dir, outline)
	if err != nil {
		t.Fatalf("CollectDir: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("docs = %d, want 1", len(docs))
	}
	if strings.Contains(docs[0].BodyMD, "第一章") {
		t.Errorf("body still carries the duplicate title heading:\n%q", docs[0].BodyMD)
	}
	if !strings.Contains(docs[0].BodyMD, "正文内容。") {
		t.Errorf("body content lost:\n%q", docs[0].BodyMD)
	}
}

func TestCitationIDsLabel(t *testing.T) {
	tests := []struct {
		name string
		ids  []string
		want string
	}{
		{"数值感知排序", []string{"10", "2", "1"}, "[1] [2] [10]"},
		{"空为无引用符", nil, "—"},
		{"非数值字典序", []string{"b", "a"}, "[a] [b]"},
	}
	for _, tt := range tests {
		if got := citationIDsLabel(tt.ids); got != tt.want {
			t.Errorf("%s: citationIDsLabel(%v) = %q, want %q", tt.name, tt.ids, got, tt.want)
		}
	}
}

func TestReasoningSummary(t *testing.T) {
	if got := reasoningSummary("首行结论。\n第二行细节可以很长很长很长很长。"); got != "首行结论。" {
		t.Errorf("first line: got %q", got)
	}
	long := strings.Repeat("长", 50)
	got := reasoningSummary(long)
	if got != strings.Repeat("长", 40)+"…" {
		t.Errorf("truncation: got %q", got)
	}
}

func TestSourcesXHTMLSortsCitations(t *testing.T) {
	c := book.OutlineChapter{Citations: []book.Citation{
		{ID: "3", Title: "C3"}, {ID: "10", Title: "C10"}, {ID: "4", Title: "C4"},
		{ID: "6", Title: "C6"}, {ID: "1", Title: "C1"}, {ID: "2", Title: "C2"},
	}}
	out := SourcesXHTML(c)
	prev := -1
	for _, id := range []string{"1", "2", "3", "4", "6", "10"} {
		idx := strings.Index(out, "["+id+"] ")
		if idx < 0 {
			t.Fatalf("citation [%s] missing in:\n%s", id, out)
		}
		if idx < prev {
			t.Errorf("citation [%s] out of numeric order in:\n%s", id, out)
		}
		prev = idx
	}
}

// sampleInput builds a small two-part book input with one missing chapter.
func sampleInput() BookInput {
	meta := &book.Meta{
		ID: "44444444-4444-4444-4444-444444444444", Slug: "demo", Title: "测试书",
		Subtitle: "副题", Author: "作者", License: "CC BY-SA 4.0", Language: "zh",
		Status:    book.BookStatusFinal,
		UpdatedAt: time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC),
	}
	outline := &book.Outline{Parts: []book.OutlinePart{
		{Index: 1, Title: "第一部", Intro: "介绍", Chapters: []book.OutlineChapter{
			{Index: 1, Title: "第一章", Status: book.StatusFinal,
				Claims:    []book.Claim{{Text: "论断一", CitationIDs: []string{"1"}}},
				Citations: []book.Citation{{ID: "1", URL: "https://example.com/a", Title: "来源A"}},
				Verdicts:  []book.ClaimVerdict{{ClaimText: "论断一", Verified: true}},
			},
		}},
		{Index: 2, Title: "第二部", Chapters: []book.OutlineChapter{
			{Index: 1, Title: "第二章", Status: book.StatusFinal},
		}},
	}}
	docs := []ChapterDoc{
		{PartIndex: 1, ChapterIndex: 1, PartTitle: "第一部", Title: "第一章",
			BodyMD: "正文。[^1]\n\n[^1]: [来源A](https://example.com/a)\n",
			Meta:   outline.Parts[0].Chapters[0]},
		{PartIndex: 2, ChapterIndex: 1, PartTitle: "第二部", Title: "第二章",
			Missing: true, Meta: outline.Parts[1].Chapters[0]},
	}
	return BookInput{Meta: meta, Outline: outline, Chapters: docs}
}

func unzip(t *testing.T, data []byte) *zip.Reader {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	return zr
}

func fileOf(t *testing.T, zr *zip.Reader, name string) string {
	t.Helper()
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %s: %v", name, err)
			}
			defer rc.Close()
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(rc); err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			return buf.String()
		}
	}
	t.Fatalf("zip entry %s not found", name)
	return ""
}

func TestBuildEPUBStructureAndWellFormedness(t *testing.T) {
	in := sampleInput()
	data, err := BuildEPUB(in, Options{JianwuVersion: "test"})
	if err != nil {
		t.Fatalf("BuildEPUB: %v", err)
	}
	zr := unzip(t, data)

	// mimetype: first entry, stored uncompressed.
	if len(zr.File) == 0 || zr.File[0].Name != "mimetype" {
		t.Fatalf("first entry = %q, want mimetype", zr.File[0].Name)
	}
	if zr.File[0].Method != zip.Store {
		t.Errorf("mimetype method = %d, want Store", zr.File[0].Method)
	}
	if got := fileOf(t, zr, "mimetype"); got != "application/epub+zip" {
		t.Errorf("mimetype content = %q", got)
	}

	// Every XML document is well-formed.
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".xhtml") && !strings.HasSuffix(f.Name, ".opf") &&
			!strings.HasSuffix(f.Name, ".xml") {
			continue
		}
		raw := fileOf(t, zr, f.Name)
		if err := xml.Unmarshal([]byte(raw), new(any)); err != nil {
			t.Errorf("%s is not well-formed XML: %v", f.Name, err)
		}
	}

	// OPF identity fields.
	opf := fileOf(t, zr, "OEBPS/content.opf")
	for _, want := range []string{
		"urn:uuid:44444444-4444-4444-4444-444444444444",
		"<dc:language>zh</dc:language>",
		"<dc:creator>作者</dc:creator>",
		"<dc:rights>CC BY-SA 4.0</dc:rights>",
		`<meta property="dcterms:modified">2026-10-02T10:00:00Z</meta>`,
		`properties="nav"`,
	} {
		if !strings.Contains(opf, want) {
			t.Errorf("opf missing %q:\n%s", want, opf)
		}
	}

	// Spine hrefs exist in the zip; nav links point at existing entries.
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	for _, want := range []string{
		"OEBPS/nav.xhtml", "OEBPS/style.css", "OEBPS/title.xhtml",
		"OEBPS/part-01.xhtml", "OEBPS/part-02.xhtml",
		"OEBPS/ch-01-01.xhtml", "OEBPS/ch-02-01.xhtml",
	} {
		if !names[want] {
			t.Errorf("zip entry %s missing", want)
		}
	}

	// Chapter page carries footnotes + sources + disclosure.
	ch := fileOf(t, zr, "OEBPS/ch-01-01.xhtml")
	for _, want := range []string{`epub:type="noteref"`, "来源与核验", "✓ 来源支持", "来源A"} {
		if !strings.Contains(ch, want) {
			t.Errorf("chapter page missing %q", want)
		}
	}
	missing := fileOf(t, zr, "OEBPS/ch-02-01.xhtml")
	if !strings.Contains(missing, "（本章尚未展开）") {
		t.Errorf("missing chapter placeholder absent:\n%s", missing)
	}

	// Title page disclosure.
	title := fileOf(t, zr, "OEBPS/title.xhtml")
	if !strings.Contains(title, "辅助生成") || !strings.Contains(title, "CC BY-SA 4.0") {
		t.Errorf("title page disclosure missing:\n%s", title)
	}
}

func TestBuildEPUBDeterministic(t *testing.T) {
	in := sampleInput()
	a, err := BuildEPUB(in, Options{JianwuVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildEPUB(in, Options{JianwuVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Error("identical input must produce identical EPUB bytes")
	}
}

func TestBuildEPUBWithCover(t *testing.T) {
	in := sampleInput()
	data, err := BuildEPUB(in, Options{JianwuVersion: "test", CoverData: []byte("png"), CoverMedia: "image/png"})
	if err != nil {
		t.Fatal(err)
	}
	zr := unzip(t, data)
	opf := fileOf(t, zr, "OEBPS/content.opf")
	if !strings.Contains(opf, `properties="cover-image"`) || !strings.Contains(opf, "cover.png") {
		t.Errorf("cover not wired into opf:\n%s", opf)
	}
	if fileOf(t, zr, "OEBPS/cover.xhtml") == "" {
		t.Error("cover page missing")
	}
}

func TestBuildEPUBRequiresChaptersAndParts(t *testing.T) {
	in := sampleInput()
	in.Chapters = nil
	if _, err := BuildEPUB(in, Options{}); err == nil {
		t.Error("expected error for nil chapters")
	}
	empty := BookInput{Meta: in.Meta, Outline: &book.Outline{}, Chapters: []ChapterDoc{}}
	if _, err := BuildEPUB(empty, Options{}); err == nil {
		t.Error("expected error for book without parts")
	}
}
