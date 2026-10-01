package factcheck

import (
	"context"
	"errors"
	"strings"
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
	urls    []string
}

func (s *stubReader) Read(ctx context.Context, url string) (reader.Content, error) {
	s.urls = append(s.urls, url)
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
	rd := &stubReader{content: "Source text that supports the claim. " + strings.Repeat("Padding body so the content gate passes. ", 20)}

	out, err := Run(context.Background(), chatter, rd, Input{
		ChapterTitle: "Test Chapter",
		Claims: []book.Claim{
			{Text: "The sky is blue.", HasCitation: true, CitationIDs: []string{"1"}},
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

func TestRunFactCheckReportsClaimWithoutCitation(t *testing.T) {
	chatter := mock.New(llm.ChatResponse{Content: `{"verified":true}`})
	rd := &stubReader{content: "irrelevant " + strings.Repeat("Padding body so the content gate passes. ", 20)}

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
	if len(out.Verdicts) != 1 || out.Verdicts[0].Verified {
		t.Errorf("got %d verdicts, want 1 unverified", len(out.Verdicts))
	}
}

func TestRunFactCheckReaderError(t *testing.T) {
	chatter := mock.New(llm.ChatResponse{Content: `{"verified":true}`})
	rd := &stubReader{err: errors.New("network error")}

	out, err := Run(context.Background(), chatter, rd, Input{
		ChapterTitle: "Test",
		Claims: []book.Claim{
			{Text: "Claim.", HasCitation: true, CitationIDs: []string{"1"}},
		},
		Citations: []book.Citation{
			{ID: "1", URL: "https://example.com/x"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Verdicts) != 1 || out.Verdicts[0].Verified {
		t.Errorf("got %d verdicts, want 1 unverified", len(out.Verdicts))
	}
	if len(out.SourceErrors) != 1 {
		t.Errorf("got %d source errors, want 1", len(out.SourceErrors))
	}
}

func TestRunFactCheckLLMError(t *testing.T) {
	chatter := mock.NewError(errors.New("LLM down"))
	rd := &stubReader{content: "source text " + strings.Repeat("Padding body so the content gate passes. ", 20)}

	out, err := Run(context.Background(), chatter, rd, Input{
		ChapterTitle: "Test",
		Claims: []book.Claim{
			{Text: "Claim.", HasCitation: true, CitationIDs: []string{"1"}},
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

func TestRunFactCheckDoesNotTrustWhitelist(t *testing.T) {
	// Chatter would error if called — but whitelisted claim should skip LLM.
	chatter := mock.NewError(errors.New("should not be called"))
	rd := &stubReader{content: "source text " + strings.Repeat("Padding body so the content gate passes. ", 20)}

	out, err := Run(context.Background(), chatter, rd, Input{
		ChapterTitle: "Test",
		Claims: []book.Claim{
			{Text: "Already verified claim.", HasCitation: true, CitationIDs: []string{"1"}},
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
	if out.Verdicts[0].Verified {
		t.Error("whitelist bypassed source verification")
	}
	if out.Verdicts[0].Reasoning == "previously verified in another chapter (whitelist)" {
		t.Errorf("reasoning: %q", out.Verdicts[0].Reasoning)
	}
}

func TestRunExplicitCitationIDs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		ids      []string
		want     int
		verified bool
	}{
		{"shuffled", []string{"b"}, 1, true},
		{"multiple", []string{"b", "a"}, 2, true},
		{"missing", []string{"missing"}, 1, false},
		{"legacy", nil, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := mock.New(llm.ChatResponse{Content: `{"verified":true}`})
			rd := &stubReader{content: "source " + strings.Repeat("Padding body so the content gate passes. ", 20)}
			out, err := Run(context.Background(), p, rd, Input{
				Claims:    []book.Claim{{Text: "claim", HasCitation: true, CitationIDs: tc.ids}},
				Citations: []book.Citation{{ID: "a", URL: "https://example.com/a"}, {ID: "b", URL: "https://example.com/b"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Verdicts) != tc.want {
				t.Fatalf("verdicts: %+v", out.Verdicts)
			}
			if tc.verified {
				for i, id := range tc.ids {
					if rd.urls[i] != "https://example.com/"+id {
						t.Errorf("read wrong URL: %v", rd.urls)
					}
				}
			}
			for i, v := range out.Verdicts {
				if v.Verified != tc.verified {
					t.Errorf("verdict: %+v", v)
				}
				if len(tc.ids) > 0 && v.CitationID != tc.ids[i] {
					t.Errorf("matched wrong source: %+v", v)
				}
			}
		})
	}
}

func TestRunUnavailableSourcesNeverVerify(t *testing.T) {
	for _, tc := range []struct {
		name      string
		citations []book.Citation
		rd        reader.Reader
	}{
		{"invalid URL", []book.Citation{{ID: "1", URL: "file:///private/source"}}, &stubReader{content: "supports " + strings.Repeat("Padding body so the content gate passes. ", 20)}},
		{"empty source", []book.Citation{{ID: "1", URL: "https://example.com"}}, &stubReader{}},
		{"no reader", []book.Citation{{ID: "1", URL: "https://example.com"}}, nil},
		{"duplicate ID", []book.Citation{{ID: "1", URL: "https://example.com"}, {ID: "1", URL: "https://other.example"}}, &stubReader{content: "supports " + strings.Repeat("Padding body so the content gate passes. ", 20)}},
		{"login-walled source", []book.Citation{{ID: "1", URL: "https://zhuanlan.zhihu.com/p/1"}}, &stubReader{content: "# 标题\n\n请您登录后查看更多专业优质内容\n\n打开知乎 App"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Run(context.Background(), mock.New(llm.ChatResponse{Content: `{"verified":true}`}), tc.rd, Input{Claims: []book.Claim{{Text: "claim", CitationIDs: []string{"1"}}}, Citations: tc.citations})
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Verdicts) != 1 || out.Verdicts[0].Verified {
				t.Fatalf("unavailable source verified: %+v", out)
			}
		})
	}
}

// TestRunLoginWalledSourceReasoning asserts the login-wall case is reported
// as an unusable source (recorded in SourceErrors), not a content mismatch.
func TestRunLoginWalledSourceReasoning(t *testing.T) {
	rd := &stubReader{content: "# 标题\n\n请您登录后查看更多专业优质内容\n\n打开知乎 App"}
	out, err := Run(context.Background(), mock.New(llm.ChatResponse{Content: `{"verified":true}`}), rd, Input{
		Claims:    []book.Claim{{Text: "claim", CitationIDs: []string{"1"}}},
		Citations: []book.Citation{{ID: "1", URL: "https://zhuanlan.zhihu.com/p/1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Verdicts) != 1 {
		t.Fatalf("verdicts = %d, want 1", len(out.Verdicts))
	}
	v := out.Verdicts[0]
	if !strings.Contains(v.Reasoning, "source unusable (login-walled)") {
		t.Errorf("reasoning = %q, want login-walled unusable", v.Reasoning)
	}
	if len(out.SourceErrors) != 1 {
		t.Errorf("SourceErrors = %v, want the walled URL recorded", out.SourceErrors)
	}
}
