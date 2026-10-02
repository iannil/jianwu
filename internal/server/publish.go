package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/iannil/jianwu/internal/release"
	"github.com/iannil/jianwu/internal/storage"
)

// handlePublish validates a publish request; with dry_run it reports the
// gate + next version synchronously, otherwise it starts a publish job.
func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Version string `json:"version"`
		Major   bool   `json:"major"`
		DryRun  bool   `json:"dry_run"`
	}
	if err := decodeBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.requireWorkspace(w) {
		return
	}
	slug := r.PathValue("slug")
	bc, err := s.loadBook(slug)
	if err != nil {
		failErr(w, err)
		return
	}
	if body.DryRun {
		rep, stats := release.CheckGate(bc.Meta, bc.Outline)
		version, err := release.NextVersion(storage.OS, filepath.Join(bc.BookDir, "releases"), bc.Outline,
			release.VersionOpts{Explicit: body.Version, ForceMajor: body.Major})
		if err != nil {
			failf(w, http.StatusConflict, "%s", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"dry_run": true, "version": version, "gate": rep, "stats": stats,
		})
		return
	}
	version, major := body.Version, body.Major
	jobID := s.jobs.Start("publish", slug, func(ctx context.Context, j *Job) error {
		return s.runPublishJob(j, slug, version, major)
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID})
}

// runPublishJob executes one publish inside the serial job queue.
func (s *Server) runPublishJob(j *Job, slug, version string, major bool) error {
	bc, err := s.loadBook(slug)
	if err != nil {
		return err
	}
	j.SetProgress(20, "检查发布门")
	// The EPUB artifact rides along since the export slice exists; release
	// stays decoupled via the injected hook (ADR 29).
	opts := release.Options{Version: version, ForceMajor: major, JianwuVersion: s.version}
	opts.BuildEPUB = func() ([]byte, error) { return s.buildEPUBArtifact(bc) }
	res, perr := release.Publish(storage.OS, release.Input{BookDir: bc.BookDir, Meta: bc.Meta, Outline: bc.Outline}, opts)
	if res != nil {
		for _, b := range res.Report.Blockers {
			j.Logf("✗ %s", b)
		}
		for _, warn := range res.Report.Warnings {
			j.Logf("! %s", warn)
		}
	}
	if perr != nil {
		return perr
	}
	j.SetProgress(90, "写入 release")
	j.Logf("✓ 已发布 %s → %s（%d 个文件）", res.Version, res.Dir, len(res.Files))
	j.SetResult("version", res.Version)
	j.SetResult("dir", res.Dir)
	j.SetResult("files", len(res.Files))
	j.SetResult("claims_unverified", res.Stats.ClaimsUnverified)
	return nil
}

// releaseView summarizes one published version for the releases listing.
type releaseView struct {
	Version          string `json:"version"`
	CreatedAt        string `json:"created_at"`
	Chapters         int    `json:"chapters"`
	ClaimsUnverified int    `json:"claims_unverified"`
	HasEPUB          bool   `json:"has_epub"`
}

// handleReleases lists published versions (latest first). The releases
// directory itself is the inventory; missing dir means no releases.
func (s *Server) handleReleases(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	slug := r.PathValue("slug")
	bc, err := s.loadBook(slug)
	if err != nil {
		failErr(w, err)
		return
	}
	releasesDir := filepath.Join(bc.BookDir, "releases")
	entries, err := storage.OS.ReadDir(releasesDir)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]any{"releases": []releaseView{}})
			return
		}
		failErr(w, fmt.Errorf("list releases: %w", err))
		return
	}
	var views []releaseView
	for _, e := range entries {
		if _, _, ok := release.ParseVersion(e.Name()); !ok {
			continue
		}
		raw, err := storage.OS.ReadFile(filepath.Join(releasesDir, e.Name(), "manifest.json"))
		if err != nil {
			continue // unreadable manifest listed as absent; publish refuses to rebuild it
		}
		var m release.Manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		views = append(views, releaseView{
			Version:          m.Version,
			CreatedAt:        m.CreatedAt.Format("2006-01-02 15:04"),
			Chapters:         m.Content.ChaptersTotal,
			ClaimsUnverified: m.Content.ClaimsUnverified,
			HasEPUB:          m.EPUB != nil,
		})
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Version > views[j].Version })
	writeJSON(w, http.StatusOK, map[string]any{"releases": views})
}
