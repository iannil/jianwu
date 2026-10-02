package server

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestSiteGenerateDryRunAndJob(t *testing.T) {
	srv, _ := newTestEnv(t)
	writePublishableFixture(t, srv.WSRoot(), "life")
	// The site reads releases only — publish first.
	rec, out := do(t, srv, "POST", "/api/v1/books/life/publish", map[string]any{})
	wantCode(t, rec, http.StatusAccepted)
	if job := waitJob(t, srv, out["job_id"].(string)); job.Status != JobSucceeded {
		t.Fatalf("publish job = %s (%v)", job.Status, job.Err)
	}

	// dry_run reports the shelf synchronously.
	rec, out = do(t, srv, "POST", "/api/v1/site/generate", map[string]any{"dry_run": true})
	wantCode(t, rec, http.StatusOK)
	books := out["books"].([]any)
	if len(books) != 1 || books[0].(map[string]any)["slug"] != "life" {
		t.Fatalf("dry-run books = %v", out["books"])
	}

	// Real generation runs as a job and writes the site.
	rec, out = do(t, srv, "POST", "/api/v1/site/generate", map[string]any{})
	wantCode(t, rec, http.StatusAccepted)
	job := waitJob(t, srv, out["job_id"].(string))
	if job.Status != JobSucceeded {
		t.Fatalf("site job = %s (%v)", job.Status, job.Err)
	}
	siteDir := filepath.Join(srv.WSRoot(), "site")
	if _, err := os.Stat(filepath.Join(siteDir, "index.html")); err != nil {
		t.Errorf("site index missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(siteDir, "opds.xml")); err != nil {
		t.Errorf("opds missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(siteDir, "epub", "life.epub")); err != nil {
		t.Errorf("epub download missing: %v", err)
	}
}
