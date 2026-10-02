// internal/cli/site_test.go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestSite_GeneratesFromPublishedReleases(t *testing.T) {
	tmp := writePublishableBook(t, "demo")
	chdir(t, tmp)
	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})
	// Publish first: the site reads releases, not working state.
	if err := runPublish(cmd, "demo", publishTestOptions()); err != nil {
		t.Fatalf("runPublish: %v", err)
	}
	if err := runSite(cmd, "", "", false); err != nil {
		t.Fatalf("runSite: %v", err)
	}
	indexPath := filepath.Join(tmp, "site", "index.html")
	raw, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("site index missing: %v", err)
	}
	if !strings.Contains(string(raw), "测试书") {
		t.Errorf("index missing book title:\n%s", raw)
	}
	if _, err := os.Stat(filepath.Join(tmp, "site", "opds.xml")); err != nil {
		t.Errorf("opds feed missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "site", "demo", "ch-01-01.html")); err != nil {
		t.Errorf("chapter page missing: %v", err)
	}
}

func TestSite_DryRunWritesNothing(t *testing.T) {
	tmp := writePublishableBook(t, "demo")
	chdir(t, tmp)
	var buf strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	if err := runSite(cmd, "", "", true); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(buf.String(), "nothing written") {
		t.Errorf("dry-run output wrong: %q", buf.String())
	}
	if _, err := os.Stat(filepath.Join(tmp, "site")); !os.IsNotExist(err) {
		t.Error("dry-run must not write the site dir")
	}
}
