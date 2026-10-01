package llmfactory

import (
	"context"
	"errors"
	"testing"

	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/provider/llm"
)

type modelRecorder struct {
	chat, stream, embed string
	err                 error
}

func (p *modelRecorder) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.chat = req.Model
	return &llm.ChatResponse{}, p.err
}
func (p *modelRecorder) Stream(_ context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	p.stream = req.Model
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, p.err
}
func (p *modelRecorder) Embed(_ context.Context, req llm.EmbedRequest) (*llm.EmbedResponse, error) {
	p.embed = req.Model
	return &llm.EmbedResponse{}, p.err
}

func TestDefaultModel(t *testing.T) {
	for _, tt := range []struct{ name, in, want string }{{"default", "", "configured"}, {"override", "explicit", "explicit"}} {
		t.Run(tt.name, func(t *testing.T) {
			inner := &modelRecorder{}
			p := &modelProvider{inner: inner, model: "configured"}
			ctx := context.Background()
			chat := llm.ChatRequest{Model: tt.in}
			embed := llm.EmbedRequest{Model: tt.in}
			if _, err := p.Chat(ctx, chat); err != nil {
				t.Fatal(err)
			}
			if _, err := p.Stream(ctx, chat); err != nil {
				t.Fatal(err)
			}
			if _, err := p.Embed(ctx, embed); err != nil {
				t.Fatal(err)
			}
			if inner.chat != tt.want || inner.stream != tt.want || inner.embed != tt.want {
				t.Fatalf("models chat=%q stream=%q embed=%q want %q", inner.chat, inner.stream, inner.embed, tt.want)
			}
			if chat.Model != tt.in || embed.Model != tt.in {
				t.Fatal("mutated caller request")
			}
		})
	}
}

func TestFallbackModelsBoundIndependently(t *testing.T) {
	first := &modelRecorder{err: errors.New("failed")}
	second := &modelRecorder{}
	p := &llm.FallbackWrapper{Primary: &modelProvider{inner: first, model: "primary-model"}, Fallback: &modelProvider{inner: second, model: "fallback-model"}}
	ctx := context.Background()
	if _, err := p.Chat(ctx, llm.ChatRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Stream(ctx, llm.ChatRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Embed(ctx, llm.EmbedRequest{}); err != nil {
		t.Fatal(err)
	}
	if first.chat != "primary-model" || first.stream != "primary-model" || first.embed != "primary-model" {
		t.Fatalf("primary models = %+v", first)
	}
	if second.chat != "fallback-model" || second.stream != "fallback-model" || second.embed != "fallback-model" {
		t.Fatalf("fallback models = %+v", second)
	}
}

func TestFactoriesBindConfiguredModel(t *testing.T) {
	ref := config.ModelRef{Provider: "ollama", Model: "local-model"}
	secrets := &config.Secrets{}
	chatter, err := NewChatter(ref, secrets)
	if err != nil {
		t.Fatal(err)
	}
	embedder, err := NewEmbedder(ref, secrets)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewProvider(ref, secrets)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []any{chatter, embedder, provider} {
		bound, ok := p.(*modelProvider)
		if !ok || bound.model != ref.Model {
			t.Fatalf("factory returned unbound provider %#v", p)
		}
		if _, ok := p.(llm.Streamer); !ok {
			t.Fatal("factory lost streaming")
		}
	}
}
