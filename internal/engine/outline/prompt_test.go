package outline

import (
	"bytes"
	"strings"
	"testing"
	"text/template"
)

func TestSystemTemplateRenders(t *testing.T) {
	data := promptData{
		Archetype:      "id: test-archetype\nparts: [...]",
		Samples:        "sample 1...\nsample 2...",
		CorpusOutlines: "Book A outline...\nBook B outline...",
		Topic:          "时间的实在",
		Audience:       "educated-general",
		Depth:          "advanced",
		Goal:           "understanding",
		Length:         "long",
		Language:       "zh",
	}
	raw, err := loadSystem()
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := template.New("system").Parse(string(raw))
	if err != nil {
		t.Fatalf("parse system template: %v", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()
	// Verify rendering contains key fields (system template only interpolates these fields)
	if !strings.Contains(out, "sample 1") {
		t.Errorf("missing samples content")
	}
	if !strings.Contains(out, "test-archetype") {
		t.Errorf("missing archetype content")
	}
	if !strings.Contains(out, "Book A outline") {
		t.Errorf("missing corpus outlines content")
	}
	// Verify the template is non-empty
	if len(out) == 0 {
		t.Errorf("rendered output is empty")
	}
}

func TestUserTemplateRenders(t *testing.T) {
	data := promptData{
		Topic:    "时间的实在",
		Audience: "scholar",
		Depth:    "advanced",
		Goal:     "understanding",
		Length:   "long",
		Language: "zh",
	}
	raw, err := loadUser()
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := template.New("user").Parse(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "时间的实在") {
		t.Errorf("missing topic in user prompt")
	}
	if !strings.Contains(out, "scholar") {
		t.Errorf("missing audience in user prompt")
	}
}
