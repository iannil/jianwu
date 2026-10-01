package collect

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/reader"
	"github.com/iannil/jianwu/internal/provider/search"
)

// --- test doubles ---

// scriptedChatter returns preset responses in order, cycling the last one.
type scriptedChatter struct {
	mu        sync.Mutex
	responses []llm.ChatResponse
	calls     int
}

func (s *scriptedChatter) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.calls
	s.calls++
	if i >= len(s.responses) {
		i = len(s.responses) - 1
	}
	resp := s.responses[i]
	resp.PopulateUsage()
	return &resp, nil
}

type fakeSearcher struct {
	mu    sync.Mutex
	calls int
}

func (f *fakeSearcher) Search(ctx context.Context, query string, opts search.SearchOpts) ([]search.SearchResult, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return []search.SearchResult{
		{Title: "Book Review A", URL: "https://example.com/a"},
		{Title: "TOC B", URL: "https://example.com/b"},
	}, nil
}

type failSearcher struct{}

func (failSearcher) Search(ctx context.Context, query string, opts search.SearchOpts) ([]search.SearchResult, error) {
	return nil, errors.New("no key")
}

type fakeReader struct{}

func (fakeReader) Read(ctx context.Context, url string) (reader.Content, error) {
	return reader.Content{URL: url, Title: "Page " + url, Markdown: "## 目录\n\n第一章 时间\n第二章 空间"}, nil
}

type failReader struct{}

func (failReader) Read(ctx context.Context, url string) (reader.Content, error) {
	return reader.Content{}, errors.New("read failed")
}

// extractedJSON builds an extraction response body from book JSON fragments.
func extractedJSON(books ...string) string {
	return `{"books":[` + strings.Join(books, ",") + `]}`
}

var goodBookA = `{
	"slug": "Time Reality",
	"title_zh": "时间的实在",
	"title_en": "Time Reality",
	"archetype": "ontology-epistemology-practice",
	"audience": "scholar",
	"abstract": "关于时间本质的著作",
	"source_url": "https://example.com/a",
	"parts": [
		{"title_zh": "本体", "role": "ontology", "chapters": [
			{"title_zh": "时间是实在的吗", "abstract": "引入"},
			{"title_zh": "块状宇宙"}
		]},
		{"title_zh": "认识", "chapters": [{"title_zh": "我们如何知道"}]}
	]
}`

var goodBookB = `{
	"slug": "",
	"title_zh": "空 间 的 顺 序",
	"title_en": "Order of Space",
	"archetype": "theory-dynamics-history-present",
	"depth": "bogus",
	"parts": [{"title_zh": "历史", "chapters": [{"title_zh": "古代宇宙论"}]}]
}`

var badNoTitle = `{
	"slug": "no-title",
	"archetype": "ontology-epistemology-practice",
	"parts": [{"title_zh": "x", "chapters": [{"title_zh": "y"}]}]
}`

var badArchetype = `{
	"slug": "bad-arch",
	"title_zh": "无原型",
	"archetype": "not-a-real-archetype",
	"parts": [{"title_zh": "x", "chapters": [{"title_zh": "y"}]}]
}`

var badNoParts = `{
	"slug": "no-parts",
	"title_zh": "无结构",
	"archetype": "ontology-epistemology-practice",
	"parts": []
}`

// --- tests ---

