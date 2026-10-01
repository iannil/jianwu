package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/storage"
	"github.com/iannil/jianwu/internal/workspace"
)

type batchChatter struct {
	mu    sync.Mutex
	calls int
	fail  bool
}

func (p *batchChatter) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	if p.fail && strings.Contains(req.Messages[0].Content, "研究助手") && strings.Contains(req.Messages[1].Content, "Chapter 2") {
		return nil, errors.New("chapter failure")
	}
	s := req.Messages[0].Content
	content := `{"findings":[],"candidates":[]}`
	if strings.Contains(s, "写作助手") {
		content = "Body[^4].\n\n[^4]: [Source](https://example.com/source)"
	}
	if strings.Contains(s, "校验助手") {
		content = `{"revised_markdown":"Body[^4].\n\n[^4]: [Source](https://example.com/source)","claims":[{"text":"Body","has_citation":true,"citation_ids":["4"]}]}`
	}
	return &llm.ChatResponse{Content: content, Usage: llm.Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}}, nil
}

func setupBatchBook(t *testing.T, n int) string {
	t.Helper()
	root := t.TempDir()
	if err := workspace.Init(root, workspace.InitOpts{}); err != nil {
		t.Fatal(err)
	}
	old := cliWorkspaceDir
	cliWorkspaceDir = root
	t.Cleanup(func() { cliWorkspaceDir = old })
	dir := filepath.Join(root, "books", "batch")
	if err := book.SaveMeta(filepath.Join(dir, "meta.json"), &book.Meta{Slug: "batch", Title: "Batch", Archetype: "ontology-epistemology-practice", Status: book.BookStatusDraft}); err != nil {
		t.Fatal(err)
	}
	o := &book.Outline{Parts: []book.OutlinePart{{Index: 1, Title: "Part", Chapters: []book.OutlineChapter{}}}}
	for i := 1; i <= n; i++ {
		o.Parts[0].Chapters = append(o.Parts[0].Chapters, book.OutlineChapter{Index: i, Title: fmt.Sprintf("Chapter %d", i), Status: book.StatusScaffolded})
	}
	if err := book.SaveOutline(filepath.Join(dir, "outline.json"), o); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestExpandBatchPreservesAllChapterUpdates(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail bool
		want int
	}{{"all succeed", false, 6}, {"partial failure", true, 5}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupBatchBook(t, 6)
			var output bytes.Buffer
			cmd := newExpandCmd()
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			p := &batchChatter{fail: tc.fail}
			err := runExpandAllWithDeps(cmd, "batch", 0, &ProviderDeps{Chatter: p, Searcher: &stubSearcher{}, Reader: &stubReader{}, Embedder: &stubEmbedder{}}, true)
			if (err != nil) != tc.fail {
				t.Fatalf("error=%v want failure %v", err, tc.fail)
			}
			o, err := book.LoadOutline(filepath.Join(dir, "outline.json"))
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, ch := range o.Parts[0].Chapters {
				if ch.Status == book.StatusExpanded {
					count++
					if len(ch.Claims) != 1 {
						t.Errorf("chapter %d: claims not persisted", ch.Index)
					}
					if _, _, err := book.ReadChapter(book.ChapterPath(dir, 1, ch.Index)); err != nil {
						t.Fatal(err)
					}
				}
			}
			meta, err := book.LoadMeta(filepath.Join(dir, "meta.json"))
			if err != nil {
				t.Fatal(err)
			}
			wantCalls := tc.want * 3
			if tc.fail {
				wantCalls++
			}
			if meta.TokenUsage.CallCount != wantCalls || meta.TokenUsage.TotalTokens != tc.want*15 {
				t.Errorf("usage=%+v want %d calls/%d tokens", meta.TokenUsage, wantCalls, tc.want*15)
			}
			if count != tc.want {
				t.Fatalf("expanded=%d want %d", count, tc.want)
			}
			if !strings.Contains(output.String(), "tokens:") {
				t.Error("--all --tokens omitted usage")
			}
		})
	}
}

