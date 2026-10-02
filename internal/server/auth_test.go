package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// doAuth executes one request with an Authorization header.
func doAuth(t *testing.T, srv *Server, method, path, auth string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(""))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestTokenAuth(t *testing.T) {
	srv, _ := newTestEnv(t)
	srv.SetToken("s3cret")

	// No token → 401 with challenge.
	rec := doAuth(t, srv, "GET", "/api/v1/workspace", "")
	wantCode(t, rec, http.StatusUnauthorized)
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("expected WWW-Authenticate challenge")
	}

	// Wrong token → 401.
	wantCode(t, doAuth(t, srv, "GET", "/api/v1/workspace", "Bearer wrong"), http.StatusUnauthorized)

	// Correct token → 200.
	wantCode(t, doAuth(t, srv, "GET", "/api/v1/workspace", "Bearer s3cret"), http.StatusOK)

	// SPA assets stay open in token mode.
	if rec := doAuth(t, srv, "GET", "/", ""); rec.Code == http.StatusUnauthorized {
		t.Error("SPA must not be gated by the API token")
	}

	// No token set → open (default localhost trust model).
	srv2, _ := newTestEnv(t)
	wantCode(t, doAuth(t, srv2, "GET", "/api/v1/workspace", ""), http.StatusOK)
}
