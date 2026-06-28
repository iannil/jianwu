package expand

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
)

func TestBuildValidatePrompts_InjectsGuide(t *testing.T) {
	sys, _, err := buildValidatePrompts("草稿", ResearchNotes{}, "VALIDATE-GUIDE-MARKER")
	if err != nil {
		t.Fatalf("buildValidatePrompts: %v", err)
	}
	if !strings.Contains(sys, "VALIDATE-GUIDE-MARKER") {
		t.Error("validate system prompt missing injected style guide")
	}
	if strings.Contains(sys, "无空话、无 emoji、术语标「」") {
		t.Error("validate should drop the old inline 3-item shorthand")
	}
}

func TestRunValidateParsesResult(t *testing.T) {
	result := ValidationResult{
		RevisedMarkdown: "# ...",
		Claims: []Claim{
			{Text: "fact A", HasCitation: true},
			{Text: "fact B", HasCitation: false},
		},
	}
	body, _ := json.Marshal(result)
	p := mock.New(llm.ChatResponse{Content: string(body)})
	out, err := RunValidate(context.Background(), p, "draft", ResearchNotes{}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Claims) != 2 {
		t.Errorf("claims: got %d, want 2", len(out.Claims))
	}
	if out.Claims[1].HasCitation != false {
		t.Errorf("claim 2 should be unverified, got %v", out.Claims[1].HasCitation)
	}
	if out.RevisedMarkdown == "" {
		t.Error("empty revised markdown")
	}
}

func TestRunValidateHandlesInvalidJSON(t *testing.T) {
	p := mock.New(llm.ChatResponse{Content: "invalid json"})
	_, err := RunValidate(context.Background(), p, "draft", ResearchNotes{}, "", nil)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestRunValidateWithEmptyDraft(t *testing.T) {
	result := ValidationResult{
		RevisedMarkdown: "",
		Claims:          []Claim{},
	}
	body, _ := json.Marshal(result)
	p := mock.New(llm.ChatResponse{Content: string(body)})
	out, err := RunValidate(context.Background(), p, "", ResearchNotes{}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.RevisedMarkdown != "" {
		t.Errorf("expected empty revised markdown, got %q", out.RevisedMarkdown)
	}
	if len(out.Claims) != 0 {
		t.Errorf("expected no claims, got %d", len(out.Claims))
	}
}

func TestRunValidateWithResearchNotes(t *testing.T) {
	result := ValidationResult{
		RevisedMarkdown: "# Test\n\nContent with citations.",
		Claims: []Claim{
			{Text: "fact", HasCitation: true},
		},
	}
	body, _ := json.Marshal(result)
	p := mock.New(llm.ChatResponse{Content: string(body)})
	notes := ResearchNotes{
		Findings: []Finding{
			{Query: "test", URL: "https://example.com", Title: "Test", Snippet: "snippet", Note: "note"},
		},
		Candidates: []string{"https://example.com"},
	}
	out, err := RunValidate(context.Background(), p, "draft", notes, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Claims) != 1 {
		t.Errorf("claims: got %d, want 1", len(out.Claims))
	}
}
