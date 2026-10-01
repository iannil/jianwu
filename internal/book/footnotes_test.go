// internal/book/footnotes_test.go
package book

import (
	"strings"
	"testing"
	"time"
)

func TestRenumberFootnotes_Sequential(t *testing.T) {
	ch1 := "句子[^1]和[^2]。\n\n[^1]: a\n[^2]: b"
	out1, next1 := RenumberFootnotes(ch1, 1)
	if next1 != 3 {
		t.Errorf("next after ch1 = %d, want 3", next1)
	}
	if !strings.Contains(out1, "句子[^1]和[^2]。") || !strings.Contains(out1, "[^1]: a") {
		t.Errorf("ch1 should keep 1,2 starting at 1:\n%s", out1)
	}

	ch2 := "另一句[^1]。\n\n[^1]: c"
	out2, next2 := RenumberFootnotes(ch2, next1)
	if next2 != 4 {
		t.Errorf("next after ch2 = %d, want 4", next2)
	}
	// ch2's [^1] must become [^3] globally (no collision with ch1's [^1]).
	if !strings.Contains(out2, "另一句[^3]。") || !strings.Contains(out2, "[^3]: c") {
		t.Errorf("ch2 [^1] should remap to [^3]:\n%s", out2)
	}
	if strings.Contains(out2, "[^1]") {
		t.Errorf("ch2 must not still contain [^1]:\n%s", out2)
	}
}

func TestRenumberFootnotes_NoFootnotes(t *testing.T) {
	out, next := RenumberFootnotes("纯正文无脚注", 5)
	if out != "纯正文无脚注" || next != 5 {
		t.Errorf("no-footnote body changed: %q next=%d", out, next)
	}
}

func TestNormalizeFootnoteDates(t *testing.T) {
	real := time.Date(2026, 10, 1, 11, 5, 3, 0, time.UTC)
	other := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	md := strings.Join([]string{
		"正文引用。[^1]",
		"",
		"[^1]: [Example](https://example.com/a) accessed 2026-06-21",
		"[^2]: [Other](https://example.com/b) accessed 2026-06-21",
		"[^3]: [Unknown](https://example.com/missing) accessed 2026-06-21",
		"[^4]: [NoDate](https://example.com/a)",
	}, "\n")
	got := NormalizeFootnoteDates(md, map[string]time.Time{
		"https://example.com/a": real,
		"https://example.com/b": other,
	})
	for _, want := range []string{
		"[^1]: [Example](https://example.com/a) accessed 2026-10-01",
		"[^2]: [Other](https://example.com/b) accessed 2026-09-15",
		"[^3]: [Unknown](https://example.com/missing) accessed 2026-06-21",
		"[^4]: [NoDate](https://example.com/a) accessed 2026-10-01",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestNormalizeFootnoteDatesEmptyMap(t *testing.T) {
	md := "[^1]: [Example](https://example.com/a) accessed 2026-06-21"
	if got := NormalizeFootnoteDates(md, nil); got != md {
		t.Errorf("nil map should return input unchanged, got %q", got)
	}
}
