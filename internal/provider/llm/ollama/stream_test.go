package ollama

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
)

func TestStreamReportsFinalUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"message":{"content":"hello"},"done":true,"prompt_eval_count":7,"eval_count":3}`)
	}))
	defer srv.Close()
	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := p.Stream(context.Background(), llm.ChatRequest{Model: "test"})
	if err != nil {
		t.Fatal(err)
	}
	var usage *llm.Usage
	for c := range ch {
		if c.Err != nil {
			t.Fatal(c.Err)
		}
		if c.Usage != nil {
			usage = c.Usage
		}
	}
	if usage == nil || usage.TotalTokens != 10 {
		t.Fatalf("usage=%+v", usage)
	}
}
