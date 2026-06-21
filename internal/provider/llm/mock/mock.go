package mock

import (
	"context"
	"sync"

	"github.com/zhurong/jianwu/internal/provider/llm"
)

// Provider is a scripted Chatter + Embedder for tests.
// Chat returns the same response (or error) for every call.
type Provider struct {
	chatResp   llm.ChatResponse
	chatErr    error
	embedResp  [][]float32
	embedErr   error
	mu         sync.Mutex
	chatCalls  []llm.ChatRequest
	embedCalls []llm.EmbedRequest
}

// New creates a Provider that always returns the given chat response.
func New(resp llm.ChatResponse) *Provider {
	return &Provider{chatResp: resp}
}

// NewError creates a Provider that always returns the given error from Chat.
func NewError(err error) *Provider {
	return &Provider{chatErr: err}
}

// NewEmbed creates a Provider that always returns the given embeddings.
func NewEmbed(embeddings [][]float32) *Provider {
	return &Provider{embedResp: embeddings}
}

func (p *Provider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.chatCalls = append(p.chatCalls, req)
	p.mu.Unlock()
	if p.chatErr != nil {
		return nil, p.chatErr
	}
	resp := p.chatResp
	return &resp, nil
}

func (p *Provider) Embed(ctx context.Context, req llm.EmbedRequest) (*llm.EmbedResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.embedCalls = append(p.embedCalls, req)
	p.mu.Unlock()
	if p.embedErr != nil {
		return nil, p.embedErr
	}
	return &llm.EmbedResponse{Embeddings: p.embedResp}, nil
}

// Calls returns a copy of all Chat calls recorded so far.
func (p *Provider) Calls() []llm.ChatRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]llm.ChatRequest, len(p.chatCalls))
	copy(out, p.chatCalls)
	return out
}
