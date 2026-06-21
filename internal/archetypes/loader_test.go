package archetypes

import (
	"testing"
)

func TestLoadReturnsAllThreeArchetypes(t *testing.T) {
	m, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	want := []string{
		"ontology-epistemology-practice",
		"diagnosis-decoding-breakthrough",
		"foundations-application-practice",
	}
	if len(m) != len(want) {
		t.Fatalf("got %d archetypes, want %d", len(m), len(want))
	}
	for _, id := range want {
		if _, ok := m[id]; !ok {
			t.Errorf("missing archetype %q", id)
		}
	}
}

func TestArchetypeHasParts(t *testing.T) {
	m, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	a := m["ontology-epistemology-practice"]
	if len(a.Parts) == 0 {
		t.Error("archetype has no parts")
	}
	if a.Parts[0].Role == "" {
		t.Error("first part has empty role")
	}
	if a.Name.Zh == "" {
		t.Error("Name.Zh is empty")
	}
}
