package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"archive/zip"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/export"
	"github.com/iannil/jianwu/internal/storage"
)

// handleFinalize transitions all reviewed chapters to final (sync; mirrors
// cli.runFinalize). dry_run validates and reports blockers without writing.
func (s *Server) handleFinalize(w http.ResponseWriter, r *http.Request) {
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
	slug := r.PathValue("slug")
	bc, err := s.loadBook(slug)
	if err != nil {
		failErr(w, err)
		return
	}
	if bc.Meta.Status == book.BookStatusFinal {
		failf(w, http.StatusConflict, "图书 %q 已是 final", slug)
		return
	}
	var total int
	var blockers []string
	for pi := range bc.Outline.Parts {
		for ci := range bc.Outline.Parts[pi].Chapters {
			total++
			c := bc.Outline.Parts[pi].Chapters[ci]
			if c.Status != book.StatusReviewed {
				blockers = append(blockers, fmt.Sprintf("%02d-%02d %q（%s）",
					bc.Outline.Parts[pi].Index, c.Index, c.Title, c.Status))
			}
		}
	}
	if total == 0 {
		failf(w, http.StatusConflict, "图书 %q 没有可定稿的章节", slug)
		return
	}
	if len(blockers) > 0 {
		failf(w, http.StatusConflict, "无法定稿：%d 个章节未 review：%s", len(blockers), strings.Join(blockers, "；"))
		return
	}
	if body.DryRun {
		writeJSON(w, http.StatusOK, map[string]any{"dry_run": true, "chapters": total})
		return
	}
	for pi := range bc.Outline.Parts {
		for ci := range bc.Outline.Parts[pi].Chapters {
			bc.Outline.Parts[pi].Chapters[ci].Status = book.StatusFinal
			if err := mirrorChapterStatus(bc.BookDir, bc.Outline.Parts[pi].Index, bc.Outline.Parts[pi].Chapters[ci].Index, book.StatusFinal); err != nil {
				failErr(w, err)
				return
			}
		}
	}
	bc.Meta.Status = book.BookStatusFinal
	bc.Meta.UpdatedAt = time.Now().UTC()
	if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
		failErr(w, err)
		return
	}
	if err := book.SaveMeta(filepath.Join(bc.BookDir, "meta.json"), bc.Meta); err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "chapters": total})
}

// handleExport starts an export job (md/hugo/pdf) or, with dry_run, reports
// the plan synchronously.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Target string `json:"target"`
		DryRun bool   `json:"dry_run"`
	}
	if err := decodeBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Target == "" {
		body.Target = "md"
	}
	switch body.Target {
	case "md", "hugo", "pdf", "epub":
	default:
		failf(w, http.StatusBadRequest, "不支持的导出目标 %q；支持 md、hugo、pdf、epub", body.Target)
		return
	}
	if !s.requireWorkspace(w) {
		return
	}
	slug := r.PathValue("slug")
	if _, err := s.loadBook(slug); err != nil {
		failErr(w, err)
		return
	}
	if body.DryRun {
		bc, err := s.loadBook(slug)
		if err != nil {
			failErr(w, err)
			return
		}
		present, missing := countExportableChapters(bc)
		writeJSON(w, http.StatusOK, map[string]any{"dry_run": true, "chapters": present, "missing": missing})
		return
	}
	target := body.Target
	jobID := s.jobs.Start("export", slug, func(ctx context.Context, j *Job) error {
		return s.runExport(ctx, j, slug, target)
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID})
}

