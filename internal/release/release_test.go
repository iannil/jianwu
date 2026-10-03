package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/storage"
)

// publishableBook builds a final, licensed book on disk and returns its
// bookCtx-equivalent Input plus the book dir root.
func publishableBook(t *testing.T, slug string) (Input, string) {
	t.Helper()
	tmp := t.TempDir()
	bookDir := filepath.Join(tmp, "books", slug)
	if err := os.MkdirAll(filepath.Join(bookDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	reviewed := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	meta := &book.Meta{
		ID: "11111111-1111-1111-1111-111111111111", Slug: slug, Title: "测试书",
		Author: "作者", License: "CC BY-SA 4.0", Archetype: "ontology-epistemology-practice",
		Language: "zh", Status: book.BookStatusFinal,
	}
	outline := &book.Outline{Parts: []book.OutlinePart{{
		Index: 1, Title: "第一部分",
		Chapters: []book.OutlineChapter{
			{
				Index: 1, Title: "第一章", Status: book.StatusFinal, WordCount: 100,
				ReviewedAt: &reviewed, ReviewedBy: "reviewer",
				Claims: []book.Claim{
					{Text: "已核验论断", CitationIDs: []string{"1"}, HasCitation: true},
					{Text: "未核验论断", CitationIDs: []string{"2"}, HasCitation: true},
				},
				Citations: []book.Citation{
					{ID: "1", URL: "https://example.com/a", Title: "来源A", AccessedAt: reviewed},
					{ID: "2", URL: "https://example.com/b", Title: "来源B", AccessedAt: reviewed},
				},
				Verdicts: []book.ClaimVerdict{
					{ClaimText: "已核验论断", Verified: true, CitationID: "1"},
					{ClaimText: "未核验论断", Verified: false, CitationID: "2"},
				},
				ExpandedWith: &book.ExpandedWith{Provider: "glm", Model: "glm-4.6", Tokens: book.Tokens{In: 10, Out: 20}},
			},
		},
	}}}
	for ci := range outline.Parts[0].Chapters {
		c := outline.Parts[0].Chapters[ci]
		if _, err := book.WriteChapter(bookDir, 1, c.Index, book.ChapterFrontmatter{
			Title: c.Title, PartIndex: 1, ChapterIndex: c.Index, Status: book.StatusFinal,
		}, "正文内容"+c.Title); err != nil {
			t.Fatal(err)
		}
	}
	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"), meta); err != nil {
		t.Fatal(err)
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
		t.Fatal(err)
	}
	return Input{BookDir: bookDir, Meta: meta, Outline: outline}, bookDir
}

func TestCheckGate(t *testing.T) {
	in, _ := publishableBook(t, "demo")
	// The base book carries 2 standing warnings: 1 unverified claim, 1 failed verdict.
	tests := []struct {
		name        string
		mutate      func(*book.Meta, *book.Outline)
		wantBlocked bool
		wantWarn    int
	}{
		{name: "final book with license passes", mutate: func(m *book.Meta, o *book.Outline) {},
			wantBlocked: false, wantWarn: 2},
		{name: "draft book blocked", mutate: func(m *book.Meta, o *book.Outline) { m.Status = book.BookStatusDraft },
			wantBlocked: true, wantWarn: 2},
		{name: "missing license blocked", mutate: func(m *book.Meta, o *book.Outline) { m.License = "" },
			wantBlocked: true, wantWarn: 2},
		{name: "missing id blocked", mutate: func(m *book.Meta, o *book.Outline) { m.ID = "" },
			wantBlocked: true, wantWarn: 2},
		{name: "chapter not final blocked", mutate: func(m *book.Meta, o *book.Outline) {
			o.Parts[0].Chapters[0].Status = book.StatusExpanded
		}, wantBlocked: true, wantWarn: 2},
		{name: "missing reviewed_by only warns", mutate: func(m *book.Meta, o *book.Outline) {
			o.Parts[0].Chapters[0].ReviewedBy = ""
		}, wantBlocked: false, wantWarn: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, outline := *in.Meta, *in.Outline
			meta2, outline2 := &meta, &outline
			outline2.Parts = append([]book.OutlinePart(nil), in.Outline.Parts...)
			for i := range outline2.Parts {
				outline2.Parts[i].Chapters = append([]book.OutlineChapter(nil), in.Outline.Parts[i].Chapters...)
			}
			tt.mutate(meta2, outline2)
			rep, st := CheckGate(meta2, outline2)
			if got := !rep.OK(); got != tt.wantBlocked {
				t.Errorf("blocked = %v, want %v (blockers: %v)", got, tt.wantBlocked, rep.Blockers)
			}
			if len(rep.Warnings) != tt.wantWarn {
				t.Errorf("warnings = %d (%v), want %d", len(rep.Warnings), rep.Warnings, tt.wantWarn)
			}
			if st.ChaptersTotal != 1 || st.ClaimsTotal != 2 || st.ClaimsUnverified != 1 ||
				st.CitationsTotal != 2 || st.VerdictsFailed != 1 {
				t.Errorf("stats = %+v", st)
			}
		})
	}
}

