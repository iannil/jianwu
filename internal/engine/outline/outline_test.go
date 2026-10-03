package outline

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
)

func TestGenerateValidatesInput(t *testing.T) {
	_, err := Generate(context.Background(), mock.New(llm.ChatResponse{}), Input{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestGenerateParsesLLMResponse(t *testing.T) {
	// Build a sample outline JSON the LLM might return.
	sample := `{"parts":[{"index":1,"title":"第一部 本体","role":"ontology","chapters":[
        {"index":1,"title":"第一章 引子","abstract":"...","key_concepts":["概念A"],"status":"scaffolded"}
    ]}]}`

	p := mock.New(llm.ChatResponse{Content: sample})
	out, err := Generate(context.Background(), p, Input{
		ArchetypeID: "ontology-epistemology-practice",
		Topic:       "时间的实在",
		Audience:    "educated-general",
		Depth:       "advanced",
		Goal:        "understanding",
		Length:      "long",
		Language:    "zh",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(out.Parts) != 1 {
		t.Fatalf("got %d parts", len(out.Parts))
	}
	if out.Parts[0].Role != "ontology" {
		t.Errorf("role: %q", out.Parts[0].Role)
	}
	if len(out.Parts[0].Chapters) != 1 {
		t.Fatalf("got %d chapters", len(out.Parts[0].Chapters))
	}
	if out.Parts[0].Chapters[0].Title != "第一章 引子" {
		t.Errorf("title: %q", out.Parts[0].Chapters[0].Title)
	}
}

func TestGeneratePropagatesLLMError(t *testing.T) {
	p := mock.NewError(errors.New("llm exploded"))
	_, err := Generate(context.Background(), p, Input{
		ArchetypeID: "ontology-epistemology-practice",
		Topic:       "X",
		Audience:    "scholar",
		Depth:       "advanced",
		Goal:        "understanding",
		Length:      "long",
		Language:    "zh",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGenerateRejectsMalformedJSON(t *testing.T) {
	p := mock.New(llm.ChatResponse{Content: "this is not json"})
	_, err := Generate(context.Background(), p, Input{
		ArchetypeID: "ontology-epistemology-practice",
		Topic:       "X",
		Audience:    "scholar",
		Depth:       "advanced",
		Goal:        "understanding",
		Length:      "long",
		Language:    "zh",
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) && !strings.Contains(err.Error(), "parse outline JSON") {
		// The wrap message includes "parse outline JSON" so callers can detect it.
		t.Errorf("error should mention 'parse outline JSON', got: %v", err)
	}
}

func TestGenerateRenumberIndices(t *testing.T) {
	// GLM often returns all-zero indices; positional renumbering must fix them.
	fenced := "```json\n{\"parts\":[{\"index\":0,\"title\":\"P\",\"role\":\"r\",\"chapters\":[{\"index\":0,\"title\":\"a\"},{\"index\":0,\"title\":\"b\"}]}]}\n```"
	c := mock.New(llm.ChatResponse{Content: fenced})
	out, err := Generate(context.Background(), c, Input{
		ArchetypeID: "micro-meso-macro", Topic: "t", Audience: "beginner",
		Depth: "intermediate", Goal: "understanding", Length: "medium", Language: "zh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Parts[0].Index != 1 {
		t.Errorf("part index = %d, want 1", out.Parts[0].Index)
	}
	for j, want := range []int{1, 2} {
		if out.Parts[0].Chapters[j].Index != want {
			t.Errorf("chapter %d index = %d, want %d", j, out.Parts[0].Chapters[j].Index, want)
		}
	}
}

func TestGenerateClampsWordTargets(t *testing.T) {
	// LLM-planned word targets are unreliable at the extremes; Generate
	// must clamp them into the single-pass draft range (absent stays 0).
	sample := `{"parts":[{"index":1,"title":"P","role":"r","chapters":[
        {"index":1,"title":"a","word_count_target":999999},
        {"index":2,"title":"b","word_count_target":100},
        {"index":3,"title":"c"},
        {"index":4,"title":"d","word_count_target":3000}
    ]}]}`
	c := mock.New(llm.ChatResponse{Content: sample})
	out, err := Generate(context.Background(), c, Input{
		ArchetypeID: "micro-meso-macro", Topic: "t", Audience: "beginner",
		Depth: "intermediate", Goal: "understanding", Length: "medium", Language: "zh",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []int{8000, 500, 0, 3000}
	for j, w := range want {
		if got := out.Parts[0].Chapters[j].WordCountTarget; got != w {
			t.Errorf("chapter %d word_count_target = %d, want %d", j+1, got, w)
		}
	}
}

func TestClampWordTarget(t *testing.T) {
	tests := []struct {
		in   int
		want int
	}{
		{0, 0},
		{-5, 0},
		{100, 500},
		{1500, 1500},
		{8000, 8000},
		{999999, 8000},
	}
	for _, tt := range tests {
		if got := clampWordTarget(tt.in); got != tt.want {
			t.Errorf("clampWordTarget(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestWordTargetHint(t *testing.T) {
	tests := []struct {
		length       string
		wantContains string
		wantAnchor   string
	}{
		{"short", "1200-2000", "1500"},
		{"medium", "2000-3500", "2500"},
		{"long", "3000-5000", "4000"},
		{"", "2000-3500", "2500"},
	}
	for _, tt := range tests {
		got := wordTargetHint(tt.length)
		if !strings.Contains(got, tt.wantContains) || !strings.Contains(got, tt.wantAnchor) {
			t.Errorf("wordTargetHint(%q) = %q, want range %q and anchor %q", tt.length, got, tt.wantContains, tt.wantAnchor)
		}
	}
}
