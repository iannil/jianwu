package scaffolding

import (
	"context"
	"sync"
	"testing"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
)

func TestScaffoldProgressReportsEachOutcome(t *testing.T) {
	outline := &book.Outline{Parts: []book.OutlinePart{{Index: 1, Title: "P", Role: "ontology", Chapters: []book.OutlineChapter{{Index: 1, Title: "success"}, {Index: 2, Title: "failure"}}}}}
	chatter := &titleFailingChatter{inner: mock.New(llm.ChatResponse{Content: `{"abstract":"X","key_concepts":[],"learning_objectives":[],"suggested_examples":[]}`}), failTitle: "failure"}
	var mu sync.Mutex
	got := map[int][]ScaffoldProgress{}
	results := ScaffoldAll(context.Background(), chatter, outline, "ontology-epistemology-practice", ChapterParams{Topic: "T", Language: "en"}, Options{Concurrency: 2, Progress: func(e ScaffoldProgress) {
		mu.Lock()
		defer mu.Unlock()
		got[e.ChapterIdx] = append(got[e.ChapterIdx], e)
	}})
	for _, tc := range []struct {
		in   int
		want string
	}{{1, "done"}, {2, "failed"}} {
		events := got[tc.in]
		if len(events) != 2 {
			t.Fatalf("chapter %d events=%+v", tc.in, events)
		}
		if events[0].Status != "running" || events[1].Status != tc.want {
			t.Errorf("chapter %d events=%+v", tc.in, events)
		}
		for _, e := range events {
			if e.PartIndex != 1 || e.ChapterSlug != fmtKey(1, tc.in) || e.Title == "" {
				t.Errorf("event identity=%+v", e)
			}
		}
		if (events[1].Err != nil) != (tc.want == "failed") {
			t.Errorf("event error=%v", events[1].Err)
		}
	}
	if results[fmtKey(1, 1)].Err != nil || results[fmtKey(1, 2)].Err == nil {
		t.Fatalf("results=%+v", results)
	}
}
