package mock

import (
	"context"
	"errors"
	"testing"

	"github.com/zhurong/jianwu/internal/provider/llm"
)

func TestMockChatReturnsScriptedResponse(t *testing.T) {
	p := New(llm.ChatResponse{Content: "hello world"})
	resp, err := p.Chat(context.Background(), llm.ChatRequest{Model: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello world" {
		t.Errorf("got %q", resp.Content)
	}
}

func TestMockChatReturnsScriptedError(t *testing.T) {
	p := NewError(errors.New("boom"))
	_, err := p.Chat(context.Background(), llm.ChatRequest{})
	if err == nil || err.Error() != "boom" {
		t.Errorf("got %v", err)
	}
}

func TestMockChatRecordsCallSequence(t *testing.T) {
	p := New(llm.ChatResponse{Content: "ok"})
	_, _ = p.Chat(context.Background(), llm.ChatRequest{Model: "gpt-4", Messages: []llm.Message{{Role: "user", Content: "hi"}}})
	calls := p.Calls()
	if len(calls) != 1 {
		t.Fatalf("got %d calls", len(calls))
	}
	if calls[0].Model != "gpt-4" {
		t.Errorf("model: %q", calls[0].Model)
	}
}

func TestMockEmbed(t *testing.T) {
	p := NewEmbed([][]float32{{0.1, 0.2, 0.3}})
	resp, err := p.Embed(context.Background(), llm.EmbedRequest{Inputs: []string{"foo"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Embeddings) != 1 {
		t.Fatalf("got %d embeddings", len(resp.Embeddings))
	}
}
