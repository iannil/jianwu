package expand

import (
	"context"
	"testing"

	"github.com/zhurong/jianwu/internal/provider/llm"
	"github.com/zhurong/jianwu/internal/provider/llm/mock"
)

func TestRunDraftReturnsMarkdown(t *testing.T) {
	p := mock.New(llm.ChatResponse{Content: "# Heading\n\nBody[^1].\n\n[^1]: [X](https://x)"})
	out, err := RunDraft(context.Background(), p, ExpandInput{
		Topic: "T", ChapterTitle: "C", Abstract: "A", Language: "zh",
	}, ResearchNotes{Candidates: []string{"https://x"}})
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Error("empty draft")
	}
}

func TestRunDraftWithDefaultLanguage(t *testing.T) {
	p := mock.New(llm.ChatResponse{Content: "# Test\n\nContent."})
	out, err := RunDraft(context.Background(), p, ExpandInput{
		Topic: "T", ChapterTitle: "C", Abstract: "A", Language: "",
	}, ResearchNotes{})
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Error("empty draft")
	}
}

func TestRunDraftWithLengthHints(t *testing.T) {
	p := mock.New(llm.ChatResponse{Content: "# Long\n\nContent."})
	out, err := RunDraft(context.Background(), p, ExpandInput{
		Topic: "T", ChapterTitle: "C", Abstract: "A", Language: "en", Length: "long",
	}, ResearchNotes{})
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Error("empty draft")
	}
}

func TestParagraphHint(t *testing.T) {
	tests := []struct {
		length string
		want   string
	}{
		{"short", "50-150 字/段"},
		{"long", "200-400 字/段"},
		{"", "100-200 字/段"},
		{"medium", "100-200 字/段"},
	}
	for _, tt := range tests {
		if got := paragraphHint(tt.length); got != tt.want {
			t.Errorf("paragraphHint(%q) = %q, want %q", tt.length, got, tt.want)
		}
	}
}

func TestWordTarget(t *testing.T) {
	tests := []struct {
		length string
		want   int
	}{
		{"short", 1500},
		{"long", 4000},
		{"", 2500},
		{"medium", 2500},
	}
	for _, tt := range tests {
		if got := wordTarget(tt.length); got != tt.want {
			t.Errorf("wordTarget(%q) = %d, want %d", tt.length, got, tt.want)
		}
	}
}

func TestJoinComma(t *testing.T) {
	tests := []struct {
		input []string
		want  string
	}{
		{[]string{"a", "b", "c"}, "a, b, c"},
		{[]string{"a"}, "a"},
		{[]string{}, ""},
	}
	for _, tt := range tests {
		if got := joinComma(tt.input); got != tt.want {
			t.Errorf("joinComma(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestJoinCandidates(t *testing.T) {
	tests := []struct {
		input []string
		want  string
	}{
		{[]string{"https://a.com", "https://b.com"}, "- https://a.com\n- https://b.com"},
		{[]string{"https://a.com"}, "- https://a.com"},
		{[]string{}, ""},
	}
	for _, tt := range tests {
		if got := joinCandidates(tt.input); got != tt.want {
			t.Errorf("joinCandidates(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestJsonMarshalNotes(t *testing.T) {
	notes := ResearchNotes{
		Findings: []Finding{
			{Query: "test", URL: "https://example.com", Title: "Test", Snippet: "snippet", Note: "note"},
		},
		Candidates: []string{"https://example.com"},
	}
	got, err := jsonMarshalNotes(notes)
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Error("empty JSON")
	}
}
