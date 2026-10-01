package deepseek

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

func TestProviderChatJSONObjectMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		if req["model"] != "deepseek-chat" {
			t.Errorf("model: %v", req["model"])
		}
		rf, ok := req["response_format"].(map[string]any)
		if !ok {
			t.Fatalf("response_format missing: %v", req["response_format"])
		}
		// DeepSeek 拒绝 json_schema；必须降级为 json_object。
		if rf["type"] != "json_object" {
			t.Errorf("response_format.type: %v, want json_object", rf["type"])
		}
		if _, has := rf["json_schema"]; has {
			t.Errorf("response_format must not carry json_schema for DeepSeek")
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": `{"ok":true}`}, "finish_reason": "stop"},
			},
			"usage": map[string]any{"prompt_tokens": 8, "completion_tokens": 2},
		})
	}))
	defer srv.Close()

	p, err := New(Config{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := p.Chat(context.Background(), llm.ChatRequest{
		Model:      "deepseek-chat",
		Messages:   []llm.Message{{Role: "user", Content: "输出 json"}},
		JSONSchema: []byte(`{"type":"object"}`),
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != `{"ok":true}` {
		t.Errorf("content: %q", resp.Content)
	}
}

func TestProviderChatNoSchemaOmitsResponseFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		if _, has := req["response_format"]; has {
			t.Errorf("response_format sent without JSONSchema: %v", req["response_format"])
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "plain"}, "finish_reason": "stop"},
			},
		})
	}))
	defer srv.Close()
	p, _ := New(Config{APIKey: "k", BaseURL: srv.URL})
	if _, err := p.Chat(context.Background(), llm.ChatRequest{Model: "deepseek-chat"}); err != nil {
		t.Fatal(err)
	}
}

func TestProviderEmbedUnsupported(t *testing.T) {
	p, err := New(Config{APIKey: "k", BaseURL: "http://unused"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Embed(context.Background(), llm.EmbedRequest{Model: "deepseek-chat", Inputs: []string{"a"}})
	if err == nil || !strings.Contains(err.Error(), "no embeddings endpoint") {
		t.Fatalf("want unsupported-embeddings error, got %v", err)
	}
}

func TestProviderStreamYieldsTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		writeSSE := func(data string) {
			w.Write([]byte("data: " + data + "\n\n"))
			flusher.Flush()
		}
		writeSSE(`{"choices":[{"delta":{"content":"你好"}}]}`)
		writeSSE(`{"choices":[],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`)
		writeSSE(`[DONE]`)
	}))
	defer srv.Close()

	p, _ := New(Config{APIKey: "k", BaseURL: srv.URL})
	ch, err := p.Stream(context.Background(), llm.ChatRequest{Model: "deepseek-chat"})
	if err != nil {
		t.Fatal(err)
	}
	var content string
	var usage *llm.Usage
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatal(chunk.Err)
		}
		content += chunk.Content
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
	}
	if content != "你好" {
		t.Errorf("content: %q", content)
	}
	if usage == nil || usage.TotalTokens != 4 {
		t.Errorf("usage: %+v", usage)
	}
}
