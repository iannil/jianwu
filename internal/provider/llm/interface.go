package llm

import "context"

// Chatter is the chat-completion interface. Implementations: gemini.Provider, glm.Provider, mock.Provider.
// Stream and Tools are intentionally split out (Streamer, Tooler) — Go idiom of small interfaces.
// They will be added in S5 (grill) and S7 (expand) respectively.
type Chatter interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// Embedder produces embedding vectors.
type Embedder interface {
	Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)
}

// StreamChunk is a single token from a streaming chat response.
// The channel is closed after the final chunk.
type StreamChunk struct {
	Content string // text fragment (may be empty on Done/Err)
	Done    bool   // true when stream is complete (all tokens received)
	Err     error  // non-nil if stream terminated with error; implies Done
}

// Streamer streams chat tokens. Implementations: gemini.Provider, glm.Provider, mock.Provider.
type Streamer interface {
	Stream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
}

// Tooler executes tool calls in an agent loop. Deferred to S7 (expand).
// type Tooler interface { ... }
// type Tooler interface { ... }

// ChatterEmbedder is the union of Chatter + Embedder for wrappers that compose both.
type ChatterEmbedder interface {
	Chatter
	Embedder
}
