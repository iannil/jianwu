package llm

// Message is a single message in a chat conversation.
type Message struct {
	Role    string `json:"role"`              // "system" | "user" | "assistant" | "tool"
	Content string `json:"content"`
}

// ChatRequest is the input to Chatter.Chat.
type ChatRequest struct {
	Model       string    // provider-specific model ID, e.g. "gemini-2.5-pro"
	Messages    []Message
	Temperature *float64  // optional; nil = provider default
	MaxTokens   int       // 0 = provider default
	Tools       []ToolSpec // optional; for agent loops (deferred to S7)
	// JSONSchema forces structured output (per Q10 decision D). Empty = free-form.
	JSONSchema []byte // optional JSON Schema for response_format
}

// ToolSpec is a tool/function definition (used in S7 expand).
type ToolSpec struct {
	Name        string
	Description string
	Parameters  []byte // JSON Schema
}

// ChatResponse is the output of Chatter.Chat.
type ChatResponse struct {
	Content     string  // assistant's text reply
	ToolCalls   []ToolCall // optional; populated when Tools were sent
	FinishReason string // "stop" | "length" | "tool_calls" | "error"
	TokensIn    int
	TokensOut   int
	Cached      bool   // true if response came from cache
}

// ToolCall is a tool invocation requested by the model (used in S7).
type ToolCall struct {
	ID        string
	Name      string
	Arguments []byte // raw JSON arguments
}

// EmbedRequest is the input to Embedder.Embed.
type EmbedRequest struct {
	Model  string
	Inputs []string
}

// EmbedResponse is the output of Embedder.Embed.
type EmbedResponse struct {
	Embeddings [][]float32 // one vector per input
	TokensIn   int
}
