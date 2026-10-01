package expand

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/reader"
	"github.com/iannil/jianwu/internal/provider/search"
)

// stubSearcher implements search.Searcher for tests.
type stubSearcher struct {
	results []search.SearchResult
	err     error
	calls   int
}

// usableTestMD is long enough to pass reader.ContentIssue (>= 500 runes).
var usableTestMD = "This is a full-length test article body. " +
	"It explains the protocol design in several detailed paragraphs so that " +
	"the content-quality gate treats it as usable page content. " +
	strings.Repeat("The quick brown fox jumps over the lazy dog while explaining QUIC handshakes. ", 12)

func (s *stubSearcher) Search(ctx context.Context, query string, opts search.SearchOpts) ([]search.SearchResult, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.results, nil
}

// stubReader implements reader.Reader for tests.
type stubReader struct {
	content reader.Content
	err     error
	calls   int
}

func (r *stubReader) Read(ctx context.Context, url string) (reader.Content, error) {
	r.calls++
	if r.err != nil {
		return reader.Content{}, r.err
	}
	return r.content, nil
}

// stubEmbedder implements llm.Embedder for tests.
type stubEmbedder struct{}

func (e *stubEmbedder) Embed(ctx context.Context, req llm.EmbedRequest) (*llm.EmbedResponse, error) {
	// Return dummy embeddings
	embeddings := make([][]float32, len(req.Inputs))
	for i := range embeddings {
		embeddings[i] = []float32{0.1, 0.2, 0.3}
	}
	return &llm.EmbedResponse{
		Embeddings: embeddings,
		TokensIn:   len(req.Inputs) * 10, // dummy token count
	}, nil
}

func TestWebSearchCapExpires(t *testing.T) {
	stubSearch := &stubSearcher{
		results: []search.SearchResult{
			{Title: "Test", URL: "https://example.com", Snippet: "A result"},
		},
	}
	registry := NewToolRegistry(stubSearch, &stubReader{}, &stubEmbedder{})

	ctx := context.Background()

	// First 5 calls should succeed
	for i := 0; i < 5; i++ {
		_, err := registry.WebSearch(ctx, "test query")
		if err != nil {
			t.Fatalf("Call %d should succeed, got error: %v", i+1, err)
		}
	}

	// 6th call should fail
	_, err := registry.WebSearch(ctx, "test query")
	if err == nil {
		t.Fatal("6th call should fail with cap exceeded error")
	}
	if stubSearch.calls != 5 {
		t.Fatalf("Expected 5 actual search calls, got %d", stubSearch.calls)
	}
}

func TestReadURLCapExpires(t *testing.T) {
	stubRdr := &stubReader{
		content: reader.Content{
			URL:      "https://example.com",
			Title:    "Test Page",
			Markdown: usableTestMD,
		},
	}
	registry := NewToolRegistry(&stubSearcher{}, stubRdr, &stubEmbedder{})

	ctx := context.Background()

	// First 10 calls should succeed
	for i := 0; i < 10; i++ {
		_, err := registry.ReadURL(ctx, "https://example.com")
		if err != nil {
			t.Fatalf("Call %d should succeed, got error: %v", i+1, err)
		}
	}

	// 11th call should fail
	_, err := registry.ReadURL(ctx, "https://example.com")
	if err == nil {
		t.Fatal("11th call should fail with cap exceeded error")
	}
	if stubRdr.calls != 10 {
		t.Fatalf("Expected 10 actual reader calls, got %d", stubRdr.calls)
	}
}

func TestReadURLRegistersCitation(t *testing.T) {
	stubRdr := &stubReader{
		content: reader.Content{
			URL:      "https://example.com/test",
			Title:    "Test Article",
			Markdown: usableTestMD,
		},
	}
	registry := NewToolRegistry(&stubSearcher{}, stubRdr, &stubEmbedder{})

	ctx := context.Background()
	url := "https://example.com/test"

	_, err := registry.ReadURL(ctx, url)
	if err != nil {
		t.Fatalf("ReadURL should succeed, got error: %v", err)
	}

	citations := registry.Citations()
	if len(citations) != 1 {
		t.Fatalf("Expected 1 citation, got %d", len(citations))
	}

	citation := citations[0]
	if citation.URL != url {
		t.Errorf("Expected URL %s, got %s", url, citation.URL)
	}
	if citation.Title != "Test Article" {
		t.Errorf("Expected title 'Test Article', got '%s'", citation.Title)
	}
	if citation.ReaderProvider != "jina" {
		t.Errorf("Expected ReaderProvider 'jina', got '%s'", citation.ReaderProvider)
	}
	if citation.Snippet == "" {
		t.Error("Expected Snippet to be set, got empty string")
	}
	if citation.AccessedAt.IsZero() {
		t.Error("Expected AccessedAt to be set, got zero time")
	}
}

