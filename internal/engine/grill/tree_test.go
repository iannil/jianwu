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
