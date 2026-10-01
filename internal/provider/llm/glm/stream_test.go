package glm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
)

func TestStreamPreservesTrailingUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":3,\"total_tokens\":10}}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	p, err := New(Config{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := p.Stream(context.Background(), llm.ChatRequest{Model: "test"})
	if err != nil {
		t.Fatal(err)
	}
	var usage *llm.Usage
	var text string
	for c := range ch {
		if c.Err != nil {
			t.Fatal(c.Err)
		}
		text += c.Content
		if c.Usage != nil {
			usage = c.Usage
		}
	}
	if text != "hello" || usage == nil || usage.TotalTokens != 10 {
		t.Fatalf("text=%q usage=%+v", text, usage)
	}
}
