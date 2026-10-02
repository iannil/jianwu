package export

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"time"
)

// zipEpoch is the fixed timestamp stamped on every zip entry (MS-DOS zip
// time starts at 1980). Deterministic entries are a release-layer
// requirement: identical book state → identical EPUB bytes.
var zipEpoch = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

// Options configures EPUB assembly.
type Options struct {
	// JianwuVersion stamps the generator metadata.
	JianwuVersion string
	// CoverData / CoverMedia optionally attach a cover image
	// (see FindCover).
	CoverData  []byte
	CoverMedia string
}

// BuildEPUB assembles the book into EPUB 3 bytes: mimetype (stored, first),
// OCF container, package document, nav, stylesheet, title/part/chapter
// pages. Deterministic — no wall clock, no memory addresses, fixed entry
// order (dcterms:modified comes from Meta.UpdatedAt).
func BuildEPUB(in BookInput, opts Options) ([]byte, error) {
	if in.Meta == nil || in.Outline == nil {
		return nil, errors.New("export: meta and outline are required")
	}
	if in.Chapters == nil {
		return nil, errors.New("export: chapters missing — run Collect first")
	}
	if len(in.Outline.Parts) == 0 {
		return nil, errors.New("export: book has no parts")
	}

	type entry struct {
		name string
		data []byte
	}
	entries := []entry{
		{"mimetype", []byte("application/epub+zip")},
		{"META-INF/container.xml", []byte(containerXML)},
	}

	// Pre-render every document so failures happen before the zip is built.
	chapterPages := make([]string, len(in.Chapters))
	for i, d := range in.Chapters {
		page, err := chapterXHTML(in, d)
		if err != nil {
			return nil, err
		}
		chapterPages[i] = page
	}

	entries = append(entries,
		entry{"OEBPS/content.opf", []byte(contentOPF(in, opts, in.Chapters))},
		entry{"OEBPS/nav.xhtml", []byte(navXHTML(in, in.Chapters))},
		entry{"OEBPS/style.css", styleCSS()},
		entry{"OEBPS/title.xhtml", []byte(titleXHTML(in, opts.JianwuVersion))},
	)
	if opts.CoverData != nil {
		entries = append(entries,
			entry{"OEBPS/cover.xhtml", []byte(coverXHTML(in, "cover."+coverExt(opts.CoverMedia)))},
			entry{"OEBPS/cover." + coverExt(opts.CoverMedia), opts.CoverData},
		)
	}
	// Every outline part gets a divider page, chapters follow their part —
	// matching the manifest/spine order built in contentOPF.
	for pi := range in.Outline.Parts {
		p := in.Outline.Parts[pi]
		entries = append(entries, entry{"OEBPS/" + partHref(p.Index), []byte(partXHTML(in, p))})
		for i, d := range in.Chapters {
			if d.PartIndex != p.Index {
				continue
			}
			entries = append(entries, entry{"OEBPS/" + chapterHref(d.PartIndex, d.ChapterIndex), []byte(chapterPages[i])})
		}
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		hdr := &zip.FileHeader{Name: e.name, Method: zip.Deflate, Modified: zipEpoch}
		if e.name == "mimetype" {
			// OCF requires the mimetype entry first, uncompressed.
			hdr.Method = zip.Store
		}
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return nil, fmt.Errorf("zip entry %s: %w", e.name, err)
		}
		if _, err := w.Write(e.data); err != nil {
			return nil, fmt.Errorf("zip write %s: %w", e.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), nil
}
