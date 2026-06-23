package expand

import (
	"strings"
	"testing"
)

func TestLoadDraftContext_Valid(t *testing.T) {
	dc, err := loadDraftContext("ontology-epistemology-practice")
	if err != nil {
		t.Fatalf("loadDraftContext: %v", err)
	}
	if !strings.Contains(dc.ArchetypeText, "id: ontology-epistemology-practice") {
		t.Errorf("ArchetypeText missing id, got: %q", dc.ArchetypeText)
	}
	if dc.SampleText == "" || dc.SampleText == "(no samples for this archetype)" {
		t.Errorf("SampleText should be the real sample, got: %q", dc.SampleText)
	}
	if !strings.Contains(dc.StyleGuide, "硬规则") {
		t.Errorf("StyleGuide should contain the guide body, got %d bytes", len(dc.StyleGuide))
	}
}

func TestLoadDraftContext_UnknownArchetype(t *testing.T) {
	_, err := loadDraftContext("does-not-exist")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got: %v", err)
	}
}

func TestPickSample(t *testing.T) {
	m := map[string]string{"a": "SAMPLE-A"}
	if got := pickSample(m, "a"); got != "SAMPLE-A" {
		t.Errorf("hit: got %q", got)
	}
	if got := pickSample(m, "z"); got != "(no samples for this archetype)" {
		t.Errorf("miss: got %q", got)
	}
}
