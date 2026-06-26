package factcheck

import (
	"context"
	"errors"
	"testing"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
	"github.com/iannil/jianwu/internal/provider/reader"
)

// stubReader returns scripted content for URLs.
type stubReader struct {
	content string
	err     error
}

func (s *stubReader) Read(ctx context.Context, url string) (reader.Content, error) {
	if s.err != nil {
		return reader.Content{}, s.err
	}
	return reader.Content{
		URL:      url,
		Title:    "Test Source",
		Markdown: s.content,
	}, nil
}

func TestRunFactCheck(t *testing.T) {
	chatter := mock.New(llm.ChatResponse{Content: `{"verified":true,"reasoning":"The source explicitly states this.","suggested_rewrite":""}`})
	rd := &stubReader{content: "Source text that supports the claim."}

	out, err := Run(context.Background(), chatter, rd, Input{
		ChapterTitle: "Test Chapter",
		Claims: []book.Claim{
			{Text: "The sky is blue.", HasCitation: true},
		},
		Citations: []book.Citation{
			{ID: "1", URL: "https://example.com/sky", Title: "Sky Colors"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Verdicts) != 1 {
		t.Fatalf("got %d verdicts, want 1", len(out.Verdicts))
	}
	if !out.Verdicts[0].Verified {
		t.Error("expected verified=true")
	}
	if out.Verdicts[0].ClaimText != "The sky is blue." {
		t.Errorf("claim text: %q", out.Verdicts[0].ClaimText)
	}
}

func TestRunFactCheckSkipsClaimWithoutCitation(t *testing.T) {
	chatter := mock.New(llm.ChatResponse{Content: `{"verified":true}`})
	rd := &stubReader{content: "irrelevant"}

	out, err := Run(context.Background(), chatter, rd, Input{
		ChapterTitle: "Test",
		Claims: []book.Claim{
			{Text: "Claim without citation.", HasCitation: false},
		},
		Citations: []book.Citation{
			{ID: "1", URL: "https://example.com/x"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Verdicts) != 0 {
		t.Errorf("got %d verdicts, want 0", len(out.Verdicts))
	}
}

func TestRunFactCheckReaderError(t *testing.T) {
	chatter := mock.New(llm.ChatResponse{Content: `{"verified":true}`})
	rd := &stubReader{err: errors.New("network error")}

	out, err := Run(context.Background(), chatter, rd, Input{
		ChapterTitle: "Test",
		Claims: []book.Claim{
			{Text: "Claim.", HasCitation: true},
		},
		Citations: []book.Citation{
			{ID: "1", URL: "https://example.com/x"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Verdicts) != 0 {
		t.Errorf("got %d verdicts, want 0", len(out.Verdicts))
	}
	if len(out.SourceErrors) != 1 {
		t.Errorf("got %d source errors, want 1", len(out.SourceErrors))
	}
}

func TestRunFactCheckLLMError(t *testing.T) {
	chatter := mock.NewError(errors.New("LLM down"))
	rd := &stubReader{content: "source text"}

	out, err := Run(context.Background(), chatter, rd, Input{
		ChapterTitle: "Test",
		Claims: []book.Claim{
			{Text: "Claim.", HasCitation: true},
		},
		Citations: []book.Citation{
			{ID: "1", URL: "https://example.com/x"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Verdicts) != 1 {
		t.Fatalf("got %d verdicts, want 1", len(out.Verdicts))
	}
	if out.Verdicts[0].Verified {
		t.Error("expected verified=false on LLM error")
	}
}

func TestRunFactCheckSkipsWhitelistedClaim(t *testing.T) {
	// Chatter would error if called — but whitelisted claim should skip LLM.
	chatter := mock.NewError(errors.New("should not be called"))
	rd := &stubReader{content: "source text"}

	out, err := Run(context.Background(), chatter, rd, Input{
		ChapterTitle: "Test",
		Claims: []book.Claim{
			{Text: "Already verified claim.", HasCitation: true},
		},
		Citations: []book.Citation{
			{ID: "1", URL: "https://example.com/x"},
		},
		ClaimWhitelist: map[string]bool{"Already verified claim.": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Verdicts) != 1 {
		t.Fatalf("got %d verdicts, want 1", len(out.Verdicts))
	}
	if !out.Verdicts[0].Verified {
		t.Error("expected whitelisted claim to be verified")
	}
	if out.Verdicts[0].Reasoning != "previously verified in another chapter (whitelist)" {
		t.Errorf("reasoning: %q", out.Verdicts[0].Reasoning)
	}
}
