package grill

import "testing"

func TestWalkReturnsCoreDimensionsInOrder(t *testing.T) {
	tree := DefaultTree()
	// Empty answers: topic can be asked first (no deps); audience depends on topic, etc.
	ordered := tree.Walk(map[string]string{})
	if len(ordered) == 0 {
		t.Fatal("Walk returned no dimensions")
	}
	if ordered[0].ID != "topic" {
		t.Errorf("first dimension: %q, want topic", ordered[0].ID)
	}
}

func TestWalkRespectsDependencies(t *testing.T) {
	tree := DefaultTree()
	// After topic answered, audience/depth/language/scope/etc become eligible.
	ordered := tree.Walk(map[string]string{"topic": "X"})
	ids := make([]string, len(ordered))
	for i, d := range ordered {
		ids[i] = d.ID
	}
	mustContain(t, ids, "audience")
	mustContain(t, ids, "language")
	mustContain(t, ids, "scope")
	mustNotContain(t, ids, "archetype") // depends on goal
	mustNotContain(t, ids, "length")    // depends on archetype+depth
}

func TestWalkFullProgression(t *testing.T) {
	tree := DefaultTree()
	answers := map[string]string{
		"topic":     "X",
		"audience":  "scholar",
		"goal":      "understanding",
		"archetype": "ontology-epistemology-practice",
		"depth":     "advanced",
		"length":    "long",
		"language":  "zh",
	}
	ordered := tree.Walk(answers)
	ids := make([]string, len(ordered))
	for i, d := range ordered {
		ids[i] = d.ID
	}
	// All conditional dimensions should be eligible.
	for _, want := range []string{"scope", "example_type", "citation_style", "visualization", "timeliness"} {
		mustContain(t, ids, want)
	}
}

func TestWalkConditionalTriggerRequiresMet(t *testing.T) {
	tree := DefaultTree()
	// citation_style requires audience=scholar.
	// With audience=educated-general, citation_style should NOT be in the walk.
	answers := map[string]string{
		"topic":     "X",
		"audience":  "educated-general",
		"goal":      "understanding",
		"archetype": "ontology-epistemology-practice",
		"depth":     "advanced",
		"length":    "long",
		"language":  "zh",
	}
	ordered := tree.Walk(answers)
	for _, d := range ordered {
		if d.ID == "citation_style" {
			t.Error("citation_style should be skipped when audience != scholar")
		}
	}
}

func TestNextPendingEmpty(t *testing.T) {
	tree := DefaultTree()
	d := tree.NextPending(map[string]string{})
	if d == nil {
		t.Fatal("expected a dimension, got nil")
	}
	if d.ID != "topic" {
		t.Errorf("first pending: %q, want topic", d.ID)
	}
}

func TestNextPendingAfterPartialAnswers(t *testing.T) {
	tree := DefaultTree()
	// After answering topic, audience should be next
	d := tree.NextPending(map[string]string{"topic": "X"})
	if d == nil {
		t.Fatal("expected a dimension, got nil")
	}
	if d.ID != "audience" {
		t.Errorf("next pending: %q, want audience", d.ID)
	}
}

func TestNextPendingReturnsNilWhenComplete(t *testing.T) {
	tree := DefaultTree()
	answers := map[string]string{
		"topic":           "X",
		"audience":        "scholar",
		"goal":            "understanding",
		"archetype":       "ontology-epistemology-practice",
		"depth":           "advanced",
		"length":          "long",
		"language":        "zh",
		"scope":           "single",
		"example_type":    "mixed",
		"citation_style":  "academic",
		"visualization":   "tables",
		"timeliness":      "timeless",
	}
	d := tree.NextPending(answers)
	if d != nil {
		t.Errorf("expected nil, got %q", d.ID)
	}
}

func TestDefaultTreeValidates(t *testing.T) {
	tree := DefaultTree()
	if err := tree.Validate(); err != nil {
		t.Fatalf("DefaultTree validation: %v", err)
	}
}

func TestValidateDetectsCycle(t *testing.T) {
	tree := &DesignTree{
		Dimensions: []Dimension{
			{ID: "a", DependsOn: []string{"b"}},
			{ID: "b", DependsOn: []string{"a"}},
		},
	}
	err := tree.Validate()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestValidateDetectsUnknownDep(t *testing.T) {
	tree := &DesignTree{
		Dimensions: []Dimension{
			{ID: "a", DependsOn: []string{"nonexistent"}},
		},
	}
	err := tree.Validate()
	if err == nil {
		t.Fatal("expected error for unknown dependency, got nil")
	}
}

func TestValidateDetectsDuplicateID(t *testing.T) {
	tree := &DesignTree{
		Dimensions: []Dimension{
			{ID: "a"},
			{ID: "a"},
		},
	}
	err := tree.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate ID, got nil")
	}
}

func TestValidateAnswer(t *testing.T) {
	tree := DefaultTree()
	tests := []struct {
		name     string
		dimID    string
		answer   string
		valid    bool
	}{
		{"topic accepts any", "topic", "anything at all", true},
		{"audience accepts valid option", "audience", "scholar", true},
		{"audience accepts valid option 2", "audience", "educated-general", true},
		{"audience rejects invalid", "audience", "not-an-option", false},
		{"depth accepts valid", "depth", "advanced", true},
		{"depth rejects invalid", "depth", "expert", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := tree.Find(tt.dimID)
			if d == nil {
				t.Fatalf("dimension %q not found", tt.dimID)
			}
			got := d.ValidateAnswer(tt.answer)
			if got != tt.valid {
				t.Errorf("ValidateAnswer(%q) = %v, want %v", tt.answer, got, tt.valid)
			}
		})
	}
}

func mustContain(t *testing.T, ids []string, want string) {
	t.Helper()
	for _, id := range ids {
		if id == want {
			return
		}
	}
	t.Errorf("expected %q in %v", want, ids)
}

func mustNotContain(t *testing.T, ids []string, avoid string) {
	t.Helper()
	for _, id := range ids {
		if id == avoid {
			t.Errorf("did not expect %q in %v", avoid, ids)
		}
	}
}
