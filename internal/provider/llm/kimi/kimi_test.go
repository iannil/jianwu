package kimi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
)

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(Config{})
	if err == nil || !strings.Contains(err.Error(), "APIKey") {
		t.Fatalf("want APIKey error, got %v", err)
	}
}

func TestProviderChatJSONSchemaMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		if req["model"] != "kimi-k3" {
			t.Errorf("model: %v", req["model"])
		}
		rf, ok := req["response_format"].(map[string]any)
		if !ok {
			t.Fatalf("response_format missing: %v", req["response_format"])
		}
		if rf["type"] != "json_schema" {
			t.Errorf("response_format.type: %v (Kimi K 系列支持严格 schema)", rf["type"])
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": `{"ok":true}`}, "finish_reason": "stop"},
			},
			"usage": map[string]any{"prompt_tokens": 12, "completion_tokens": 4},
		})
	}))
	defer srv.Close()

	p, err := New(Config{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := p.Chat(context.Background(), llm.ChatRequest{
		Model:      "kimi-k3",
		Messages:   []llm.Message{{Role: "user", Content: "hi"}},
		JSONSchema: []byte(`{"type":"object"}`),
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != `{"ok":true}` {
		t.Errorf("content: %q", resp.Content)
	}
	if resp.TokensIn != 12 || resp.TokensOut != 4 {
		t.Errorf("tokens: in=%d out=%d", resp.TokensIn, resp.TokensOut)
	}
}

func TestProviderChat4xxErrorPrefixed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "invalid key"}})
	}))
	defer srv.Close()
	p, _ := New(Config{APIKey: "k", BaseURL: srv.URL})
	_, err := p.Chat(context.Background(), llm.ChatRequest{Model: "kimi-k3"})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("want 401 in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "kimi") {
		t.Errorf("error should carry provider prefix: %v", err)
	}
}
