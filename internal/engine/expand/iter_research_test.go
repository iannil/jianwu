package expand

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
)

func TestRunResearchParsesLLMResponse(t *testing.T) {
	notes := ResearchNotes{
		Findings:   []Finding{{Query: "q", URL: "https://x", Title: "T", Snippet: "S", Note: "N"}},
		Candidates: []string{"https://x"},
	}
	body, _ := json.Marshal(notes)
	p := mock.New(llm.ChatResponse{Content: string(body)})
	out, err := RunResearch(context.Background(), p, nil, ExpandInput{
		Topic: "T", ChapterTitle: "C", Abstract: "A", KeyConcepts: []string{"k"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Findings) != 1 {
		t.Errorf("findings: %d", len(out.Findings))
	}
}

func TestRunResearchRejectsMalformed(t *testing.T) {
	p := mock.New(llm.ChatResponse{Content: "not json"})
	_, err := RunResearch(context.Background(), p, nil, ExpandInput{
		Topic: "T", ChapterTitle: "C", Abstract: "A", KeyConcepts: []string{"k"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
