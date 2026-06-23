// internal/cli/footnotes_test.go
package cli

import (
	"strings"
	"testing"
)

func TestRenumberFootnotes_Sequential(t *testing.T) {
	ch1 := "句子[^1]和[^2]。\n\n[^1]: a\n[^2]: b"
	out1, next1 := renumberFootnotes(ch1, 1)
	if next1 != 3 {
		t.Errorf("next after ch1 = %d, want 3", next1)
	}
	if !strings.Contains(out1, "句子[^1]和[^2]。") || !strings.Contains(out1, "[^1]: a") {
		t.Errorf("ch1 should keep 1,2 starting at 1:\n%s", out1)
	}

	ch2 := "另一句[^1]。\n\n[^1]: c"
	out2, next2 := renumberFootnotes(ch2, next1)
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
	out, next := renumberFootnotes("纯正文无脚注", 5)
	if out != "纯正文无脚注" || next != 5 {
		t.Errorf("no-footnote body changed: %q next=%d", out, next)
	}
}
