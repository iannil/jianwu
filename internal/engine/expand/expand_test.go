package expand

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/zhurong/jianwu/internal/provider/llm"
	"github.com/zhurong/jianwu/internal/provider/llm/mock"
)

// mockChatter3Phases scripts responses for research → draft → validate.
type mockChatter3Phases struct {
	researchResp string
	draftResp    string
	validateResp string
	calls        int
}

func (m *mockChatter3Phases) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	defer func() { m.calls++ }()
	switch m.calls {
	case 0:
		return &llm.ChatResponse{Content: m.researchResp}, nil
	case 1:
		return &llm.ChatResponse{Content: m.draftResp}, nil
	case 2:
		return &llm.ChatResponse{Content: m.validateResp}, nil
	default:
		return nil, errors.New("boom")
	}
}

// Embed for chatterEmbedder interface
func (m *mockChatter3Phases) Embed(ctx context.Context, req llm.EmbedRequest) (*llm.EmbedResponse, error) {
	return &llm.EmbedResponse{}, nil
}

func TestGenerateChainsIterations(t *testing.T) {
	researchJSON, _ := json.Marshal(ResearchNotes{
		Findings:   []Finding{{URL: "https://x", Title: "X"}},
		Candidates: []string{"https://x"},
	})
	draftMD := "# Title\n\nBody[^1].\n\n[^1]: [X](https://x)"
	validateJSON, _ := json.Marshal(ValidationResult{
		RevisedMarkdown: draftMD,
		Claims: []Claim{
			{Text: "fact", HasCitation: true},
			{Text: "unverified", HasCitation: false},
		},
	})

	p := &mockChatter3Phases{
		researchResp:  string(researchJSON),
		draftResp:     draftMD,
		validateResp:  string(validateJSON),
	}
	out, err := Generate(context.Background(), p, nil, ExpandInput{
		Topic: "T", ChapterTitle: "C", Abstract: "A", Language: "zh",
		KeyConcepts: []string{"k"},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out.Markdown == "" {
		t.Error("empty markdown")
	}
	if len(out.Citations) != 1 {
		t.Errorf("citations: %d", len(out.Citations))
	}
	if out.Citations[0].URL != "https://x" {
		t.Errorf("url: %q", out.Citations[0].URL)
	}
	if len(out.UnverifiedClaims) != 1 {
		t.Errorf("unverified: %d", len(out.UnverifiedClaims))
	}
	if out.WordCount == 0 {
		t.Error("zero word count")
	}
}

func TestGeneratePropagatesIter1Error(t *testing.T) {
	p := mock.NewError(errors.New("iter1 fail"))
	_, err := Generate(context.Background(), p, nil, ExpandInput{
		Topic: "T", ChapterTitle: "C", Abstract: "A",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