func TestPublish_FirstReleaseWritesVerifiableManifest(t *testing.T) {
	in, bookDir := publishableBook(t, "demo")
	res, err := Publish(storage.OS, in, Options{JianwuVersion: "0.3.10-test", Now: func() time.Time {
		return time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)
	}})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if res.Version != "1.0" {
		t.Errorf("version = %q, want 1.0", res.Version)
	}
	relDir := filepath.Join(bookDir, "releases", "1.0")
	// Manifest exists and hashes match the snapshot files.
	manRaw, err := os.ReadFile(filepath.Join(relDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var man Manifest
	if err := json.Unmarshal(manRaw, &man); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if man.BookID != in.Meta.ID || man.Version != "1.0" || man.JianwuVersion != "0.3.10-test" {
		t.Errorf("manifest identity fields wrong: %+v", man)
	}
	if man.CreatedAt.UTC() != time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC) {
		t.Errorf("created_at = %v, want injected clock", man.CreatedAt)
	}
	metaRaw, _ := os.ReadFile(filepath.Join(bookDir, "meta.json"))
	if man.Content.MetaSHA256 != sha256Hex(metaRaw) {
		t.Error("meta hash mismatch")
	}
	chRaw, _ := os.ReadFile(filepath.Join(bookDir, "chapters", "01-01.md"))
	if len(man.Content.Chapters) != 1 || man.Content.Chapters[0].SHA256 != sha256Hex(chRaw) {
		t.Error("chapter entry/hash mismatch")
	}
	got, _ := os.ReadFile(filepath.Join(relDir, "content", "01-01.md"))
	if string(got) != string(chRaw) {
		t.Error("chapter copy differs from source")
	}
	// Provenance records sources, models and disclosure.
	provRaw, err := os.ReadFile(filepath.Join(relDir, "provenance.json"))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}
	var prov Provenance
	if err := json.Unmarshal(provRaw, &prov); err != nil {
		t.Fatalf("parse provenance: %v", err)
	}
	if len(prov.Sources) != 2 || len(prov.Models) != 1 || len(prov.AIDisclosure.HumanReview) != 1 {
		t.Errorf("provenance incomplete: %+v", prov)
	}
	if prov.AIDisclosure.GeneratedBy == "" {
		t.Error("missing AI disclosure")
	}
	// No staging left behind.
	if _, err := os.Stat(filepath.Join(bookDir, "releases", ".staging-1.0")); !os.IsNotExist(err) {
		t.Error("staging dir left behind")
	}
}