// handleExportFile serves the most recent export artifact for download.
func (s *Server) handleExportFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	slug := r.PathValue("slug")
	target := r.URL.Query().Get("target")
	if target == "" {
		target = "md"
	}
	bc, err := s.loadBook(slug)
	if err != nil {
		failErr(w, err)
		return
	}
	var path, name string
	switch target {
	case "md":
		path = filepath.Join(bc.BookDir, "export", bc.Meta.Slug+".md")
		name = bc.Meta.Slug + ".md"
	case "pdf":
		path = filepath.Join(bc.BookDir, "export", bc.Meta.Slug+".pdf")
		name = bc.Meta.Slug + ".pdf"
	case "epub":
		path = filepath.Join(bc.BookDir, "export", bc.Meta.Slug+".epub")
		name = bc.Meta.Slug + ".epub"
	case "hugo":
		// Hugo export is a directory; zip it for download.
		dir := filepath.Join(bc.BookDir, "export", "hugo")
		if _, err := os.Stat(dir); err != nil {
			failf(w, http.StatusNotFound, "Hugo 导出不存在；请先执行导出")
			return
		}
		zipPath := filepath.Join(bc.BookDir, "export", bc.Meta.Slug+"-hugo.zip")
		if err := zipDir(dir, zipPath); err != nil {
			failErr(w, err)
			return
		}
		path, name = zipPath, bc.Meta.Slug+"-hugo.zip"
	default:
		failf(w, http.StatusBadRequest, "不支持的导出目标 %q", target)
		return
	}
	data, err := storage.OS.ReadFile(path)
	if err != nil {
		failf(w, http.StatusNotFound, "导出文件不存在；请先执行导出")
		return
	}
	ctype := "text/markdown; charset=utf-8"
	if strings.HasSuffix(name, ".pdf") {
		ctype = "application/pdf"
	} else if strings.HasSuffix(name, ".epub") {
		ctype = "application/epub+zip"
	} else if strings.HasSuffix(name, ".zip") {
		ctype = "application/zip"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// countExportableChapters counts present vs missing chapter files.
func countExportableChapters(bc *bookCtx) (present, missing int) {
	for _, p := range bc.Outline.Parts {
		for _, c := range p.Chapters {
			if _, err := os.Stat(book.ChapterPath(bc.BookDir, p.Index, c.Index)); err == nil {
				present++
			} else {
				missing++
			}
		}
	}
	return present, missing
}

// runExport mirrors cli.runExport for one target.
func (s *Server) runExport(_ context.Context, j *Job, slug, target string) error {
	bc, err := s.loadBook(slug)
	if err != nil {
		return err
	}
	j.SetProgress(10, "组装章节")
	switch target {
	case "md":
		return s.exportMD(j, bc)
	case "hugo":
		return s.exportHugo(j, bc)
	case "pdf":
		return s.exportPDF(j, bc)
	case "epub":
		return s.exportEPUB(j, bc)
	}
	return fmt.Errorf("不支持的导出目标 %q", target)
}

// buildEPUBArtifact assembles deterministic EPUB 3 bytes; shared by the
// export job and the publish release artifact.
func (s *Server) buildEPUBArtifact(bc *bookCtx) ([]byte, error) {
	docs, err := export.Collect(bc.BookDir, bc.Outline)
	if err != nil {
		return nil, err
	}
	cover, media := export.FindCover(bc.BookDir)
	return export.BuildEPUB(export.BookInput{Meta: bc.Meta, Outline: bc.Outline, Chapters: docs},
		export.Options{JianwuVersion: s.version, CoverData: cover, CoverMedia: media})
}

// exportEPUB writes export/<slug>.epub (EPUB 3, no external toolchain).
func (s *Server) exportEPUB(j *Job, bc *bookCtx) error {
	j.SetProgress(40, "渲染 XHTML 与脚注")
	data, err := s.buildEPUBArtifact(bc)
	if err != nil {
		return err
	}
	outPath := filepath.Join(bc.BookDir, "export", bc.Meta.Slug+".epub")
	if err := storage.OS.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("mkdir export dir: %w", err)
	}
	if err := storage.OS.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("write epub: %w", err)
	}
	j.Logf("✓ 导出 %s（含来源核验节）", outPath)
	j.SetResult("file", outPath)
	return nil
}

// exportMD writes a single markdown file with Pandoc YAML frontmatter
// (mirror of cli.exportMD).
func (s *Server) exportMD(j *Job, bc *bookCtx) error {
	slug := bc.Meta.Slug
	var b strings.Builder
	fmt.Fprintf(&b, "---\n")
	fmt.Fprintf(&b, "title: \"%s\"\n", bc.Meta.Title)
	if bc.Meta.Subtitle != "" {
		fmt.Fprintf(&b, "subtitle: \"%s\"\n", bc.Meta.Subtitle)
	}
	fmt.Fprintf(&b, "language: %s\n", bc.Meta.Language)
	fmt.Fprintf(&b, "author: \"Generated by jianwu %s\"\n", s.version)
	fmt.Fprintf(&b, "date: \"%s\"\n", bc.Meta.CreatedAt.Format("2006-01-02"))
	fmt.Fprintf(&b, "status: \"%s\"\n", bc.Meta.Status)
	fmt.Fprintf(&b, "archetype: \"%s\"\n", bc.Meta.Archetype)
	fmt.Fprintf(&b, "---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", bc.Meta.Title)
	if bc.Meta.Subtitle != "" {
		fmt.Fprintf(&b, "*%s*\n\n", bc.Meta.Subtitle)
	}

	counter := 1
	present, missing := 0, 0
	for pi := range bc.Outline.Parts {
		p := bc.Outline.Parts[pi]
		fmt.Fprintf(&b, "## %s\n\n", p.Title)
		if p.Intro != "" {
			fmt.Fprintf(&b, "%s\n\n", p.Intro)
		}
		for ci := range p.Chapters {
			c := p.Chapters[ci]
			fmt.Fprintf(&b, "### %s\n\n", c.Title)
			_, bodyMD, rerr := book.ReadChapter(book.ChapterPath(bc.BookDir, p.Index, c.Index))
			if rerr != nil {
				if os.IsNotExist(rerr) {
					b.WriteString("> （本章尚未展开）\n\n")
					missing++
					continue
				}
				return fmt.Errorf("read chapter %02d-%02d: %w", p.Index, c.Index, rerr)
			}
			renumbered, next := book.RenumberFootnotes(bodyMD, counter)
			counter = next
			b.WriteString(renumbered)
			b.WriteString("\n\n")
			present++
		}
	}
	outPath := filepath.Join(bc.BookDir, "export", slug+".md")
	if err := storage.OS.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("mkdir export dir: %w", err)
	}
	if err := storage.OS.WriteFile(outPath, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write export: %w", err)
	}
	j.Logf("✓ 导出 %s（%d 章，%d 缺失）", outPath, present, missing)
	j.SetResult("file", outPath)
	j.SetResult("chapters", present)
	return nil
}

