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

// Streamer streams chat chunks. Deferred to S5 (grill).
// type Streamer interface { ... }

// Tooler executes tool calls in an agent loop. Deferred to S7 (expand).
// type Tooler interface { ... }
