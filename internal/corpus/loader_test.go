package corpus

import "testing"

func TestLoadReturnsAllSixBooks(t *testing.T) {
	m, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	want := []string{
		"reality-construction",
		"advancement-of-reality",
		"silent-games",
		"forced-convergence",
		"ai-engineer-in-action",
		"intelligent-computing-center-construction-guide",
	}
	if len(m) != len(want) {
		t.Fatalf("got %d books, want %d", len(m), len(want))
	}
	for _, slug := range want {
		if _, ok := m[slug]; !ok {
			t.Errorf("missing book %q", slug)
		}
	}
}

func TestBookHasPartsAndChapters(t *testing.T) {
	m, _ := Load()
	b := m["reality-construction"]
	if len(b.Parts) == 0 {
		t.Fatal("book has no parts")
	}
	if len(b.Parts[0].Chapters) == 0 {
		t.Error("first part has no chapters")
	}
}