func TestPublish_CoverSnapshottedIntoRelease(t *testing.T) {
	in, bookDir := publishableBook(t, "covered")
	// 约定封面（书根 cover.png）必须被快照进 release 并记入 manifest。
	cover := []byte("\x89PNG-not-real-but-bytes")
	if err := os.WriteFile(filepath.Join(bookDir, "cover.png"), cover, 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Publish(storage.OS, in, Options{JianwuVersion: "test"})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	// 读回 manifest 验证。
	manRaw, err := os.ReadFile(filepath.Join(res.Dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var man Manifest
	if err := json.Unmarshal(manRaw, &man); err != nil {
		t.Fatal(err)
	}
	if man.Cover == nil {
		t.Fatalf("manifest.cover missing:\n%s", manRaw)
	}
	if man.Cover.Path != "cover.png" || man.Cover.Bytes != len(cover) {
		t.Errorf("cover info = %+v", man.Cover)
	}
	got, err := os.ReadFile(filepath.Join(res.Dir, "cover.png"))
	if err != nil {
		t.Fatalf("release cover not written: %v", err)
	}
	if string(got) != string(cover) {
		t.Errorf("cover bytes differ")
	}
}

func TestPublish_GateFailureReturnsReportAndError(t *testing.T) {
	in, _ := publishableBook(t, "demo")
	in.Meta.License = ""
	res, err := Publish(storage.OS, in, Options{})
	if err == nil {
		t.Fatal("expected gate error")
	}
	if res == nil || res.Report.OK() {
		t.Fatal("expected report with blockers on result")
	}
	if _, statErr := os.Stat(filepath.Join(in.BookDir, "releases")); !os.IsNotExist(statErr) {
		t.Error("no releases dir should exist after gate failure")
	}
}

func TestPublish_DryRunWritesNothing(t *testing.T) {
	in, bookDir := publishableBook(t, "demo")
	res, err := Publish(storage.OS, in, Options{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if res.Version != "1.0" || !res.DryRun || len(res.Files) == 0 {
		t.Errorf("dry-run result incomplete: %+v", res)
	}
	if _, statErr := os.Stat(filepath.Join(bookDir, "releases")); !os.IsNotExist(statErr) {
		t.Error("dry-run must not write")
	}
}

func TestPublish_ImmutableVersion(t *testing.T) {
	in, _ := publishableBook(t, "demo")
	if _, err := Publish(storage.OS, in, Options{}); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	// Same explicit version must be refused; auto never collides.
	if _, err := Publish(storage.OS, in, Options{Version: "1.0"}); err == nil {
		t.Fatal("expected immutability error for existing version")
	}
	if _, err := Publish(storage.OS, in, Options{}); err != nil {
		t.Fatalf("auto publish after 1.0: %v", err)
	}
}

func TestPublish_VersionBumps(t *testing.T) {
	// Each case gets a fresh book with a baseline 1.0 release.
	tests := []struct {
		name string
		opts Options
		// mutate changes book content (minor) or structure (major) after 1.0.
		mutate    func(*Input)
		wantError bool
		want      string
	}{
		{name: "content edit bumps minor", opts: Options{},
			mutate: func(in *Input) {
				c := &in.Outline.Parts[0].Chapters[0]
				c.WordCount = 200
				if _, err := book.WriteChapter(in.BookDir, 1, 1, book.ChapterFrontmatter{
					Title: c.Title, PartIndex: 1, ChapterIndex: 1, Status: book.StatusFinal,
				}, "改后的正文"); err != nil {
					t.Fatal(err)
				}
			}, want: "1.1"},
		{name: "added chapter bumps major", opts: Options{},
			mutate: func(in *Input) {
				in.Outline.Parts[0].Chapters = append(in.Outline.Parts[0].Chapters, book.OutlineChapter{
					Index: 2, Title: "第二章", Status: book.StatusFinal,
				})
				if _, err := book.WriteChapter(in.BookDir, 1, 2, book.ChapterFrontmatter{
					Title: "第二章", PartIndex: 1, ChapterIndex: 2, Status: book.StatusFinal,
				}, "新章正文"); err != nil {
					t.Fatal(err)
				}
			}, want: "2.0"},
		{name: "force major", opts: Options{ForceMajor: true}, want: "2.0"},
		{name: "explicit valid", opts: Options{Version: "3.0"}, want: "3.0"},
		{name: "explicit regression refused", opts: Options{Version: "1.0"}, wantError: true},
		{name: "explicit malformed refused", opts: Options{Version: "v1"}, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in, _ := publishableBook(t, "demo")
			if _, err := Publish(storage.OS, in, Options{}); err != nil {
				t.Fatalf("baseline publish 1.0: %v", err)
			}
			if tt.mutate != nil {
				tt.mutate(&in)
			}
			res, err := Publish(storage.OS, in, tt.opts)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got result %+v", res)
				}
				return
			}
			if err != nil {
				t.Fatalf("Publish: %v", err)
			}
			if res.Version != tt.want {
				t.Errorf("version = %q, want %q", res.Version, tt.want)
			}
		})
	}
}

func TestPublish_EPUBHookWritesArtifact(t *testing.T) {
	in, bookDir := publishableBook(t, "demo")
	epub := []byte("PK-fake-epub")
	if _, err := Publish(storage.OS, in, Options{BuildEPUB: func() ([]byte, error) { return epub, nil }}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(bookDir, "releases", "1.0", "artifact", "demo.epub"))
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if string(got) != string(epub) {
		t.Error("artifact bytes differ")
	}
	manRaw, _ := os.ReadFile(filepath.Join(bookDir, "releases", "1.0", "manifest.json"))
	var man Manifest
	_ = json.Unmarshal(manRaw, &man)
	if man.EPUB == nil || man.EPUB.SHA256 != sha256Hex(epub) || man.EPUB.Bytes != len(epub) {
		t.Errorf("manifest epub node wrong: %+v", man.EPUB)
	}
}

func TestPublish_ViaMemStorage(t *testing.T) {
	in, _ := publishableBook(t, "demo")
	st := storage.NewMemStorage()
	// Mirror the on-disk book into memory.
	paths := []string{"meta.json", "outline.json", "chapters/01-01.md"}
	for _, p := range paths {
		raw, err := os.ReadFile(filepath.Join(in.BookDir, p))
		if err != nil {
			t.Fatal(err)
		}
		if err := st.WriteFile(filepath.Join(in.BookDir, p), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := Publish(st, in, Options{})
	if err != nil {
		t.Fatalf("Publish via MemStorage: %v", err)
	}
	if _, err := st.ReadFile(filepath.Join(res.Dir, "manifest.json")); err != nil {
		t.Errorf("manifest not in MemStorage after dir rename: %v", err)
	}
}

func TestNextVersion_MissingManifestRefused(t *testing.T) {
	in, bookDir := publishableBook(t, "demo")
	relDir := filepath.Join(bookDir, "releases", "1.0")
	if err := os.MkdirAll(relDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := NextVersion(storage.OS, filepath.Join(bookDir, "releases"), in.Outline, VersionOpts{})
	if err == nil {
		t.Fatal("expected error for release dir without manifest")
	}
}
