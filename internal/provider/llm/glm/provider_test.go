package glm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zhurong/jianwu/internal/provider/llm"
)

func TestProviderChatSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("auth: %q", auth)
		}
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		if req["model"] != "glm-4.6" {
			t.Errorf("model: %v", req["model"])
		}
		// Echo back an OpenAI-style response
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "hello from glm"}, "finish_reason": "stop"},
			},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 3},
		})
	}))
	defer srv.Close()

	p, err := New(Config{APIKey: "test-key", BaseURL: srv.URL + "/v4"})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := p.Chat(context.Background(), llm.ChatRequest{
		Model:    "glm-4.6",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "hello from glm" {
		t.Errorf("got %q", resp.Content)
	}
	if resp.TokensIn != 10 || resp.TokensOut != 3 {
		t.Errorf("tokens: in=%d out=%d", resp.TokensIn, resp.TokensOut)
	}
}

func TestProviderChat4xxDoesNotRetry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "bad"}})
	}))
	defer srv.Close()
	p, _ := New(Config{APIKey: "k", BaseURL: srv.URL + "/v4"})
	_, err := p.Chat(context.Background(), llm.ChatRequest{Model: "glm-4.6"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected 400 in error, got %v", err)
	}
}

func TestProviderEmbed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"embedding": []float32{0.1, 0.2, 0.3}},
				{"embedding": []float32{0.4, 0.5, 0.6}},
			},
			"usage": map[string]any{"prompt_tokens": 5},
		})
	}))
	defer srv.Close()
	p, _ := New(Config{APIKey: "k", BaseURL: srv.URL + "/v4"})
	resp, err := p.Embed(context.Background(), llm.EmbedRequest{
		Model:  "embedding-3",
		Inputs: []string{"a", "b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Embeddings) != 2 {
		t.Fatalf("got %d embeddings", len(resp.Embeddings))
	}
}
