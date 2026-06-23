package expand

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
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
	// Return valid embeddings with dummy data
	out := make([][]float32, len(req.Inputs))
	for i := range out {
		// Return a small embedding vector (3 dimensions)
		out[i] = []float32{0.1, 0.2, 0.3}
	}
	return &llm.EmbedResponse{Embeddings: out}, nil
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
		researchResp: string(researchJSON),
		draftResp:    draftMD,
		validateResp: string(validateJSON),
	}
	out, err := Generate(context.Background(), p, nil, ExpandInput{
		ArchetypeID: "ontology-epistemology-practice",
		Topic:       "T", ChapterTitle: "C", Abstract: "A", Language: "zh",
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
		ArchetypeID: "ontology-epistemology-practice",
		Topic:       "T", ChapterTitle: "C", Abstract: "A",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGenerate_BadArchetypeFailsFast(t *testing.T) {
	in := ExpandInput{ArchetypeID: "nonexistent-archetype", ChapterTitle: "c"}
	_, err := Generate(context.Background(), &mockChatter3Phases{}, nil, in)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected archetype 'not found' before any LLM call, got: %v", err)
	}
}

func TestTruncateUTF8(t *testing.T) {
	s := "这是中文测试字符串"
	got := truncate(s, 4)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected ... suffix, got %q", got)
	}
	if !utf8.ValidString(got) {
		t.Errorf("not valid UTF-8: %q", got)
	}
	// First 4 runes preserved
	if !strings.HasPrefix(got, "这是中文") {
		t.Errorf("expected first 4 runes, got %q", got)
	}
}

func TestLookupSimilarBookCapExpires(t *testing.T) {
	// stubEmbedder returns empty embeddings
	stubEmbedder := mockChatter3Phases{}
	reg := NewToolRegistry(nil, nil, &stubEmbedder)

	// 1st call should succeed
	_, err1 := reg.LookupSimilarBook(context.Background(), "test")
	if err1 != nil {
		t.Fatalf("1st call failed: %v", err1)
	}

	// 2nd call should succeed
	_, err2 := reg.LookupSimilarBook(context.Background(), "test")
	if err2 != nil {
		t.Fatalf("2nd call failed: %v", err2)
	}

	// 3rd call should error
	_, err3 := reg.LookupSimilarBook(context.Background(), "test")
	if err3 == nil {
		t.Fatal("3rd call should error, but succeeded")
	}
	if !strings.Contains(err3.Error(), "call limit") {
		t.Errorf("expected limit error, got: %v", err3)
	}
}
