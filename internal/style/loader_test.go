package style

import (
	"strings"
	"testing"
)

func TestLoadGuideReturnsNonEmpty(t *testing.T) {
	s, err := LoadGuide()
	if err != nil {
		t.Fatalf("LoadGuide error: %v", err)
	}
	if len(s) == 0 {
		t.Error("guide is empty")
	}
	if !strings.Contains(s, "硬规则") {
		t.Error("guide missing expected section 硬规则")
	}
}

func TestLoadSamplesReturnsThree(t *testing.T) {
	m, err := LoadSamples()
	if err != nil {
		t.Fatalf("LoadSamples error: %v", err)
	}
	want := []string{
		"ontology-epistemology-practice",
		"diagnosis-decoding-breakthrough",
		"foundations-application-practice",
	}
	if len(m) != len(want) {
		t.Fatalf("got %d samples, want %d", len(m), len(want))
	}
	for _, id := range want {
		if _, ok := m[id]; !ok {
			t.Errorf("missing sample %q", id)
		}
	}
}
