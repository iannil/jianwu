package expand

import "testing"

func TestParseFootnotesWithLink(t *testing.T) {
	md := "Some text[^1].\n\n[^1]: [Example](https://example.com/foo) accessed 2026-06-22"
	defs := ParseFootnotes(md)
	if len(defs) != 1 {
		t.Fatalf("got %d defs", len(defs))
	}
	d := defs["1"]
	if d.URL != "https://example.com/foo" {
		t.Errorf("url: %q", d.URL)
	}
	if d.Title != "Example" {
		t.Errorf("title: %q", d.Title)
	}
}

func TestParseFootnotesBareURL(t *testing.T) {
	md := "[^1]: https://example.com/bar"
	defs := ParseFootnotes(md)
	if defs["1"].URL != "https://example.com/bar" {
		t.Errorf("url: %q", defs["1"].URL)
	}
}

func TestCountWordsChinese(t *testing.T) {
	if got := CountWords("时间是宇宙的基本维度"); got != 10 {
		t.Errorf("got %d, want 10 (10 Chinese characters)", got)
	}
}

func TestCountWordsMixed(t *testing.T) {
	if got := CountWords("时间是 relative to observer"); got != 6 {
		t.Errorf("got %d", got)
	}
}
