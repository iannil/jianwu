package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iannil/jianwu/internal/corpus"
	"github.com/iannil/jianwu/internal/provider/llm"
)

// TestCorpusCollectJob drives POST /api/v1/corpus/collect end to end with
// scripted providers: query planning → extraction → saved files. HOME is
// isolated so the best-effort reindex step finds no embedder key and skips
// without touching real secrets.
func TestCorpusCollectJob(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GLM_API_KEY", "")

	srv, chat := newTestEnv(t)
	chat.responses = []llm.ChatResponse{
		{Content: "时间的实在 书评\ntime reality book toc"}, // query planning
		{Content: `{"books":[{` +
			`"slug": "Collected Book",` +
			`"title_zh": "采集的书",` +
			`"title_en": "Collected Book",` +
			`"archetype": "ontology-epistemology-practice",` +
			`"audience": "scholar",` +
			`"abstract": "自动采集的摘要",` +
			`"source_url": "https://example.com/a",` +
			`"parts": [{"title_zh": "第一部分", "role": "ontology", "chapters": [{"title_zh": "第一章", "abstract": "引入"}]}]` +
			`}]}`}, // extraction
	}

	// Missing topic → 400.
	rec, _ := do(t, srv, "POST", "/api/v1/corpus/collect", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing topic: code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec, resp := do(t, srv, "POST", "/api/v1/corpus/collect", map[string]any{"topic": "时间的实在"})
	wantCode(t, rec, http.StatusAccepted)
	job := waitJob(t, srv, resp["job_id"].(string))
	if job.Status != JobSucceeded {
		t.Fatalf("collect job failed: %s\nlog: %s", job.Err, job.Log)
	}
	if got, _ := job.Result["saved"].(int); got != 1 {
		t.Fatalf("saved = %v, want 1 (log: %s)", job.Result["saved"], job.Log)
	}

	// The book is persisted in the workspace corpus and reloads via corpus.Load.
	path := filepath.Join(srv.WSRoot(), ".jianwu", "corpus", "collected-book.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("saved corpus file: %v", err)
	}
	if !strings.Contains(string(data), "采集的书") {
		t.Errorf("corpus file missing title: %s", data)
	}
	books, err := corpus.Load(srv.WSRoot())
	if err != nil || len(books) != 1 {
		t.Fatalf("corpus.Load = %d books, err %v", len(books), err)
	}
	b := books["collected-book"]
	if b == nil || b.Archetype != "ontology-epistemology-practice" || len(b.Parts[0].Chapters) != 1 {
		t.Fatalf("loaded book = %+v", b)
	}
	// Slug was normalized to [a-z0-9-].
	if b.Slug != "collected-book" {
		t.Errorf("slug = %q, want normalized", b.Slug)
	}
	// Token usage recorded from the 2 LLM calls.
	if got, _ := job.Result["call_count"].(int); got != 2 {
		t.Errorf("call_count = %v, want 2", job.Result["call_count"])
	}
}

func TestCorpusCollectRequiresWorkspace(t *testing.T) {
	srv, _ := newTestEnvAt(t, t.TempDir())
	rec, _ := do(t, srv, "POST", "/api/v1/corpus/collect", map[string]any{"topic": "x"})
	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("code = %d, want 428", rec.Code)
	}
}

func TestCorpusListEmptyWithoutBooks(t *testing.T) {
	srv, _ := newTestEnv(t)
	rec, resp := do(t, srv, "GET", "/api/v1/corpus", nil)
	wantCode(t, rec, http.StatusOK)
	list, ok := resp["books"].([]any)
	if !ok || len(list) != 0 {
		t.Fatalf("books = %v, want empty list", resp["books"])
	}
}
