package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// doSPA executes one request against the bare SPA handler (no API/auth wiring).
func doSPA(t *testing.T, path, ifNoneMatch string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, strings.NewReader(""))
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	rec := httptest.NewRecorder()
	spaHandler().ServeHTTP(rec, req)
	return rec
}

func TestSPAAssetsCarryETag(t *testing.T) {
	rec := doSPA(t, "/app.js", "")
	wantCode(t, rec, http.StatusOK)
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("app.js must carry an ETag")
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}

	// Conditional request with the matching ETag short-circuits to 304.
	rec304 := doSPA(t, "/app.js", etag)
	wantCode(t, rec304, http.StatusNotModified)
	if rec304.Body.Len() != 0 {
		t.Errorf("304 response must be empty, got %d bytes", rec304.Body.Len())
	}

	// A stale ETag still serves the full body.
	wantCode(t, doSPA(t, "/app.js", `"stale"`), http.StatusOK)

	// The ETag is content-addressed: unchanged asset → same value.
	rec2 := doSPA(t, "/app.js", "")
	if got := rec2.Header().Get("ETag"); got != etag {
		t.Errorf("etag drifted between requests: %q vs %q", got, etag)
	}
}

func TestSPAFallbackServesIndexWithETag(t *testing.T) {
	root := doSPA(t, "/", "")
	wantCode(t, root, http.StatusOK)

	// Unknown client-side routes fall back to the SPA entry point.
	deep := doSPA(t, "/some/client/route", "")
	wantCode(t, deep, http.StatusOK)
	if !strings.Contains(deep.Body.String(), "<!doctype html>") {
		t.Error("fallback must serve the SPA index page")
	}
	// Fallback serves the same content (and thus the same ETag) as "/".
	if deep.Header().Get("ETag") != root.Header().Get("ETag") {
		t.Error("fallback and root must serve identical index.html content")
	}
	wantCode(t, doSPA(t, "/some/client/route", deep.Header().Get("ETag")), http.StatusNotModified)
}
