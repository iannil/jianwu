package server

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/iannil/jianwu/internal/site"
)

// handleSiteGenerate rebuilds the static reading site from published
// releases; dry_run returns the shelf synchronously.
func (s *Server) handleSiteGenerate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DryRun bool `json:"dry_run"`
	}
	if err := decodeBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.requireWorkspace(w) {
		return
	}
	if body.DryRun {
		books, skipped, err := site.Scan(s.WSRoot())
		if err != nil {
			failErr(w, err)
			return
		}
		list := make([]map[string]any, 0, len(books))
		for i := range books {
			b := &books[i]
			list = append(list, map[string]any{
				"slug": b.Slug, "title": b.Meta.Title, "version": b.Manifest.Version,
				"updated":  b.Manifest.CreatedAt.Format("2006-01-02"),
				"has_epub": b.Manifest.EPUB != nil,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"dry_run": true, "books": list, "skipped": skipped})
		return
	}
	jobID := s.jobs.Start("site", "", func(ctx context.Context, j *Job) error {
		return s.runSiteGenerate(j)
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID})
}

// runSiteGenerate executes the site rebuild inside the serial job queue.
func (s *Server) runSiteGenerate(j *Job) error {
	out := filepath.Join(s.WSRoot(), "site")
	j.SetProgress(20, "扫描已发布书架")
	res, err := site.Generate(s.WSRoot(), out)
	if err != nil {
		return err
	}
	for _, sk := range res.Skipped {
		j.Logf("! skipped %s", sk)
	}
	j.SetProgress(90, "写入静态页")
	j.Logf("✓ 生成 %s（%d 本书 · %d 页 · %d 个 EPUB）", res.Dir, res.Books, res.Pages, res.EPUBs)
	j.SetResult("dir", res.Dir)
	j.SetResult("books", res.Books)
	j.SetResult("pages", res.Pages)
	return nil
}
