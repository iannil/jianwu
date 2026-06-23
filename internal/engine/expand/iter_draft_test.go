package expand

import (
	"context"
	"strings"
	"testing"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
)

func TestRunDraftReturnsMarkdown(t *testing.T) {
	p := mock.New(llm.ChatResponse{Content: "# Heading\n\nBody[^1].\n\n[^1]: [X](https://x)"})
	out, err := RunDraft(context.Background(), p, ExpandInput{
		Topic: "T", ChapterTitle: "C", Abstract: "A", Language: "zh",
	}, DraftContext{}, ResearchNotes{Candidates: []string{"https://x"}})
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
	}, DraftContext{}, ResearchNotes{})
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
	}, DraftContext{}, ResearchNotes{})
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

func TestBuildDraftPrompts_InjectsMaterial(t *testing.T) {
	in := ExpandInput{
		Language:     "zh",
		Length:       "medium",
		Topic:        "测试主题",
		ChapterTitle: "测试章节",
		Abstract:     "本章摘要",
		KeyConcepts:  []string{"概念甲", "概念乙"},
		PreviousChapter: &book.OutlineChapter{
			Title: "前章标题", Abstract: "前章摘要", KeyConcepts: []string{"前概念"},
		},
		NextChapter: &book.OutlineChapter{
			Title: "后章标题", Abstract: "后章摘要", KeyConcepts: []string{"后概念"},
		},
	}
	dc := DraftContext{
		ArchetypeText: "id: test-arch\nparts:\n",
		SampleText:    "SAMPLE-MARKER",
		StyleGuide:    "GUIDE-MARKER",
	}
	sys, user, err := buildDraftPrompts(in, dc, ResearchNotes{Candidates: []string{"https://x"}})
	if err != nil {
		t.Fatalf("buildDraftPrompts: %v", err)
	}
	for _, want := range []string{"id: test-arch", "SAMPLE-MARKER", "GUIDE-MARKER"} {
		if !strings.Contains(sys, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
	if strings.Contains(sys, "loaded at orchestrator level") {
		t.Error("system prompt still contains old placeholder")
	}
	for _, want := range []string{"前章标题", "前章摘要", "前概念", "后章标题"} {
		if !strings.Contains(user, want) {
			t.Errorf("user prompt missing adjacent material %q", want)
		}
	}
	if strings.Contains(user, "按 schema 输出") {
		t.Error("user prompt still contains stale 'schema' instruction")
	}
}

func TestBuildDraftPrompts_FirstChapterShowsHeader(t *testing.T) {
	in := ExpandInput{
		Language:     "zh",
		Length:       "medium",
		ChapterTitle: "测试章节",
		NextChapter: &book.OutlineChapter{
			Title: "后章标题", Abstract: "后章摘要", KeyConcepts: []string{"后概念"},
		},
	}
	_, user, err := buildDraftPrompts(in, DraftContext{}, ResearchNotes{})
	if err != nil {
		t.Fatalf("buildDraftPrompts: %v", err)
	}
	for _, want := range []string{"相邻章节", "后章标题"} {
		if !strings.Contains(user, want) {
			t.Errorf("first-chapter user prompt missing %q", want)
		}
	}
}

func TestBuildDraftPrompts_OmitsNilAdjacent(t *testing.T) {
	in := ExpandInput{Language: "zh", Length: "medium", ChapterTitle: "c"}
	_, user, err := buildDraftPrompts(in, DraftContext{}, ResearchNotes{})
	if err != nil {
		t.Fatalf("buildDraftPrompts: %v", err)
	}
	if strings.Contains(user, "相邻章节") {
		t.Error("nil adjacent should omit the 相邻章节 section")
	}
}
