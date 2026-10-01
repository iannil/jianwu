package server

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
)

// TestScaffoldRetryJob drives POST /api/v1/books/{slug}/scaffold-retry with a
// scripted chatter: the one failed chapter is re-scaffolded, the other
// chapters stay untouched, and the outline persists the recovered status.
func TestScaffoldRetryJob(t *testing.T) {
	srv, chat := newTestEnv(t)
	bookDir := createBookFixture(t, srv.WSRoot(), "demo", 2)
	// Mark chapter 2 as a failed scaffold.
	o, err := book.LoadOutline(filepath.Join(bookDir, "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	o.Parts[0].Chapters[1].Status = book.StatusFailed
	o.Parts[0].Chapters[1].Abstract = ""
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), o); err != nil {
		t.Fatal(err)
	}
	chat.responses = []llm.ChatResponse{{Content: testScaffoldJSON}}

	// No failed chapters → 409 is checked on a healthy book first.
	srv2, _ := newTestEnv(t)
	createBookFixture(t, srv2.WSRoot(), "ok", 1)
	rec, _ := do(t, srv2, "POST", "/api/v1/books/ok/scaffold-retry", nil)
	wantCode(t, rec, http.StatusConflict)

	rec, resp := do(t, srv, "POST", "/api/v1/books/demo/scaffold-retry", nil)
	wantCode(t, rec, http.StatusAccepted)
	job := waitJob(t, srv, resp["job_id"].(string))
	if job.Status != JobSucceeded {
		t.Fatalf("scaffold-retry job failed: %s\nlog: %s", job.Err, job.Log)
	}
	if got, _ := job.Result["recovered"].(int); got != 1 {
		t.Fatalf("recovered = %v, want 1 (log: %s)", job.Result["recovered"], job.Log)
	}

	o, err = book.LoadOutline(filepath.Join(bookDir, "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	c2 := o.Parts[0].Chapters[1]
	if c2.Status != book.StatusScaffolded || c2.Abstract != "abstract" {
		t.Errorf("chapter 2 not recovered: status=%q abstract=%q", c2.Status, c2.Abstract)
	}
	if o.Parts[0].Chapters[0].Status != book.StatusScaffolded {
		t.Errorf("untouched chapter changed status: %q", o.Parts[0].Chapters[0].Status)
	}
}
