package outline

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONSchemaHasPartsAndChapters(t *testing.T) {
	raw, err := JSONSchema()
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	props, ok := s["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema missing properties")
	}
	if _, ok := props["parts"]; !ok {
		t.Error("schema missing parts")
	}

	// Spot check: render as string and look for chapter-related keys.
	str := string(raw)
	for _, want := range []string{"parts", "chapters", "title", "abstract", "role"} {
		if !strings.Contains(str, want) {
			t.Errorf("schema missing %q", want)
		}
	}
}

func TestJSONSchemaIsObject(t *testing.T) {
	raw, _ := JSONSchema()
	var s map[string]any
	json.Unmarshal(raw, &s)
	if s["type"] != "object" {
		t.Errorf("type = %v, want object", s["type"])
	}
}
