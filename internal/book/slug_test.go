package book

import (
	"testing"

	"golang.org/x/text/unicode/norm"
)

func TestSlugifyAsciiTitle(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Reality of Time", "reality-of-time"},
		{"  Hello,  World!  ", "hello-world"},
		{"Foo / Bar", "foo-bar"},
		{"Multiple   Spaces", "multiple-spaces"},
	}
	for _, c := range cases {
		got := Slugify(c.in)
		if got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSlugifyChineseTitleReturnsPinyinOrHash(t *testing.T) {
	// Chinese titles cannot be safely slugified without a transliteration lib.
	// For v1.0 we accept a deterministic hash fallback.
	got := Slugify("时间的实在")
	if got == "" {
		t.Error("Slugify of Chinese returned empty")
	}
	// Must be deterministic
	if Slugify("时间的实在") != got {
		t.Error("Slugify is not deterministic")
	}
	// Must be lowercase ASCII
	for _, r := range got {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			t.Errorf("Slugify output contains non-slug char %q", r)
		}
	}
}

func TestSlugifyNFCEquivalence(t *testing.T) {
	// NFC (composed): "café" as single codepoint U+00E9
	nfc := "café"
	// NFD (decomposed): "e" + combining acute accent U+0301
	nfd := norm.NFD.String(nfc)

	slugNFC := Slugify(nfc)
	slugNFD := Slugify(nfd)

	if slugNFC != slugNFD {
		t.Errorf("NFC and NFD forms produced different slugs: NFC=%q, NFD=%q", slugNFC, slugNFD)
	}
	// Both should produce ASCII-only slugs
	for _, r := range slugNFC {
		if r > 127 {
			t.Errorf("NFC slug contains non-ASCII: %q", slugNFC)
		}
	}
}
