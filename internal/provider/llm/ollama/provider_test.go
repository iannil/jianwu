package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
)

func TestProviderChatSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/chat") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		if req["stream"] != false {
			t.Errorf("stream flag: %v", req["stream"])
		}
		json.NewEncoder(w).Encode(chatResponse{
			Message: message{Role: "assistant", Content: "hello from ollama"},
			Done:    true,
		})
	}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := p.Chat(context.Background(), llm.ChatRequest{
		Model:    "llama3",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "hello from ollama" {
		t.Errorf("got %q", resp.Content)
	}
}

func TestProviderChatEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(chatResponse{Done: true})
	}))
	defer srv.Close()

	p, _ := New(Config{BaseURL: srv.URL})
	_, err := p.Chat(context.Background(), llm.ChatRequest{Model: "llama3"})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestProviderEmbed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(embedResponse{
			Embeddings: [][]float32{{0.1, 0.2, 0.3}, {0.4, 0.5, 0.6}},
		})
	}))
	defer srv.Close()

	p, _ := New(Config{BaseURL: srv.URL})
	resp, err := p.Embed(context.Background(), llm.EmbedRequest{
		Model:  "llama3",
		Inputs: []string{"a", "b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Embeddings) != 2 {
		t.Fatalf("got %d embeddings", len(resp.Embeddings))
	}
}

func TestProviderStreamYieldsTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify stream=true.
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		if req["stream"] != true {
			t.Errorf("stream flag: %v", req["stream"])
		}

		// Write newline-delimited JSON (Ollama streaming format).
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		writeChunk := func(content string, done bool) {
			c, _ := json.Marshal(chatResponse{
				Message: message{Role: "assistant", Content: content},
				Done:    done,
			})
			w.Write(c)
			w.Write([]byte("\n"))
			flusher.Flush()
		}
		writeChunk("Hello ", false)
		writeChunk("world", false)
		writeChunk("", true)
	}))
	defer srv.Close()

	p, _ := New(Config{BaseURL: srv.URL})
	ch, err := p.Stream(context.Background(), llm.ChatRequest{
		Model:    "llama3",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var content string
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatal(chunk.Err)
		}
		content += chunk.Content
	}
	if content != "Hello world" {
		t.Errorf("got %q, want %q", content, "Hello world")
	}
}

func TestProviderStream4xxError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	p, _ := New(Config{BaseURL: srv.URL})
	_, err := p.Stream(context.Background(), llm.ChatRequest{Model: "llama3"})
	if err == nil {
		t.Fatal("expected error for 4xx")
	}
}
