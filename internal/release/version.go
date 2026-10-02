package release

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/storage"
)

// versionRe matches the two-segment MAJOR.MINOR release version.
var versionRe = regexp.MustCompile(`^(\d+)\.(\d+)$`)

// ParseVersion parses a "MAJOR.MINOR" string. ok is false for malformed input.
func ParseVersion(s string) (major, minor int, ok bool) {
	m := versionRe.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, false
	}
	major, _ = strconv.Atoi(m[1])
	minor, _ = strconv.Atoi(m[2])
	return major, minor, true
}

// VersionOpts controls release version derivation.
type VersionOpts struct {
	// Explicit overrides derivation with "MAJOR.MINOR"; must be well-formed
	// and exceed the largest existing version.
	Explicit string
	// ForceMajor bumps major regardless of structural comparison.
	ForceMajor bool
}

// NextVersion derives the next release version from the releases directory.
// The directory listing itself is the version inventory (no index file).
// First release is 1.0. With neither Explicit nor ForceMajor: major bumps
// when the part count or any per-part chapter count changed vs the latest
// release manifest, otherwise minor bumps.
func NextVersion(st storage.Storage, releasesDir string, outline *book.Outline, opts VersionOpts) (string, error) {
	latest, ok, err := latestVersion(st, releasesDir)
	if err != nil {
		return "", err
	}
	if opts.Explicit != "" {
		major, minor, valid := ParseVersion(opts.Explicit)
		if !valid {
			return "", fmt.Errorf("invalid version %q: want MAJOR.MINOR (e.g. 1.2)", opts.Explicit)
		}
		if ok && (major < latest.major || (major == latest.major && minor <= latest.minor)) {
			return "", fmt.Errorf("version %s must exceed the latest release %d.%d", opts.Explicit, latest.major, latest.minor)
		}
		return opts.Explicit, nil
	}
	if !ok {
		return "1.0", nil
	}
	if opts.ForceMajor {
		return fmt.Sprintf("%d.0", latest.major+1), nil
	}
	prev, err := loadManifest(st, releasesDir, fmt.Sprintf("%d.%d", latest.major, latest.minor))
	if err != nil {
		return "", err
	}
	if structureChanged(prev, outline) {
		return fmt.Sprintf("%d.0", latest.major+1), nil
	}
	return fmt.Sprintf("%d.%d", latest.major, latest.minor+1), nil
}

// version2 is a parsed two-segment version.
type version2 struct {
	major, minor int
}

// latestVersion scans releasesDir for MAJOR.MINOR directories and returns the
// largest. A missing directory means no releases yet. Non-matching entries
// (including .staging-*) are ignored.
func latestVersion(st storage.Storage, releasesDir string) (version2, bool, error) {
	entries, err := st.ReadDir(releasesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return version2{}, false, nil
		}
		return version2{}, false, fmt.Errorf("list releases dir: %w", err)
	}
	var latest version2
	found := false
	for _, e := range entries {
		major, minor, ok := ParseVersion(e.Name())
		if !ok {
			continue
		}
		v := version2{major, minor}
		if !found || v.major > latest.major || (v.major == latest.major && v.minor > latest.minor) {
			latest = v
			found = true
		}
	}
	return latest, found, nil
}

// loadManifest reads a release's manifest.json. A missing manifest in an
// existing version is an error, never silently rebuilt (ADR 29 risk rule).
func loadManifest(st storage.Storage, releasesDir, version string) (*Manifest, error) {
	raw, err := st.ReadFile(filepath.Join(releasesDir, version, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("release %s missing manifest.json (dir was modified?): %w", version, err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse manifest of release %s: %w", version, err)
	}
	return &m, nil
}

// structureChanged compares per-part chapter counts between a previous
// release manifest and the current outline. Part reordering or moving a
// chapter across parts changes the counts and counts as structural.
func structureChanged(prev *Manifest, outline *book.Outline) bool {
	want := partCounts(outline)
	got := partCountsFromManifest(prev)
	if len(want) != len(got) {
		return true
	}
	for i := range want {
		if want[i] != got[i] {
			return true
		}
	}
	return false
}

// partCounts returns per-part chapter counts in outline order.
func partCounts(o *book.Outline) []int {
	out := make([]int, 0, len(o.Parts))
	for _, p := range o.Parts {
		out = append(out, len(p.Chapters))
	}
	return out
}

// partCountsFromManifest derives per-part chapter counts from the ordered
// manifest chapter entries.
func partCountsFromManifest(m *Manifest) []int {
	var out []int
	cur := -1
	for _, ch := range m.Content.Chapters {
		if ch.Part != cur {
			out = append(out, 0)
			cur = ch.Part
		}
		out[len(out)-1]++
	}
	return out
}
