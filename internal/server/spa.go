package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed web
var webFS embed.FS

// spaHandler serves the embedded single-page UI; unknown paths fall back to
// index.html so client-side routing works on refresh.
func spaHandler() http.Handler {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err) // embed layout is fixed at build time
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err != nil {
			// Not a real file — serve the SPA entry point.
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

// handleJobsList returns all jobs, newest first.
func (s *Server) handleJobsList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"jobs": s.jobs.List()})
}

// handleJobGet returns one job's view.
func (s *Server) handleJobGet(w http.ResponseWriter, r *http.Request) {
	job, ok := s.jobs.Get(r.PathValue("id"))
	if !ok {
		failf(w, http.StatusNotFound, "任务 %q 不存在", r.PathValue("id"))
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// handleJobCancel cancels a running job.
func (s *Server) handleJobCancel(w http.ResponseWriter, r *http.Request) {
	if !s.jobs.Cancel(r.PathValue("id")) {
		failf(w, http.StatusConflict, "任务不存在或已结束")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
