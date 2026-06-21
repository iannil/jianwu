package book

import (
	"strings"
	"testing"
)

func TestSlugifyEdgeCases(t *testing.T) {
	// Empty string
	if Slugify("") != "" {
		t.Errorf("Slugify(%%) should return empty string")
	}

	// Only whitespace
	if Slugify("   ") != "" {
		t.Errorf("Slugify('   ') should return empty string")
	}

	// Mixed ASCII and Chinese (should hash since not pure ASCII)
	got := Slugify("Mixed 中文 Title")
	if got == "" {
		t.Error("Slugify of mixed content returned empty")
	}
	if !strings.HasPrefix(got, "b-") {
		t.Errorf("Mixed content should produce hash, got %q", got)
	}
}
