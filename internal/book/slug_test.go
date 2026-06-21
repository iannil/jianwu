package book

import "testing"

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
