package release

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/storage"
)

// Input carries the loaded book state for one publish run. Callers (cli /
// serve) assemble it with their bookCtx; Publish reads chapter and state files
// from BookDir via st but never writes book state.
type Input struct {
	BookDir string
	Meta    *book.Meta
	Outline *book.Outline
}

// Options configures one publish run.
type Options struct {
	// Version is an explicit "MAJOR.MINOR"; empty means derive.
	Version string
	// ForceMajor bumps major regardless of structural comparison.
	ForceMajor bool
	// DryRun reports gate + version + planned files without writing.
	DryRun bool
	// JianwuVersion stamps manifest and provenance.
	JianwuVersion string
	// BuildEPUB optionally produces the artifact bytes; wired by the caller
	// when the EPUB slice exists. release does not import export.
	BuildEPUB func() ([]byte, error)
	// Now is the release clock; defaults to time.Now().UTC().
	Now func() time.Time
}

// Result reports the outcome of a publish run. On gate failure both a Result
// (with the report) and an error are returned so callers can show why.
type Result struct {
	Version string     `json:"version"`
	Dir     string     `json:"dir,omitempty"`
	DryRun  bool       `json:"dry_run"`
	Report  GateReport `json:"report"`
	Stats   Stats      `json:"stats"`
	Files   []string   `json:"files,omitempty"`
}

// Publish checks the ADR 29 gate and writes an immutable release directory:
// staging under releases/.staging-<version>/ first, then renamed into place.
// Book state is read-only; a failed run removes its staging directory and
// leaves previous releases untouched.
func Publish(st storage.Storage, in Input, opts Options) (*Result, error) {
	rep, stats := CheckGate(in.Meta, in.Outline)
	releasesDir := filepath.Join(in.BookDir, "releases")
	version, err := NextVersion(st, releasesDir, in.Outline, VersionOpts{Explicit: opts.Version, ForceMajor: opts.ForceMajor})
	if err != nil {
		return nil, err
	}
	res := &Result{Version: version, DryRun: opts.DryRun, Report: rep, Stats: stats}
	if !rep.OK() {
		return res, fmt.Errorf("发布门未通过（%d 项）：%s", len(rep.Blockers), strings.Join(rep.Blockers, "；"))
	}
	if opts.DryRun {
		res.Files = plannedFiles(in, opts)
		return res, nil
	}

	// State snapshots: hash the on-disk bytes, not re-marshalled objects.
	metaRaw, err := st.ReadFile(filepath.Join(in.BookDir, "meta.json"))
	if err != nil {
		return res, fmt.Errorf("read meta.json: %w", err)
	}
	outlineRaw, err := st.ReadFile(filepath.Join(in.BookDir, "outline.json"))
	if err != nil {
		return res, fmt.Errorf("read outline.json: %w", err)
	}

	man := &Manifest{
		Schema:        1,
		BookID:        in.Meta.ID,
		Slug:          in.Meta.Slug,
		Version:       version,
		CreatedAt:     nowUTC(opts),
		JianwuVersion: opts.JianwuVersion,
		Content: ManifestContent{
			OutlineSHA256:    sha256Hex(outlineRaw),
			MetaSHA256:       sha256Hex(metaRaw),
			ChaptersTotal:    stats.ChaptersTotal,
			ClaimsTotal:      stats.ClaimsTotal,
			ClaimsUnverified: stats.ClaimsUnverified,
			CitationsTotal:   stats.CitationsTotal,
			VerdictsFailed:   stats.VerdictsFailed,
		},
	}

	type fileWrite struct {
		rel  string
		data []byte
	}
	writes := []fileWrite{
		{"outline.json", outlineRaw},
		{"meta.json", metaRaw},
	}
	for pi := range in.Outline.Parts {
		part := &in.Outline.Parts[pi]
		for ci := range part.Chapters {
			c := &part.Chapters[ci]
			data, err := st.ReadFile(book.ChapterPath(in.BookDir, part.Index, c.Index))
			if err != nil {
				return res, fmt.Errorf("read chapter %02d-%02d: %w", part.Index, c.Index, err)
			}
			rel := fmt.Sprintf("content/%02d-%02d.md", part.Index, c.Index)
			writes = append(writes, fileWrite{rel, data})
			man.Content.Chapters = append(man.Content.Chapters, ChapterEntry{
				Part: part.Index, Chapter: c.Index, Title: c.Title, Path: rel,
				SHA256: sha256Hex(data), WordCount: c.WordCount, Status: c.Status,
			})
		}
	}

	if opts.BuildEPUB != nil {
		epub, err := opts.BuildEPUB()
		if err != nil {
			return res, fmt.Errorf("build epub artifact: %w", err)
		}
		rel := "artifact/" + in.Meta.Slug + ".epub"
		writes = append(writes, fileWrite{rel, epub})
		man.EPUB = &ArtifactInfo{Path: rel, SHA256: sha256Hex(epub), Bytes: len(epub)}
	}

	manJSON, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return res, fmt.Errorf("marshal manifest: %w", err)
	}
	writes = append(writes, fileWrite{"manifest.json", manJSON})
	provJSON, err := json.MarshalIndent(buildProvenance(in, opts), "", "  ")
	if err != nil {
		return res, fmt.Errorf("marshal provenance: %w", err)
	}
	writes = append(writes, fileWrite{"provenance.json", provJSON})

	staging := filepath.Join(releasesDir, ".staging-"+version)
	final := filepath.Join(releasesDir, version)
	_ = st.RemoveAll(staging)
	for _, w := range writes {
		dst := filepath.Join(staging, filepath.FromSlash(w.rel))
		if err := st.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			_ = st.RemoveAll(staging)
			return res, fmt.Errorf("mkdir %s: %w", filepath.Dir(w.rel), err)
		}
		if err := st.WriteFile(dst, w.data, 0o644); err != nil {
			_ = st.RemoveAll(staging)
			return res, fmt.Errorf("write %s: %w", w.rel, err)
		}
		res.Files = append(res.Files, w.rel)
	}
	if err := st.Rename(staging, final); err != nil {
		_ = st.RemoveAll(staging)
		return res, fmt.Errorf("commit release %s: %w", version, err)
	}
	res.Dir = final
	return res, nil
}

// plannedFiles lists the files a real run would write (for --dry-run).
func plannedFiles(in Input, opts Options) []string {
	files := []string{"manifest.json", "provenance.json", "outline.json", "meta.json"}
	for pi := range in.Outline.Parts {
		part := &in.Outline.Parts[pi]
		for ci := range part.Chapters {
			files = append(files, fmt.Sprintf("content/%02d-%02d.md", part.Index, part.Chapters[ci].Index))
		}
	}
	if opts.BuildEPUB != nil {
		files = append(files, "artifact/"+in.Meta.Slug+".epub")
	}
	return files
}

// nowUTC returns the release timestamp from the injected clock.
func nowUTC(opts Options) time.Time {
	if opts.Now != nil {
		return opts.Now().UTC()
	}
	return time.Now().UTC()
}
