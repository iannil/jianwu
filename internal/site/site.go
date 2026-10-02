// Package site generates the static reading surface (ADR 29 step 3) from
// published releases ONLY — distribution never reads working book state, so
// what readers see is always a reviewed, versioned snapshot. Output is a
// self-contained directory deployable to any static host: catalog page, book
// pages, per-chapter reading pages (with the source-verification sections),
// EPUB downloads and an OPDS acquisition feed.
package site

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/release"
)

// ShelfBook is one published book on the shelf: its latest release plus the
// state snapshots carried inside that release.
type ShelfBook struct {
	Slug     string
	Meta     *book.Meta
	Outline  *book.Outline
	Manifest *release.Manifest
	Dir      string   // books/<slug>/releases/<latest>
	Versions []string // all versions, latest first
}

// Scan lists the published shelf: every book under <wsRoot>/books with at
// least one valid release. Books with unreadable manifests are reported in
// skipped, never silently dropped; books without releases are simply absent
// (working drafts are not on the shelf).
func Scan(wsRoot string) (books []ShelfBook, skipped []string, err error) {
	bookDirs, err := os.ReadDir(filepath.Join(wsRoot, "books"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("list books: %w", err)
	}
	for _, bd := range bookDirs {
		slug := bd.Name()
		releasesDir := filepath.Join(wsRoot, "books", slug, "releases")
		entries, rerr := os.ReadDir(releasesDir)
		if rerr != nil {
			continue // no releases yet
		}
		var versions []string
		for _, e := range entries {
			if _, _, ok := release.ParseVersion(e.Name()); ok {
				versions = append(versions, e.Name())
			}
		}
		if len(versions) == 0 {
			continue
		}
		sort.Slice(versions, func(i, j int) bool {
			mi, ni, _ := release.ParseVersion(versions[i])
			mj, nj, _ := release.ParseVersion(versions[j])
			return mi > mj || (mi == mj && ni > nj)
		})
		latest := versions[0]
		relDir := filepath.Join(releasesDir, latest)
		sb, perr := loadShelfBook(slug, relDir, versions)
		if perr != nil {
			skipped = append(skipped, fmt.Sprintf("%s@%s: %v", slug, latest, perr))
			continue
		}
		books = append(books, sb)
	}
	sort.Slice(books, func(i, j int) bool { return books[i].Slug < books[j].Slug })
	return books, skipped, nil
}

// loadShelfBook reads a release's manifest + state snapshots.
func loadShelfBook(slug, relDir string, versions []string) (ShelfBook, error) {
	manRaw, err := os.ReadFile(filepath.Join(relDir, "manifest.json"))
	if err != nil {
		return ShelfBook{}, fmt.Errorf("manifest: %w", err)
	}
	var man release.Manifest
	if err := json.Unmarshal(manRaw, &man); err != nil {
		return ShelfBook{}, fmt.Errorf("parse manifest: %w", err)
	}
	metaRaw, err := os.ReadFile(filepath.Join(relDir, "meta.json"))
	if err != nil {
		return ShelfBook{}, fmt.Errorf("meta snapshot: %w", err)
	}
	var meta book.Meta
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		return ShelfBook{}, fmt.Errorf("parse meta: %w", err)
	}
	outlineRaw, err := os.ReadFile(filepath.Join(relDir, "outline.json"))
	if err != nil {
		return ShelfBook{}, fmt.Errorf("outline snapshot: %w", err)
	}
	var outline book.Outline
	if err := json.Unmarshal(outlineRaw, &outline); err != nil {
		return ShelfBook{}, fmt.Errorf("parse outline: %w", err)
	}
	return ShelfBook{Slug: slug, Meta: &meta, Outline: &outline, Manifest: &man, Dir: relDir, Versions: versions}, nil
}

// Result reports what one generation produced.
type Result struct {
	Books   int
	Pages   int
	EPUBs   int
	Skipped []string
	Dir     string
	Files   []string
}

// Generate rebuilds the site at outDir from the published shelf. The
// directory is fully derived state: it is wiped and regenerated, never
// incrementally updated. Deterministic for a given shelf state (timestamps
// come from manifests, iteration is sorted).
func Generate(wsRoot, outDir string) (*Result, error) {
	books, skipped, err := Scan(wsRoot)
	if err != nil {
		return nil, err
	}
	res := &Result{Books: len(books), Skipped: skipped, Dir: outDir}
	if err := os.RemoveAll(outDir); err != nil {
		return nil, fmt.Errorf("clean site dir: %w", err)
	}
	write := func(rel string, data []byte) error {
		path := filepath.Join(outDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", rel, err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
		res.Files = append(res.Files, rel)
		res.Pages++
		return nil
	}

	if err := write("site.css", siteCSS()); err != nil {
		return nil, err
	}

	var entries []opdsEntry
	for i := range books {
		b := &books[i]
		if err := write(b.Slug+"/index.html", []byte(bookPage(b))); err != nil {
			return nil, err
		}
		docs, err := collectRelease(b)
		if err != nil {
			return nil, fmt.Errorf("book %q: %w", b.Slug, err)
		}
		for j, d := range docs {
			var prev, next string
			if j > 0 {
				prev = chapterHref(docs[j-1].PartIndex, docs[j-1].ChapterIndex)
			}
			if j < len(docs)-1 {
				next = chapterHref(docs[j+1].PartIndex, docs[j+1].ChapterIndex)
			}
			if err := write(filepath.ToSlash(filepath.Join(b.Slug, chapterHref(d.PartIndex, d.ChapterIndex))), []byte(chapterPage(b, d, prev, next))); err != nil {
				return nil, err
			}
		}
		if b.Manifest.EPUB != nil {
			data, err := os.ReadFile(filepath.Join(b.Dir, filepath.FromSlash(b.Manifest.EPUB.Path)))
			if err != nil {
				return nil, fmt.Errorf("book %q epub artifact: %w", b.Slug, err)
			}
			if err := write("epub/"+b.Slug+".epub", data); err != nil {
				return nil, err
			}
			res.EPUBs++
		}
		entries = append(entries, opdsEntry{Book: b})
	}

	if err := write("index.html", []byte(indexPage(books))); err != nil {
		return nil, err
	}
	if err := write("opds.xml", []byte(opdsFeed(entries))); err != nil {
		return nil, err
	}
	return res, nil
}