func TestCollectHappyPath(t *testing.T) {
	chat := &scriptedChatter{responses: []llm.ChatResponse{
		{Content: "时间的实在 书评\n时间 目录\nbest books on time"}, // query planning
		{Content: extractedJSON(goodBookA, goodBookB)},   // extraction
	}}
	deps := Deps{Chatter: chat, Searcher: &fakeSearcher{}, Reader: fakeReader{}}

	var steps []string
	res, err := Run(context.Background(), deps, Input{Topic: "时间的实在"}, func(p int, msg string) {
		steps = append(steps, msg)
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Books) != 2 {
		t.Fatalf("books = %d, want 2 (issues: %v)", len(res.Books), res.Issues)
	}

	a := res.Books[0]
	if a.Slug != "time-reality" {
		t.Errorf("slug = %q, want lowercased normalized", a.Slug)
	}
	if a.Title.Zh != "时间的实在" || a.Title.En != "Time Reality" {
		t.Errorf("title = %+v", a.Title)
	}
	if a.Audience != "scholar" || a.Depth != "intermediate" || a.Goal != "understanding" {
		t.Errorf("params = %s/%s/%s", a.Audience, a.Depth, a.Goal)
	}
	if len(a.Parts) != 2 || len(a.Parts[0].Chapters) != 2 || len(a.Parts[1].Chapters) != 1 {
		t.Fatalf("parts = %+v", a.Parts)
	}
	if a.Parts[0].Index != 1 || a.Parts[0].Chapters[0].Index != 1 {
		t.Errorf("indexes not renumbered: %+v", a.Parts[0])
	}
	if a.Source.URL != "https://example.com/a" || a.Source.AccessedAt == "" {
		t.Errorf("source = %+v", a.Source)
	}
	if a.Language[0] != "zh" {
		t.Errorf("language = %v", a.Language)
	}

	// Second book: empty slug falls back to English title, bogus depth defaulted.
	b := res.Books[1]
	if b.Slug != "order-of-space" {
		t.Errorf("slug = %q, want order-of-space", b.Slug)
	}
	if b.Depth != "intermediate" {
		t.Errorf("depth = %q, want defaulted intermediate", b.Depth)
	}

	// Usage: planning + extraction calls tracked.
	if res.Usage.CallCount != 2 {
		t.Errorf("usage call count = %d, want 2", res.Usage.CallCount)
	}
	if len(steps) == 0 {
		t.Error("no progress reported")
	}
}

func TestCollectQueryPlanningFallsBackToTemplates(t *testing.T) {
	chat := &scriptedChatter{responses: []llm.ChatResponse{
		{Content: "   "},                    // empty plan → fallback
		{Content: extractedJSON(goodBookA)}, // extraction
	}}
	deps := Deps{Chatter: chat, Searcher: &fakeSearcher{}, Reader: fakeReader{}}
	res, err := Run(context.Background(), deps, Input{Topic: "时间"}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Books) != 1 {
		t.Fatalf("books = %d, want 1", len(res.Books))
	}
	fallback := false
	for _, issue := range res.Issues {
		if strings.Contains(issue, "模板") {
			fallback = true
		}
	}
	if !fallback {
		t.Errorf("expected fallback issue, got %v", res.Issues)
	}
}

func TestCollectDropsInvalidEntries(t *testing.T) {
	chat := &scriptedChatter{responses: []llm.ChatResponse{
		{Content: "q"},
		{Content: extractedJSON(badNoTitle, goodBookA, badArchetype, badNoParts, goodBookA)},
	}}
	deps := Deps{Chatter: chat, Searcher: &fakeSearcher{}, Reader: fakeReader{}}
	res, err := Run(context.Background(), deps, Input{Topic: "时间"}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// badNoTitle + badArchetype + badNoParts dropped; goodBookA deduped.
	if len(res.Books) != 1 {
		t.Fatalf("books = %d, want 1 (issues: %v)", len(res.Books), res.Issues)
	}
	if res.Books[0].Slug != "time-reality" {
		t.Errorf("kept = %q", res.Books[0].Slug)
	}
	dropped := 0
	for _, issue := range res.Issues {
		if strings.Contains(issue, "丢弃") || strings.Contains(issue, "重复") {
			dropped++
		}
	}
	if dropped < 3 {
		t.Errorf("expected ≥3 drop/dup issues, got %v", res.Issues)
	}
}

func TestCollectSearchFailureErrors(t *testing.T) {
	chat := &scriptedChatter{responses: []llm.ChatResponse{{Content: "q"}}}
	deps := Deps{Chatter: chat, Searcher: failSearcher{}, Reader: fakeReader{}}
	_, err := Run(context.Background(), deps, Input{Topic: "时间"}, nil)
	if err == nil || !strings.Contains(err.Error(), "候选页面") {
		t.Fatalf("expected no-candidates error, got %v", err)
	}
}

func TestCollectReadFailureErrors(t *testing.T) {
	chat := &scriptedChatter{responses: []llm.ChatResponse{{Content: "q"}}}
	deps := Deps{Chatter: chat, Searcher: &fakeSearcher{}, Reader: failReader{}}
	_, err := Run(context.Background(), deps, Input{Topic: "时间"}, nil)
	if err == nil || !strings.Contains(err.Error(), "无法读取") {
		t.Fatalf("expected unreadable-pages error, got %v", err)
	}
}

func TestCollectRequiresTopicAndDeps(t *testing.T) {
	chat := &scriptedChatter{}
	deps := Deps{Chatter: chat, Searcher: &fakeSearcher{}, Reader: fakeReader{}}
	if _, err := Run(context.Background(), deps, Input{}, nil); err == nil {
		t.Error("expected error for empty topic")
	}
	if _, err := Run(context.Background(), Deps{Reader: fakeReader{}}, Input{Topic: "x"}, nil); err == nil {
		t.Error("expected error for missing deps")
	}
}

func TestCollectCountClamp(t *testing.T) {
	// 9 candidates in extraction, count=2 → only 2 kept.
	var many []string
	for i := 0; i < 9; i++ {
		many = append(many, `{"slug": "book-`+strings.Repeat("x", i+1)+`", "title_zh": "书`+strings.Repeat("多", i+1)+`", "archetype": "ontology-epistemology-practice", "parts": [{"title_zh": "p", "chapters": [{"title_zh": "c"}]}]}`)
	}
	chat := &scriptedChatter{responses: []llm.ChatResponse{
		{Content: "q"},
		{Content: extractedJSON(many...)},
	}}
	deps := Deps{Chatter: chat, Searcher: &fakeSearcher{}, Reader: fakeReader{}}
	res, err := Run(context.Background(), deps, Input{Topic: "时间", Count: 2}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Books) != 2 {
		t.Fatalf("books = %d, want 2 (clamped)", len(res.Books))
	}
}
