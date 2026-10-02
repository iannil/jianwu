package server

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iannil/jianwu/internal/book"
)

// writePublishableFixture upgrades createBookFixture output to a final,
// licensed, publishable book with chapter files on disk.
func writePublishableFixture(t *testing.T, root, slug string) string {
	t.Helper()
	bookDir := createBookFixture(t, root, slug, 2)
	reviewed := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	m, err := book.LoadMeta(filepath.Join(bookDir, "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	m.Author = "作者"
	m.License = "CC BY-SA 4.0"
	m.Status = book.BookStatusFinal
	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"), m); err != nil {
		t.Fatal(err)
	}
	o, err := book.LoadOutline(filepath.Join(bookDir, "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	for pi := range o.Parts {
		for ci := range o.Parts[pi].Chapters {
			c := o.Parts[pi].Chapters[ci]
			o.Parts[pi].Chapters[ci].Status = book.StatusFinal
			o.Parts[pi].Chapters[ci].ReviewedAt = &reviewed
			o.Parts[pi].Chapters[ci].ReviewedBy = "reviewer"
			if _, err := book.WriteChapter(bookDir, o.Parts[pi].Index, c.Index, book.ChapterFrontmatter{
				Title: c.Title, PartIndex: o.Parts[pi].Index, ChapterIndex: c.Index, Status: book.StatusFinal,
			}, "正文"); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), o); err != nil {
		t.Fatal(err)
	}
	return bookDir
}

func TestPublishDryRunReportsGateAndVersion(t *testing.T) {
	srv, _ := newTestEnv(t)
	writePublishableFixture(t, srv.WSRoot(), "life")

	rec, out := do(t, srv, "POST", "/api/v1/books/life/publish", map[string]any{"dry_run": true})
	wantCode(t, rec, http.StatusOK)
	if out["version"] != "1.0" {
		t.Errorf("version = %v, want 1.0", out["version"])
	}
	if _, err := os.Stat(filepath.Join(srv.WSRoot(), "books", "life", "releases")); !os.IsNotExist(err) {
		t.Error("dry-run must not write")
	}
}

func TestPublishJobWritesReleaseAndLists(t *testing.T) {
	srv, _ := newTestEnv(t)
	bookDir := writePublishableFixture(t, srv.WSRoot(), "life")

	rec, out := do(t, srv, "POST", "/api/v1/books/life/publish", map[string]any{})
	wantCode(t, rec, http.StatusAccepted)
	job := waitJob(t, srv, out["job_id"].(string))
	if job.Status != JobSucceeded {
		t.Fatalf("job status = %s, want succeeded (err: %v)", job.Status, job.Err)
	}
	if _, err := os.Stat(filepath.Join(bookDir, "releases", "1.0", "manifest.json")); err != nil {
		t.Fatalf("release manifest missing: %v", err)
	}

	rec, listOut := do(t, srv, "GET", "/api/v1/books/life/releases", nil)
	wantCode(t, rec, http.StatusOK)
	releases, ok := listOut["releases"].([]any)
	if !ok || len(releases) != 1 {
		t.Fatalf("releases = %v, want 1 entry", listOut["releases"])
	}
	first := releases[0].(map[string]any)
	if first["version"] != "1.0" || first["chapters"] != float64(2) {
		t.Errorf("release summary wrong: %v", first)
	}
}

func TestPublishJobGateFailureVisible(t *testing.T) {
	srv, _ := newTestEnv(t)
	// Draft book without license: gate must fail inside the job, visibly.
	createBookFixture(t, srv.WSRoot(), "life", 1)

	rec, out := do(t, srv, "POST", "/api/v1/books/life/publish", map[string]any{})
	wantCode(t, rec, http.StatusAccepted)
	job := waitJob(t, srv, out["job_id"].(string))
	if job.Status != JobFailed {
		t.Fatalf("job status = %s, want failed", job.Status)
	}
	if job.Err == "" {
		t.Error("failed job must carry the gate error")
	}
}

func TestPublishReleaseCarriesEPUBAndExportEPUBWorks(t *testing.T) {
	srv, _ := newTestEnv(t)
	bookDir := writePublishableFixture(t, srv.WSRoot(), "life")

	// export epub → job succeeds, file is a zip.
	rec, out := do(t, srv, "POST", "/api/v1/books/life/export", map[string]any{"target": "epub"})
	wantCode(t, rec, http.StatusAccepted)
	job := waitJob(t, srv, out["job_id"].(string))
	if job.Status != JobSucceeded {
		t.Fatalf("export epub job = %s (%v)", job.Status, job.Err)
	}
	expData, err := os.ReadFile(filepath.Join(bookDir, "export", "life.epub"))
	if err != nil || len(expData) < 2 || string(expData[:2]) != "PK" {
		t.Fatalf("epub export invalid: %v %d bytes", err, len(expData))
	}

	// publish → release artifact equals the deterministic export bytes.
	rec, out = do(t, srv, "POST", "/api/v1/books/life/publish", map[string]any{})
	wantCode(t, rec, http.StatusAccepted)
	job = waitJob(t, srv, out["job_id"].(string))
	if job.Status != JobSucceeded {
		t.Fatalf("publish job = %s (%v)", job.Status, job.Err)
	}
	relData, err := os.ReadFile(filepath.Join(bookDir, "releases", "1.0", "artifact", "life.epub"))
	if err != nil {
		t.Fatalf("release epub missing: %v", err)
	}
	if !bytes.Equal(expData, relData) {
		t.Error("release artifact must equal deterministic export output")
	}
	_, listOut := do(t, srv, "GET", "/api/v1/books/life/releases", nil)
	releases := listOut["releases"].([]any)
	if releases[0].(map[string]any)["has_epub"] != true {
		t.Errorf("release listing must report has_epub")
	}
}