// exportHugo writes a chapter-per-file Hugo content structure.
func (s *Server) exportHugo(j *Job, bc *bookCtx) error {
	baseDir := filepath.Join(bc.BookDir, "export", "hugo", "content")
	if err := storage.OS.MkdirAll(baseDir, 0o755); err != nil {
		return fmt.Errorf("mkdir hugo content: %w", err)
	}
	counter := 1
	total := 0
	for pi := range bc.Outline.Parts {
		p := bc.Outline.Parts[pi]
		partDir := filepath.Join(baseDir, fmt.Sprintf("part-%02d", p.Index))
		storage.OS.MkdirAll(partDir, 0o755)

		var partBuf strings.Builder
		fmt.Fprintf(&partBuf, "---\ntitle: \"%s\"\nslug: \"%s\"\nweight: %d\n---\n\n", p.Title, hugoSlug(p.Title), p.Index)
		if p.Intro != "" {
			fmt.Fprintf(&partBuf, "%s\n\n", p.Intro)
		}
		_ = storage.OS.WriteFile(filepath.Join(partDir, "_index.md"), []byte(partBuf.String()), 0o644)

		for ci := range p.Chapters {
			c := p.Chapters[ci]
			chSlug := hugoSlug(c.Title)
			chPath := filepath.Join(partDir, fmt.Sprintf("ch%02d-%s.md", c.Index, chSlug))
			_, bodyMD, rerr := book.ReadChapter(book.ChapterPath(bc.BookDir, p.Index, c.Index))
			if rerr != nil {
				total++
				continue
			}
			renumbered, next := book.RenumberFootnotes(bodyMD, counter)
			counter = next
			var chBuf strings.Builder
			fmt.Fprintf(&chBuf, "---\ntitle: \"%s\"\nslug: \"%s\"\nweight: %d\ndate: \"%s\"\ndraft: false\n", c.Title, chSlug, c.Index, bc.Meta.CreatedAt.Format("2006-01-02"))
			if c.Abstract != "" {
				fmt.Fprintf(&chBuf, "description: \"%s\"\n", c.Abstract)
			}
			fmt.Fprintf(&chBuf, "---\n\n%s\n", renumbered)
			_ = storage.OS.WriteFile(chPath, []byte(chBuf.String()), 0o644)
			total++
		}
	}
	outDir := filepath.Join(bc.BookDir, "export", "hugo")
	j.Logf("✓ 导出 %s（%d 章 / %d parts）", outDir, total, len(bc.Outline.Parts))
	j.SetResult("file", outDir)
	j.SetResult("chapters", total)
	return nil
}

// exportPDF runs pandoc over the markdown export.
func (s *Server) exportPDF(j *Job, bc *bookCtx) error {
	pandocPath, err := exec.LookPath("pandoc")
	if err != nil {
		return fmt.Errorf("未找到 pandoc：请安装 pandoc 并加入 PATH")
	}
	j.SetProgress(30, "生成 markdown")
	if err := s.exportMD(j, bc); err != nil {
		return err
	}
	mdPath := filepath.Join(bc.BookDir, "export", bc.Meta.Slug+".md")
	pdfPath := strings.TrimSuffix(mdPath, ".md") + ".pdf"
	j.SetProgress(60, "运行 pandoc")
	out, err := exec.Command(pandocPath, mdPath, "--pdf-engine=xelatex", "-o", pdfPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("pandoc 失败: %v\n%s", err, string(out))
	}
	j.Logf("✓ 导出 %s", pdfPath)
	j.SetResult("file", pdfPath)
	return nil
}

// hugoSlug converts a title to a URL-safe slug for Hugo filenames.
func hugoSlug(s string) string {
	var out strings.Builder
	for _, rr := range s {
		switch {
		case (rr >= 'a' && rr <= 'z') || (rr >= '0' && rr <= '9') || rr == '-':
			out.WriteRune(rr)
		case rr >= 'A' && rr <= 'Z':
			out.WriteRune(rr + 32)
		default:
			out.WriteRune('-')
		}
	}
	slug := out.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return strings.Trim(slug, "-")
}

// zipDir packs a directory tree into a zip file (for hugo export download).
func zipDir(srcDir, dstPath string) error {
	if err := storage.OS.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hdr := &zip.FileHeader{Name: filepath.ToSlash(rel), Method: zip.Deflate}
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
}