func TestSearchAndRegisterAddsCitations(t *testing.T) {
	stubSearch := &stubSearcher{
		results: []search.SearchResult{
			{
				Title:   "First Result",
				URL:     "https://example.com/first",
				Snippet: "First snippet",
			},
			{
				Title:   "Second Result",
				URL:     "https://example.com/second",
				Snippet: "Second snippet",
			},
		},
	}
	registry := NewToolRegistry(stubSearch, &stubReader{}, &stubEmbedder{})

	ctx := context.Background()

	_, err := registry.SearchAndRegister(ctx, "test query")
	if err != nil {
		t.Fatalf("SearchAndRegister should succeed, got error: %v", err)
	}

	citations := registry.Citations()
	if len(citations) != 2 {
		t.Fatalf("Expected 2 citations, got %d", len(citations))
	}

	// Check first citation
	firstFound := false
	secondFound := false
	for _, c := range citations {
		if c.URL == "https://example.com/first" {
			firstFound = true
			if c.Title != "First Result" {
				t.Errorf("Expected title 'First Result', got '%s'", c.Title)
			}
			if c.Snippet != "First snippet" {
				t.Errorf("Expected snippet 'First snippet', got '%s'", c.Snippet)
			}
			if c.SearchProvider != "brave" {
				t.Errorf("Expected SearchProvider 'brave', got '%s'", c.SearchProvider)
			}
			if c.AccessedAt.IsZero() {
				t.Error("Expected AccessedAt to be set, got zero time")
			}
		}
		if c.URL == "https://example.com/second" {
			secondFound = true
			if c.Title != "Second Result" {
				t.Errorf("Expected title 'Second Result', got '%s'", c.Title)
			}
			if c.Snippet != "Second snippet" {
				t.Errorf("Expected snippet 'Second snippet', got '%s'", c.Snippet)
			}
			if c.SearchProvider != "brave" {
				t.Errorf("Expected SearchProvider 'brave', got '%s'", c.SearchProvider)
			}
		}
	}

	if !firstFound {
		t.Error("First citation not found in registry")
	}
	if !secondFound {
		t.Error("Second citation not found in registry")
	}
}

func TestSearchAndRegisterDoesNotDuplicate(t *testing.T) {
	stubSearch := &stubSearcher{
		results: []search.SearchResult{
			{
				Title:   "Duplicate Test",
				URL:     "https://example.com/dup",
				Snippet: "First snippet",
			},
		},
	}
	registry := NewToolRegistry(stubSearch, &stubReader{}, &stubEmbedder{})

	ctx := context.Background()

	// First call
	_, err := registry.SearchAndRegister(ctx, "test query 1")
	if err != nil {
		t.Fatalf("First call should succeed, got error: %v", err)
	}

	// Second call with same URL (but different snippet)
	stubSearch.results[0].Snippet = "Second snippet"
	_, err = registry.SearchAndRegister(ctx, "test query 2")
	if err != nil {
		t.Fatalf("Second call should succeed, got error: %v", err)
	}

	citations := registry.Citations()
	if len(citations) != 1 {
		t.Fatalf("Expected 1 citation (no duplicate), got %d", len(citations))
	}

	citation := citations[0]
	if citation.Snippet != "First snippet" {
		t.Errorf("Expected original snippet 'First snippet', got '%s' (should not overwrite)", citation.Snippet)
	}
}

func TestSearchAndRegisterPropagatesSearchErrors(t *testing.T) {
	stubSearch := &stubSearcher{
		err: errors.New("search service unavailable"),
	}
	registry := NewToolRegistry(stubSearch, &stubReader{}, &stubEmbedder{})

	ctx := context.Background()

	_, err := registry.SearchAndRegister(ctx, "test query")
	if err == nil {
		t.Fatal("SearchAndRegister should propagate search errors")
	}
	if err.Error() != "search service unavailable" {
		t.Errorf("Expected error 'search service unavailable', got '%s'", err.Error())
	}

	// Verify no citations were registered
	citations := registry.Citations()
	if len(citations) != 0 {
		t.Fatalf("Expected 0 citations after error, got %d", len(citations))
	}
}

func TestReadURLPropagatesReaderErrors(t *testing.T) {
	stubRdr := &stubReader{
		err: errors.New("network error"),
	}
	registry := NewToolRegistry(&stubSearcher{}, stubRdr, &stubEmbedder{})

	ctx := context.Background()

	_, err := registry.ReadURL(ctx, "https://example.com")
	if err == nil {
		t.Fatal("ReadURL should propagate reader errors")
	}
	if err.Error() != "network error" {
		t.Errorf("Expected error 'network error', got '%s'", err.Error())
	}

	// Verify no citation was registered
	citations := registry.Citations()
	if len(citations) != 0 {
		t.Fatalf("Expected 0 citations after error, got %d", len(citations))
	}
}

func TestReadURLRejectsLoginWalledContent(t *testing.T) {
	stubRdr := &stubReader{
		content: reader.Content{
			URL:      "https://zhuanlan.zhihu.com/p/1",
			Title:    "深入剖析 HTTP/3 协议",
			Markdown: "# 深入剖析 HTTP/3 协议\n\n请您登录后查看更多专业优质内容\n\n打开知乎 App，查看更多内容",
		},
	}
	// Search registers the URL as candidate first (mirrors SearchAndRegister).
	searcher := &stubSearcher{results: []search.SearchResult{
		{URL: "https://zhuanlan.zhihu.com/p/1", Title: "深入剖析 HTTP/3 协议"},
	}}
	registry := NewToolRegistry(searcher, stubRdr, &stubEmbedder{})
	if _, err := registry.SearchAndRegister(context.Background(), "quic"); err != nil {
		t.Fatal(err)
	}
	before := registry.Citations()
	found := false
	for _, c := range before {
		if c.URL == "https://zhuanlan.zhihu.com/p/1" {
			found = true
		}
	}
	if !found {
		t.Fatal("candidate should be registered by search")
	}

	_, err := registry.ReadURL(context.Background(), "https://zhuanlan.zhihu.com/p/1")
	if err == nil {
		t.Fatal("ReadURL should reject login-walled content")
	}
	registry.DropCitation("https://zhuanlan.zhihu.com/p/1")
	for _, c := range registry.Citations() {
		if c.URL == "https://zhuanlan.zhihu.com/p/1" {
			t.Fatal("dropped URL should no longer be a citation candidate")
		}
	}
}