func TestExpandBatchCancellation(t *testing.T) {
	dir := setupBatchBook(t, 2)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := newExpandCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&bytes.Buffer{})
	p := &batchChatter{}
	if err := runExpandAllWithDeps(cmd, "batch", 0, &ProviderDeps{Chatter: p, Searcher: &stubSearcher{}, Reader: &stubReader{}, Embedder: &stubEmbedder{}}, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v want canceled", err)
	}
	if p.calls != 0 {
		t.Errorf("made %d calls after cancellation", p.calls)
	}
	o, err := book.LoadOutline(filepath.Join(dir, "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range o.Parts[0].Chapters {
		if ch.Status != book.StatusScaffolded {
			t.Errorf("status=%s", ch.Status)
		}
	}
}

// A disk error after prose replacement must not leave old approval attached to new prose.
type failOutlineStorage struct {
	storage.Storage
	fail bool
}

func (s *failOutlineStorage) Rename(a, b string) error {
	if s.fail && strings.HasSuffix(b, "outline.json") {
		s.fail = false
		return errors.New("outline rename failed")
	}
	return s.Storage.Rename(a, b)
}
func TestExpandSaveFailureRestoresReviewedChapter(t *testing.T) {
	dir := setupBatchBook(t, 1)
	path := book.ChapterPath(dir, 1, 1)
	if _, err := book.WriteChapter(dir, 1, 1, book.ChapterFrontmatter{Title: "Original", Status: book.StatusReviewed}, "Original reviewed prose"); err != nil {
		t.Fatal(err)
	}
	o, err := book.LoadOutline(filepath.Join(dir, "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	o.Parts[0].Chapters[0].Status = book.StatusReviewed
	if err := book.SaveOutline(filepath.Join(dir, "outline.json"), o); err != nil {
		t.Fatal(err)
	}
	old := book.DefaultStorage
	book.DefaultStorage = &failOutlineStorage{Storage: old, fail: true}
	t.Cleanup(func() { book.DefaultStorage = old })
	cmd := newExpandCmd()
	cmd.SetOut(&bytes.Buffer{})
	err = runExpand(cmd, []string{"batch", "01-01"}, 2, &ProviderDeps{Chatter: &batchChatter{}, Searcher: &stubSearcher{}, Reader: &stubReader{}, Embedder: &stubEmbedder{}}, false)
	if err == nil {
		t.Fatal("expected outline save failure")
	}
	fm, body, err := book.ReadChapter(path)
	if err != nil {
		t.Fatal(err)
	}
	if fm.Status != book.StatusReviewed || body != "Original reviewed prose" {
		t.Fatalf("chapter corrupted: status %s body %s", fm.Status, body)
	}
	o, err = book.LoadOutline(filepath.Join(dir, "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	if o.Parts[0].Chapters[0].Status != book.StatusReviewed {
		t.Fatal("original outline lost")
	}
}

type blockedBatchChatter struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
}

func (p *blockedBatchChatter) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	p.started <- struct{}{}
	<-ctx.Done()
	return nil, ctx.Err()
}
func TestExpandBatchBoundsWorkAndCancelsQueuedChapters(t *testing.T) {
	setupBatchBook(t, 9)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := newExpandCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&bytes.Buffer{})
	p := &blockedBatchChatter{started: make(chan struct{}, 9)}
	done := make(chan error, 1)
	go func() {
		done <- runExpandAllWithDeps(cmd, "batch", 0, &ProviderDeps{Chatter: p, Searcher: &stubSearcher{}, Reader: &stubReader{}, Embedder: &stubEmbedder{}}, false)
	}()
	for i := 0; i < 5; i++ {
		select {
		case <-p.started:
		case <-time.After(5 * time.Second):
			t.Fatal("workers did not start")
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("batch did not stop")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.calls != 5 {
		t.Errorf("started %d calls, want at most five before queued cancellation", p.calls)
	}
}
