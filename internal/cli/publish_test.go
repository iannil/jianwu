// internal/cli/publish_test.go
package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/release"
	"github.com/spf13/cobra"
)

// writePublishableBook creates a final, licensed book ready to publish.
func writePublishableBook(t *testing.T, slug string) string {
	t.Helper()
	tmp := writeMinimalBook(t, slug)
	bookDir := filepath.Join(tmp, "books", slug)
	reviewed := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	m, err := book.LoadMeta(filepath.Join(bookDir, "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	m.ID = "22222222-2222-2222-2222-222222222222"
	m.Author = "作者"
	m.License = "CC BY-SA 4.0"
	m.Status = book.BookStatusFinal
	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"), m); err != nil {
		t.Fatal(err)
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), &book.Outline{
		Parts: []book.OutlinePart{{Index: 1, Title: "第一部分", Chapters: []book.OutlineChapter{
			{Index: 1, Title: "第一章", Status: book.StatusFinal, ReviewedAt: &reviewed, ReviewedBy: "reviewer"},
		}}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := book.WriteChapter(bookDir, 1, 1, book.ChapterFrontmatter{
		Title: "第一章", PartIndex: 1, ChapterIndex: 1, Status: book.StatusFinal,
	}, "正文"); err != nil {
		t.Fatal(err)
	}
	return tmp
}

func TestPublish_WritesRelease(t *testing.T) {
	tmp := writePublishableBook(t, "demo")
	bookDir := filepath.Join(tmp, "books", "demo")
	chdir(t, tmp)
	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})
	if err := runPublish(cmd, "demo", publishTestOptions()); err != nil {
		t.Fatalf("runPublish: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bookDir, "releases", "1.0", "manifest.json")); err != nil {
		t.Errorf("release manifest missing: %v", err)
	}
}

func TestPublish_RejectsDraftBook(t *testing.T) {
	tmp := writeMinimalBook(t, "demo") // draft, no license
	chdir(t, tmp)
	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})
	err := runPublish(cmd, "demo", publishTestOptions())
	if err == nil {
		t.Fatal("expected gate rejection for draft book without license")
	}
	if _, statErr := os.Stat(filepath.Join(tmp, "books", "demo", "releases")); !os.IsNotExist(statErr) {
		t.Error("gate failure must not create releases dir")
	}
}

func TestPublish_DryRunReportsWithoutWriting(t *testing.T) {
	tmp := writePublishableBook(t, "demo")
	bookDir := filepath.Join(tmp, "books", "demo")
	chdir(t, tmp)
	var buf strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	opts := publishTestOptions()
	opts.DryRun = true
	if err := runPublish(cmd, "demo", opts); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(buf.String(), "1.0") {
		t.Errorf("dry-run output missing next version: %q", buf.String())
	}
	if _, statErr := os.Stat(filepath.Join(bookDir, "releases")); !os.IsNotExist(statErr) {
		t.Error("dry-run must not write")
	}
}

func TestPublish_ReleaseContainsEPUBArtifact(t *testing.T) {
	tmp := writePublishableBook(t, "demo")
	bookDir := filepath.Join(tmp, "books", "demo")
	chdir(t, tmp)
	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})
	if err := runPublish(cmd, "demo", publishTestOptions()); err != nil {
		t.Fatalf("runPublish: %v", err)
	}
	// The EPUB hook is wired unconditionally now; the artifact must land in
	// the release with a matching manifest hash.
	relEPUB, err := os.ReadFile(filepath.Join(bookDir, "releases", "1.0", "artifact", "demo.epub"))
	if err != nil {
		t.Fatalf("release epub missing: %v", err)
	}
	if len(relEPUB) == 0 || string(relEPUB[:2]) != "PK" {
		t.Fatalf("artifact is not a zip: %d bytes", len(relEPUB))
	}
	manRaw, err := os.ReadFile(filepath.Join(bookDir, "releases", "1.0", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var man release.Manifest
	if err := json.Unmarshal(manRaw, &man); err != nil {
		t.Fatal(err)
	}
	if man.EPUB == nil || man.EPUB.SHA256 != fmt.Sprintf("%x", sha256.Sum256(relEPUB)) {
		t.Errorf("manifest epub hash mismatch: %+v", man.EPUB)
	}
}

func TestExport_EPUB(t *testing.T) {
	tmp := writePublishableBook(t, "demo")
	bookDir := filepath.Join(tmp, "books", "demo")
	chdir(t, tmp)
	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})
	if err := runExport(cmd, []string{"demo"}, "epub", false); err != nil {
		t.Fatalf("runExport epub: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(bookDir, "export", "demo.epub"))
	if err != nil {
		t.Fatalf("epub file missing: %v", err)
	}
	if len(data) == 0 || string(data[:2]) != "PK" {
		t.Fatalf("export is not a zip: %d bytes", len(data))
	}
	// Determinism: a second run yields identical bytes.
	if err := runExport(cmd, []string{"demo"}, "epub", false); err != nil {
		t.Fatalf("second export: %v", err)
	}
	again, _ := os.ReadFile(filepath.Join(bookDir, "export", "demo.epub"))
	if string(data) != string(again) {
		t.Error("same book state must export identical EPUB bytes")
	}
}

func publishTestOptions() release.Options {
	return release.Options{JianwuVersion: "test"}
}
