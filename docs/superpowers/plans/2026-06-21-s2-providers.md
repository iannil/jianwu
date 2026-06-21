# jianwu S2: Provider Abstractions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the LLM provider + search provider + URL reader abstraction layer for jianwu. Pluggable implementations (Gemini via official SDK, GLM via direct REST, Brave/Serper search, Jina reader) with retry + fallback. No engine integration yet — providers are exercisable via unit tests with mocks.

**Architecture:** Go interfaces per DESIGN.md §5.3 (LLM) and §5.5 (Search/Reader) but split into small interfaces (Go idiom): `Chatter`, `Embedder` for LLM; `Searcher`, `Reader` for search. Implementations: `gemini.Provider` (uses `google.golang.org/genai` SDK), `glm.Provider` (direct HTTP, OpenAI-compatible pattern reusable for Qwen/Moonshot later), `brave.Searcher`, `serper.Searcher`, `jina.Reader`. Retry + fallback wrappers compose providers. Caching hooks for Gemini context cache API. All keys come from `config.LoadSecrets()` (built in S1).

**Tech Stack:** Go 1.22+, `google.golang.org/genai` (Gemini SDK), `net/http` + `encoding/json` (GLM/search/reader HTTP), `golang.org/x/sync/errgroup` (already in S1 deps).

## Global Constraints

- Go version floor: 1.22 (S1 set this)
- Module path: `github.com/zhurong/jianwu`
- License: AGPL-3.0 (code)
- Test discipline: TDD throughout; LLM-driven code uses test-after with mocks
- Exit codes: `4` = LLM/provider error, `5` = network error (defined in S1)
- All HTTP calls use `context.Context` for cancellation/timeout
- API keys from `config.LoadSecrets()` only — never from CLI args or workspace config
- Retry policy (per Q7): 3 attempts, exponential backoff (1s/2s/4s base), ±20% jitter, on network/timeout/429/5xx; 4xx does NOT trigger retry
- Fallback policy (per Q7): same triggers as retry; once retry exhausts on primary, switch to fallback model
- Caching (per Q6): Gemini uses context cache API; GLM uses native prompt caching (transparent); NO client-side hash cache
- Embedding (per Q20.2): real-time per query, no pre-built index
- LLM 4-method interface in DESIGN.md split into `Chatter` + `Embedder` for S2; `Streamer` + `Tooler` deferred to S5 (grill) and S7 (expand) respectively
- All provider errors wrapped with `fmt.Errorf("...: %w", err)` including the provider name and HTTP status when applicable
- Commit after every task

---

## File Structure

### Created in this plan

| Path | Responsibility |
|---|---|
| `internal/provider/llm/types.go` | `ChatRequest`, `ChatResponse`, `Message`, `EmbedRequest`, `EmbedResponse` types |
| `internal/provider/llm/interface.go` | `Chatter`, `Embedder` interfaces |
| `internal/provider/llm/errors.go` | `ErrLLMProvider`, `ErrNetwork`, sentinel error types + `ClassifyError` helper |
| `internal/provider/llm/mock/mock.go` | `MockProvider` for tests (scripted responses) |
| `internal/provider/llm/mock/mock_test.go` | Tests |
| `internal/provider/llm/retry.go` | `RetryWrapper` decorator |
| `internal/provider/llm/retry_test.go` | Tests |
| `internal/provider/llm/fallback.go` | `FallbackWrapper` decorator |
| `internal/provider/llm/fallback_test.go` | Tests |
| `internal/provider/llm/gemini/provider.go` | `Provider` implementing Chatter+Embedder via genai SDK |
| `internal/provider/llm/gemini/cache.go` | Gemini context cache helpers |
| `internal/provider/llm/gemini/provider_test.go` | Tests (mock HTTP/SDK where possible) |
| `internal/provider/llm/glm/client.go` | OpenAI-compatible HTTP client (reusable) |
| `internal/provider/llm/glm/provider.go` | `Provider` implementing Chatter+Embedder using the client |
| `internal/provider/llm/glm/provider_test.go` | Tests |
| `internal/provider/llm/factory.go` | `NewChatter(ref config.ModelRef, secrets) (Chatter, error)`, `NewEmbedder(...)` |
| `internal/provider/llm/factory_test.go` | Tests |
| `internal/provider/search/interface.go` | `Searcher`, `SearchOpts`, `SearchResult` |
| `internal/provider/search/errors.go` | Search-specific error types |
| `internal/provider/search/brave/brave.go` | Brave Search API impl |
| `internal/provider/search/brave/brave_test.go` | Tests |
| `internal/provider/search/serper/serper.go` | Serper.dev impl (fallback) |
| `internal/provider/search/serper/serper_test.go` | Tests |
| `internal/provider/search/factory.go` | `NewSearcher(name string, secrets)` |
| `internal/provider/reader/interface.go` | `Reader`, `Content` |
| `internal/provider/reader/jina/jina.go` | Jina Reader impl |
| `internal/provider/reader/jina/jina_test.go` | Tests |
| `internal/provider/reader/factory.go` | `NewReader(name string, secrets)` |

---

## Task 0: Dependencies + Skeleton

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: directory skeleton

**Interfaces:**
- Produces: importable packages with empty placeholders

- [ ] **Step 1: Add Google genai SDK dependency**

Run:
```bash
cd /Users/rong.zhu/Code/@zhurong/jianwu
go get google.golang.org/genai@latest
```

Expected: go.mod and go.sum updated.

- [ ] **Step 2: Create directory skeleton**

```bash
mkdir -p \
  internal/provider/llm \
  internal/provider/llm/mock \
  internal/provider/llm/gemini \
  internal/provider/llm/glm \
  internal/provider/search \
  internal/provider/search/brave \
  internal/provider/search/serper \
  internal/provider/reader \
  internal/provider/reader/jina
```

- [ ] **Step 3: Verify build still works**

```bash
go build ./...
```

Expected: no errors (no .go files yet in new dirs).

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(provider): add genai SDK dep and directory skeleton"
```

---

## Task 1: LLM Types + Interfaces

**Files:**
- Create: `internal/provider/llm/types.go`
- Create: `internal/provider/llm/interface.go`
- Create: `internal/provider/llm/errors.go`
- Create: `internal/provider/llm/interface_test.go`

**Interfaces:**
- Produces: `ChatRequest`, `ChatResponse`, `Message`, `EmbedRequest`, `EmbedResponse` types; `Chatter`, `Embedder` interfaces; `ErrLLMProvider`, `ErrNetwork`, `HTTPError`, `ClassifyError`

- [ ] **Step 1: Write `types.go`**

`internal/provider/llm/types.go`:

```go
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
```

- [ ] **Step 2: Write `interface.go`**

`internal/provider/llm/interface.go`:

```go
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
```

- [ ] **Step 3: Write `errors.go`**

`internal/provider/llm/errors.go`:

```go
package llm

import (
    "errors"
    "fmt"
    "net/http"
)

// Sentinel error categories. Used by Retry/Fallback wrappers to decide behavior.
var (
    // ErrNetwork = transient network failures (timeout, connection refused, DNS).
    // Triggers retry and fallback.
    ErrNetwork = errors.New("network error")
    // ErrLLMProvider = 4xx from the provider (auth, bad request, model not found).
    // Does NOT trigger retry or fallback (won't help).
    ErrLLMProvider = errors.New("llm provider error")
    // ErrRateLimit = 429. Triggers retry (with backoff) then fallback.
    ErrRateLimit = errors.New("rate limited")
    // ErrServer = 5xx from the provider. Triggers retry and fallback.
    ErrServer = errors.New("server error")
)

// HTTPError carries status code + body for diagnosis.
type HTTPError struct {
    Status  int
    Body    string
    Inner   error // wrapped sentinel (ErrNetwork / ErrLLMProvider / etc.)
}

func (e *HTTPError) Error() string {
    if e.Inner != nil {
        return fmt.Sprintf("%s: HTTP %d: %s", e.Inner, e.Status, truncate(e.Body, 200))
    }
    return fmt.Sprintf("HTTP %d: %s", e.Status, truncate(e.Body, 200))
}

func (e *HTTPError) Unwrap() error { return e.Inner }

func truncate(s string, n int) string {
    if len(s) <= n {
        return s
    }
    return s[:n] + "..."
}

// ClassifyError maps a raw error to one of the sentinel categories.
// Returns the sentinel (which the caller can errors.Is against) wrapped in a descriptive error.
func ClassifyError(err error, statusCode int) error {
    if err == nil {
        return nil
    }
    var sentinel error
    switch {
    case statusCode == 0:
        // No HTTP status: network/timeout/DNS.
        sentinel = ErrNetwork
    case statusCode == 429:
        sentinel = ErrRateLimit
    case statusCode >= 400 && statusCode < 500:
        sentinel = ErrLLMProvider
    case statusCode >= 500:
        sentinel = ErrServer
    default:
        sentinel = err
    }
    return &HTTPError{
        Status: statusCode,
        Body:   err.Error(),
        Inner:  sentinel,
    }
}
```

- [ ] **Step 4: Write failing test**

`internal/provider/llm/interface_test.go`:

```go
package llm

import (
    "context"
    "errors"
    "testing"
)

// stubChatter is a minimal Chatter for testing the interface contract.
type stubChatter struct {
    resp *ChatResponse
    err  error
}

func (s *stubChatter) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    return s.resp, s.err
}

func TestChatterInterface(t *testing.T) {
    var c Chatter = &stubChatter{
        resp: &ChatResponse{Content: "hello", TokensIn: 5, TokensOut: 1},
    }
    resp, err := c.Chat(context.Background(), ChatRequest{Model: "x"})
    if err != nil {
        t.Fatal(err)
    }
    if resp.Content != "hello" {
        t.Errorf("got %q, want hello", resp.Content)
    }
}

func TestClassifyErrorNetwork(t *testing.T) {
    err := ClassifyError(errors.New("connection refused"), 0)
    if !errors.Is(err, ErrNetwork) {
        t.Errorf("expected ErrNetwork, got %v", err)
    }
}

func TestClassifyErrorRateLimit(t *testing.T) {
    err := ClassifyError(errors.New("too many requests"), 429)
    if !errors.Is(err, ErrRateLimit) {
        t.Errorf("expected ErrRateLimit, got %v", err)
    }
}

func TestClassifyErrorClient(t *testing.T) {
    err := ClassifyError(errors.New("bad request"), 400)
    if !errors.Is(err, ErrLLMProvider) {
        t.Errorf("expected ErrLLMProvider, got %v", err)
    }
}

func TestClassifyErrorServer(t *testing.T) {
    err := ClassifyError(errors.New("internal"), 500)
    if !errors.Is(err, ErrServer) {
        t.Errorf("expected ErrServer, got %v", err)
    }
}
```

- [ ] **Step 5: Run tests, verify pass**

```bash
go test ./internal/provider/llm/... -v
```

Expected: PASS (no separate RED→GREEN since types compile from the start, but verify the interface contract works).

- [ ] **Step 6: Commit**

```bash
git add internal/provider/llm/
git commit -m "feat(llm): types, Chatter/Embedder interfaces, error classification"
```

---

## Task 2: Mock Provider

**Files:**
- Create: `internal/provider/llm/mock/mock.go`
- Create: `internal/provider/llm/mock/mock_test.go`

**Interfaces:**
- Consumes: `Chatter`, `Embedder`, `ChatRequest`, `ChatResponse` from Task 1
- Produces: `mock.Provider` (scripted responses for tests)

- [ ] **Step 1: Write failing test**

`internal/provider/llm/mock/mock_test.go`:

```go
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
```

- [ ] **Step 2: Run test, verify fail**

```bash
go test ./internal/provider/llm/mock/...
```

Expected: FAIL.

- [ ] **Step 3: Write `mock.go`**

`internal/provider/llm/mock/mock.go`:

```go
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
```

- [ ] **Step 4: Run tests, verify pass**

```bash
go test ./internal/provider/llm/mock/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/llm/mock/
git commit -m "feat(llm/mock): scripted MockProvider for tests"
```

---

## Task 3: Retry Wrapper

**Files:**
- Create: `internal/provider/llm/retry.go`
- Create: `internal/provider/llm/retry_test.go`

**Interfaces:**
- Consumes: `Chatter`, `Embedder`, error sentinels from Task 1
- Produces: `RetryWrapper` decorator that wraps a Chatter/Embedder

**Policy (per Q7):**
- Retry on: `ErrNetwork`, `ErrRateLimit`, `ErrServer` (transient)
- Do NOT retry on: `ErrLLMProvider` (4xx — won't help), context cancellation
- Max 3 attempts, exponential backoff: 1s × 2^attempt, ±20% jitter
- Backoff configurable via `RetryConfig`

- [ ] **Step 1: Write failing test**

`internal/provider/llm/retry_test.go`:

```go
package llm

import (
    "context"
    "errors"
    "testing"
    "time"
)

// fakeClock lets us skip backoff sleeps in tests.
type fakeClock struct{ t time.Duration }

func (c *fakeClock) Sleep(d time.Duration) { c.t += d }

func TestRetryWrapperSucceedsOnFirstTry(t *testing.T) {
    inner := &countingChatter{resp: &ChatResponse{Content: "ok"}}
    rw := &RetryWrapper{
        Inner:  inner,
        Config: RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond},
        clock:  &fakeClock{},
    }
    resp, err := rw.Chat(context.Background(), ChatRequest{})
    if err != nil {
        t.Fatal(err)
    }
    if resp.Content != "ok" {
        t.Errorf("got %q", resp.Content)
    }
    if inner.calls != 1 {
        t.Errorf("got %d calls, want 1", inner.calls)
    }
}

func TestRetryWrapperRetriesOnNetworkError(t *testing.T) {
    inner := &countingChatter{
        errs: []error{
            errors.Join(ErrNetwork, errors.New("conn refused")),
            errors.Join(ErrNetwork, errors.New("conn refused")),
            nil, // third succeeds
        },
        resp: &ChatResponse{Content: "finally"},
    }
    rw := &RetryWrapper{
        Inner:  inner,
        Config: RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond},
        clock:  &fakeClock{},
    }
    resp, err := rw.Chat(context.Background(), ChatRequest{})
    if err != nil {
        t.Fatalf("expected success after retries, got %v", err)
    }
    if resp.Content != "finally" {
        t.Errorf("got %q", resp.Content)
    }
    if inner.calls != 3 {
        t.Errorf("got %d calls, want 3", inner.calls)
    }
}

func TestRetryWrapperDoesNotRetryOn4xx(t *testing.T) {
    inner := &countingChatter{
        errs: []error{errors.Join(ErrLLMProvider, errors.New("bad request"))},
        resp: &ChatResponse{},
    }
    rw := &RetryWrapper{
        Inner:  inner,
        Config: RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond},
        clock:  &fakeClock{},
    }
    _, err := rw.Chat(context.Background(), ChatRequest{})
    if err == nil {
        t.Fatal("expected error")
    }
    if inner.calls != 1 {
        t.Errorf("got %d calls, want 1 (no retry on 4xx)", inner.calls)
    }
}

func TestRetryWrapperGivesUpAfterMaxAttempts(t *testing.T) {
    inner := &countingChatter{
        errs: []error{
            errors.Join(ErrServer, errors.New("500")),
            errors.Join(ErrServer, errors.New("500")),
            errors.Join(ErrServer, errors.New("500")),
        },
        resp: &ChatResponse{},
    }
    rw := &RetryWrapper{
        Inner:  inner,
        Config: RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond},
        clock:  &fakeClock{},
    }
    _, err := rw.Chat(context.Background(), ChatRequest{})
    if err == nil {
        t.Fatal("expected error")
    }
    if !errors.Is(err, ErrServer) {
        t.Errorf("expected ErrServer, got %v", err)
    }
    if inner.calls != 3 {
        t.Errorf("got %d calls, want 3", inner.calls)
    }
}

// countingChatter is a test Chatter that returns errors/responses in sequence.
type countingChatter struct {
    errs  []error
    resp  *ChatResponse
    calls int
}

func (c *countingChatter) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    defer func() { c.calls++ }()
    if c.calls < len(c.errs) {
        err := c.errs[c.calls]
        if err != nil {
            return nil, err
        }
    }
    return c.resp, nil
}
```

- [ ] **Step 2: Run test, verify fail**

```bash
go test ./internal/provider/llm/... -run TestRetryWrapper
```

Expected: FAIL (undefined RetryWrapper).

- [ ] **Step 3: Write `retry.go`**

`internal/provider/llm/retry.go`:

```go
package llm

import (
    "context"
    "errors"
    "fmt"
    "math/rand"
    "time"
)

// RetryConfig controls retry behavior.
type RetryConfig struct {
    MaxAttempts int           // total attempts including the first; default 3
    BaseDelay   time.Duration // base for exponential backoff; default 1s
    MaxDelay    time.Duration // cap per delay; default 30s
    Jitter      float64       // ±fraction; default 0.2 (20%)
}

// DefaultRetryConfig matches Q7 decision: 3 attempts, 1s base, 30s cap, 20% jitter.
var DefaultRetryConfig = RetryConfig{
    MaxAttempts: 3,
    BaseDelay:   1 * time.Second,
    MaxDelay:    30 * time.Second,
    Jitter:      0.2,
}

// clock is a tiny indirection so tests can skip real Sleep.
type clock struct{}

func (clock) Sleep(d time.Duration) { time.Sleep(d) }

// RetryWrapper decorates a Chatter with retry on transient errors.
type RetryWrapper struct {
    Inner  Chatter
    Config RetryConfig
    clock  interface{ Sleep(time.Duration) } // injected for tests
}

// NewRetryWrapper constructs a RetryWrapper with default config and real clock.
func NewRetryWrapper(inner Chatter) *RetryWrapper {
    return &RetryWrapper{Inner: inner, Config: DefaultRetryConfig, clock: clock{}}
}

// shouldRetry reports whether the error is retryable per Q7.
func shouldRetry(err error) bool {
    if err == nil {
        return false
    }
    if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
        return false // user cancelled or timed out — don't retry
    }
    if errors.Is(err, ErrNetwork) || errors.Is(err, ErrRateLimit) || errors.Is(err, ErrServer) {
        return true
    }
    return false // 4xx and unknown errors don't retry
}

func (rw *RetryWrapper) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    cfg := rw.Config
    if cfg.MaxAttempts == 0 {
        cfg = DefaultRetryConfig
    }
    clk := rw.clock
    if clk == nil {
        clk = clock{}
    }
    var lastErr error
    for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
        resp, err := rw.Inner.Chat(ctx, req)
        if err == nil {
            return resp, nil
        }
        lastErr = err
        if !shouldRetry(err) {
            return nil, err
        }
        if attempt == cfg.MaxAttempts-1 {
            break // no more attempts
        }
        delay := backoff(cfg, attempt)
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }
        clk.Sleep(delay)
    }
    return nil, fmt.Errorf("retry exhausted after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// backoff returns delay = BaseDelay * 2^attempt, capped at MaxDelay, with ±Jitter.
func backoff(cfg RetryConfig, attempt int) time.Duration {
    d := cfg.BaseDelay
    for i := 0; i < attempt && d < cfg.MaxDelay; i++ {
        d *= 2
    }
    if d > cfg.MaxDelay {
        d = cfg.MaxDelay
    }
    if cfg.Jitter > 0 {
        delta := float64(d) * cfg.Jitter
        d = time.Duration(float64(d) + (rand.Float64()*2-1)*delta)
    }
    return d
}
```

- [ ] **Step 4: Run tests, verify pass**

```bash
go test ./internal/provider/llm/... -run TestRetryWrapper -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/llm/retry.go internal/provider/llm/retry_test.go
git commit -m "feat(llm): retry wrapper with exp backoff + jitter (Q7 policy)"
```

---

## Task 4: Fallback Wrapper

**Files:**
- Create: `internal/provider/llm/fallback.go`
- Create: `internal/provider/llm/fallback_test.go`

**Interfaces:**
- Consumes: `Chatter`, error sentinels, `RetryWrapper` from Task 3
- Produces: `FallbackWrapper` that tries Primary then Fallback

**Policy:** Primary fails (after its own retry exhaustion, if wrapped) → switch to Fallback. Both must fail before FallbackWrapper reports failure.

- [ ] **Step 1: Write failing test**

`internal/provider/llm/fallback_test.go`:

```go
package llm

import (
    "context"
    "errors"
    "testing"
)

func TestFallbackUsesPrimaryOnSuccess(t *testing.T) {
    primary := &countingChatter{resp: &ChatResponse{Content: "from-primary"}}
    fallback := &countingChatter{resp: &ChatResponse{Content: "from-fallback"}}
    fw := &FallbackWrapper{Primary: primary, Fallback: fallback}
    resp, err := fw.Chat(context.Background(), ChatRequest{})
    if err != nil {
        t.Fatal(err)
    }
    if resp.Content != "from-primary" {
        t.Errorf("got %q, want from-primary", resp.Content)
    }
    if fallback.calls != 0 {
        t.Errorf("fallback should not have been called")
    }
}

func TestFallbackSwitchesToFallbackOnPrimaryFailure(t *testing.T) {
    primary := &countingChatter{
        errs: []error{errors.Join(ErrServer, errors.New("500"))},
        resp: &ChatResponse{},
    }
    fallback := &countingChatter{resp: &ChatResponse{Content: "from-fallback"}}
    fw := &FallbackWrapper{Primary: primary, Fallback: fallback}
    resp, err := fw.Chat(context.Background(), ChatRequest{})
    if err != nil {
        t.Fatal(err)
    }
    if resp.Content != "from-fallback" {
        t.Errorf("got %q, want from-fallback", resp.Content)
    }
    if primary.calls != 1 {
        t.Errorf("primary calls: %d", primary.calls)
    }
    if fallback.calls != 1 {
        t.Errorf("fallback calls: %d", fallback.calls)
    }
}

func TestFallbackReturnsLastErrorWhenBothFail(t *testing.T) {
    primary := &countingChatter{
        errs: []error{errors.Join(ErrServer, errors.New("primary 500"))},
        resp: &ChatResponse{},
    }
    fallback := &countingChatter{
        errs: []error{errors.Join(ErrServer, errors.New("fallback 500"))},
        resp: &ChatResponse{},
    }
    fw := &FallbackWrapper{Primary: primary, Fallback: fallback}
    _, err := fw.Chat(context.Background(), ChatRequest{})
    if err == nil {
        t.Fatal("expected error")
    }
    if !errors.Is(err, ErrServer) {
        t.Errorf("expected ErrServer, got %v", err)
    }
}

// countingChatter here intentionally duplicates the one in retry_test.go for clarity.
// To avoid duplicate-decl in same package, remove this comment and use the existing one.
// If the test file fails to compile due to duplicate countingChatter, delete the
// duplicate here and rely on the definition in retry_test.go (same package).
```

**Note:** Since both `retry_test.go` and `fallback_test.go` are in `package llm`, only ONE `countingChatter` definition is needed across the package's test files. If you copy the struct here, the compiler will report a duplicate-decl error. Either (a) delete the duplicate from `fallback_test.go` and rely on the one in `retry_test.go`, or (b) move `countingChatter` to a shared `testhelpers_test.go` file.

**Recommended:** Create `internal/provider/llm/testhelpers_test.go` with the `countingChatter` and `fakeClock` types, and remove the duplicates from the other test files. This avoids the duplicate-decl pitfall.

- [ ] **Step 2: Resolve the test helper situation**

Before writing fallback.go, refactor: create `internal/provider/llm/testhelpers_test.go`:

```go
package llm

import (
    "context"
    "time"
)

// countingChatter is a test Chatter that returns errors/responses in sequence.
type countingChatter struct {
    errs  []error
    resp  *ChatResponse
    calls int
}

func (c *countingChatter) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    defer func() { c.calls++ }()
    if c.calls < len(c.errs) {
        if err := c.errs[c.calls]; err != nil {
            return nil, err
        }
    }
    return c.resp, nil
}

// fakeClock lets us skip backoff sleeps in tests.
type fakeClock struct{ t time.Duration }

func (c *fakeClock) Sleep(d time.Duration) { c.t += d }
```

Then DELETE the duplicate `countingChatter` and `fakeClock` definitions from `retry_test.go` (and from `fallback_test.go`'s planned content).

- [ ] **Step 3: Write fallback_test.go (without duplicate helpers)**

`internal/provider/llm/fallback_test.go`:

```go
package llm

import (
    "context"
    "errors"
    "testing"
)

func TestFallbackUsesPrimaryOnSuccess(t *testing.T) {
    primary := &countingChatter{resp: &ChatResponse{Content: "from-primary"}}
    fallback := &countingChatter{resp: &ChatResponse{Content: "from-fallback"}}
    fw := &FallbackWrapper{Primary: primary, Fallback: fallback}
    resp, err := fw.Chat(context.Background(), ChatRequest{})
    if err != nil {
        t.Fatal(err)
    }
    if resp.Content != "from-primary" {
        t.Errorf("got %q", resp.Content)
    }
    if fallback.calls != 0 {
        t.Errorf("fallback should not be called")
    }
}

func TestFallbackSwitchesOnPrimaryFailure(t *testing.T) {
    primary := &countingChatter{
        errs: []error{errors.Join(ErrServer, errors.New("500"))},
        resp: &ChatResponse{},
    }
    fallback := &countingChatter{resp: &ChatResponse{Content: "from-fallback"}}
    fw := &FallbackWrapper{Primary: primary, Fallback: fallback}
    resp, err := fw.Chat(context.Background(), ChatRequest{})
    if err != nil {
        t.Fatal(err)
    }
    if resp.Content != "from-fallback" {
        t.Errorf("got %q", resp.Content)
    }
}

func TestFallbackReturnsErrorWhenBothFail(t *testing.T) {
    primary := &countingChatter{
        errs: []error{errors.Join(ErrServer, errors.New("p 500"))},
        resp: &ChatResponse{},
    }
    fallback := &countingChatter{
        errs: []error{errors.Join(ErrServer, errors.New("f 500"))},
        resp: &ChatResponse{},
    }
    fw := &FallbackWrapper{Primary: primary, Fallback: fallback}
    _, err := fw.Chat(context.Background(), ChatRequest{})
    if err == nil {
        t.Fatal("expected error")
    }
    if !errors.Is(err, ErrServer) {
        t.Errorf("got %v", err)
    }
}
```

- [ ] **Step 4: Run tests, verify fail**

```bash
go test ./internal/provider/llm/... -run TestFallback
```

Expected: FAIL (FallbackWrapper undefined).

- [ ] **Step 5: Write `fallback.go`**

`internal/provider/llm/fallback.go`:

```go
package llm

import (
    "context"
    "fmt"
)

// FallbackWrapper tries Primary first; on any non-nil error, tries Fallback.
// If both fail, returns the fallback's error (last attempt).
// Wrap Primary and Fallback in RetryWrapper if you want retry-then-fallback per Q7.
type FallbackWrapper struct {
    Primary  Chatter
    Fallback Chatter
}

func (fw *FallbackWrapper) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    resp, err := fw.Primary.Chat(ctx, req)
    if err == nil {
        return resp, nil
    }
    // Primary failed. Try fallback.
    primaryErr := err
    resp2, err2 := fw.Fallback.Chat(ctx, req)
    if err2 == nil {
        return resp2, nil
    }
    return nil, fmt.Errorf("primary: %v; fallback: %w", primaryErr, err2)
}
```

- [ ] **Step 6: Run tests, verify pass**

```bash
go test ./internal/provider/llm/... -v
```

Expected: all tests pass (retry + fallback).

- [ ] **Step 7: Commit**

```bash
git add internal/provider/llm/testhelpers_test.go internal/provider/llm/fallback.go internal/provider/llm/fallback_test.go internal/provider/llm/retry_test.go
git commit -m "feat(llm): fallback wrapper + shared test helpers"
```

---

## Task 5: GLM Provider (Direct REST, OpenAI-compatible)

**Files:**
- Create: `internal/provider/llm/glm/client.go`
- Create: `internal/provider/llm/glm/provider.go`
- Create: `internal/provider/llm/glm/provider_test.go`

**Why GLM before Gemini:** GLM is plain HTTP — simpler to implement first, and it establishes the `client.go` pattern that can be reused for Qwen/Moonshot/DeepSeek later. Gemini SDK comes in Task 6.

**Interfaces:**
- Consumes: `Chatter`, `Embedder`, `ChatRequest`, `ChatResponse`, `ClassifyError` from Task 1
- Produces: `glm.Provider`, `glm.Config`, `glm.New(cfg Config)`

**API spec (智谱 BigModel):**
- Endpoint: `https://open.bigmodel.cn/api/paas/v4/chat/completions`
- Embedding: `https://open.bigmodel.cn/api/paas/v4/embeddings`
- Auth: `Bearer <API key>`
- Request/response shape: OpenAI-compatible (same fields, different model IDs like `glm-4.6`)

- [ ] **Step 1: Write failing test (HTTP server stub)**

`internal/provider/llm/glm/provider_test.go`:

```go
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
```

- [ ] **Step 2: Run tests, verify fail**

```bash
go test ./internal/provider/llm/glm/...
```

Expected: FAIL.

- [ ] **Step 3: Write `client.go`**

`internal/provider/llm/glm/client.go`:

```go
package glm

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

// client is a thin HTTP wrapper over an OpenAI-compatible chat/embeddings API.
// Reusable for GLM, Qwen, Moonshot, DeepSeek, etc. — anything that mirrors OpenAI's shape.
type client struct {
    baseURL string
    apiKey  string
    http    *http.Client
}

func newClient(baseURL, apiKey string) *client {
    return &client{
        baseURL: baseURL,
        apiKey:  apiKey,
        http:    &http.Client{Timeout: 60 * time.Second},
    }
}

// post sends a POST with Bearer auth and JSON body, returns the raw response.
// Caller is responsible for closing the body.
func (c *client) post(ctx context.Context, path string, body any) (*http.Response, error) {
    data, err := json.Marshal(body)
    if err != nil {
        return nil, fmt.Errorf("marshal request: %w", err)
    }
    req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(data))
    if err != nil {
        return nil, fmt.Errorf("build request: %w", err)
    }
    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    req.Header.Set("Content-Type", "application/json")
    resp, err := c.http.Do(req)
    if err != nil {
        return nil, fmt.Errorf("http: %w", err)
    }
    return resp, nil
}

func decodeJSON(r io.Reader, v any) error {
    d := json.NewDecoder(r)
    return d.Decode(v)
}
```

- [ ] **Step 4: Write `provider.go`**

`internal/provider/llm/glm/provider.go`:

```go
package glm

import (
    "context"
    "fmt"
    "io"

    "github.com/zhurong/jianwu/internal/provider/llm"
)

// DefaultBaseURL is the GLM (智谱 BigModel) endpoint.
const DefaultBaseURL = "https://open.bigmodel.cn/api/paas/v4"

// Config configures a GLM provider.
type Config struct {
    APIKey  string
    BaseURL string // defaults to DefaultBaseURL if empty
}

// Provider implements llm.Chatter and llm.Embedder via GLM's OpenAI-compatible API.
type Provider struct {
    c *client
}

// New constructs a GLM Provider.
func New(cfg Config) (*Provider, error) {
    if cfg.APIKey == "" {
        return nil, fmt.Errorf("glm: APIKey is required")
    }
    if cfg.BaseURL == "" {
        cfg.BaseURL = DefaultBaseURL
    }
    return &Provider{c: newClient(cfg.BaseURL, cfg.APIKey)}, nil
}

// Chat calls GLM's /chat/completions endpoint.
func (p *Provider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
    body := map[string]any{
        "model":    req.Model,
        "messages": req.Messages,
    }
    if req.Temperature != nil {
        body["temperature"] = *req.Temperature
    }
    if req.MaxTokens > 0 {
        body["max_tokens"] = req.MaxTokens
    }
    if len(req.JSONSchema) > 0 {
        body["response_format"] = map[string]any{
            "type": "json_schema",
            "json_schema": map[string]any{
                "name": "response",
                "schema": json.RawMessage(req.JSONSchema),
            },
        }
    }

    resp, err := p.c.post(ctx, "/chat/completions", body)
    if err != nil {
        return nil, llm.ClassifyError(err, 0)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        b, _ := io.ReadAll(resp.Body)
        return nil, llm.ClassifyError(fmt.Errorf("glm: %s", string(b)), resp.StatusCode)
    }

    var out chatCompletionResponse
    if err := decodeJSON(resp.Body, &out); err != nil {
        return nil, fmt.Errorf("glm: decode response: %w", err)
    }
    if len(out.Choices) == 0 {
        return nil, fmt.Errorf("glm: empty choices in response")
    }
    return &llm.ChatResponse{
        Content:      out.Choices[0].Message.Content,
        FinishReason: out.Choices[0].FinishReason,
        TokensIn:     out.Usage.PromptTokens,
        TokensOut:    out.Usage.CompletionTokens,
    }, nil
}

// Embed calls GLM's /embeddings endpoint.
func (p *Provider) Embed(ctx context.Context, req llm.EmbedRequest) (*llm.EmbedResponse, error) {
    body := map[string]any{
        "model": req.Model,
        "input": req.Inputs,
    }
    resp, err := p.c.post(ctx, "/embeddings", body)
    if err != nil {
        return nil, llm.ClassifyError(err, 0)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        b, _ := io.ReadAll(resp.Body)
        return nil, llm.ClassifyError(fmt.Errorf("glm: %s", string(b)), resp.StatusCode)
    }
    var out embeddingsResponse
    if err := decodeJSON(resp.Body, &out); err != nil {
        return nil, fmt.Errorf("glm: decode embeddings: %w", err)
    }
    return &llm.EmbedResponse{
        Embeddings: out.embeddings(),
        TokensIn:   out.Usage.PromptTokens,
    }, nil
}

// OpenAI-compatible response shapes.

type chatCompletionResponse struct {
    Choices []struct {
        Message struct {
            Role    string `json:"role"`
            Content string `json:"content"`
        } `json:"message"`
        FinishReason string `json:"finish_reason"`
    } `json:"choices"`
    Usage struct {
        PromptTokens     int `json:"prompt_tokens"`
        CompletionTokens int `json:"completion_tokens"`
    } `json:"usage"`
}

type embeddingsResponse struct {
    Data []struct {
        Embedding []float32 `json:"embedding"`
    } `json:"data"`
    Usage struct {
        PromptTokens int `json:"prompt_tokens"`
    } `json:"usage"`
}

func (r *embeddingsResponse) embeddings() [][]float32 {
    out := make([][]float32, len(r.Data))
    for i, d := range r.Data {
        out[i] = d.Embedding
    }
    return out
}
```

**Note:** `json.RawMessage` requires `encoding/json` import. Add it to the imports.

- [ ] **Step 5: Run tests, verify pass**

```bash
go test ./internal/provider/llm/glm/... -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/llm/glm/
git commit -m "feat(llm/glm): GLM provider via OpenAI-compatible REST (chat + embed)"
```

---

## Task 6: Gemini Provider (Official SDK + Context Cache)

**Files:**
- Create: `internal/provider/llm/gemini/provider.go`
- Create: `internal/provider/llm/gemini/cache.go`
- Create: `internal/provider/llm/gemini/provider_test.go`

**Interfaces:**
- Consumes: `Chatter`, `Embedder`, types from Task 1
- Produces: `gemini.Provider`, `gemini.Config`, `gemini.New(cfg Config)`

**SDK:** `google.golang.org/genai` (added in Task 0). Official Google SDK supports `responseMimeType`, `responseSchema`, `cached_content` for context cache, and `embedding` endpoint.

- [ ] **Step 1: Inspect SDK shape**

```bash
go doc google.golang.org/genai.Client 2>&1 | head -30
go doc google.golang.org/genai.Models.GenerateContent 2>&1 | head -20
```

Familiarize with: `genai.NewClient(ctx, Config)`, `client.Models.GenerateContent(ctx, *GenerateContentRequest)`, `client.Models.EmbedContent(ctx, *EmbedContentRequest)`.

- [ ] **Step 2: Write failing test**

`internal/provider/llm/gemini/provider_test.go`:

```go
package gemini

import (
    "context"
    "testing"

    "github.com/zhurong/jianwu/internal/provider/llm"
)

// Tests against the real SDK would require a live API key, so we test only
// the construction + config plumbing. Real API behavior is covered by
// integration tests that skip when GEMINI_API_KEY is unset.
//
// The Provider.Chat / Provider.Embed methods will be exercised in integration
// tests run manually before S2 release.

func TestNewRequiresAPIKey(t *testing.T) {
    _, err := New(Config{APIKey: ""})
    if err == nil {
        t.Error("expected error on empty API key")
    }
}

func TestNewConstructsWithKey(t *testing.T) {
    // We don't actually call the SDK here; we just verify construction.
    // Real client init happens lazily on first call (or we accept the offline init).
    p, err := New(Config{APIKey: "fake-key"})
    if err != nil {
        t.Fatal(err)
    }
    if p == nil {
        t.Fatal("nil provider")
    }
}

// TestProviderChatWithLiveKey is an integration test that runs only when
// GEMINI_API_KEY is set. Skip otherwise.
func TestProviderChatWithLiveKey(t *testing.T) {
    key := apiKeyFromEnv()
    if key == "" {
        t.Skip("GEMINI_API_KEY not set; skipping live test")
    }
    p, err := New(Config{APIKey: key})
    if err != nil {
        t.Fatal(err)
    }
    resp, err := p.Chat(context.Background(), llm.ChatRequest{
        Model:    "gemini-2.5-flash",
        Messages: []llm.Message{{Role: "user", Content: "Say 'hello' and nothing else."}},
    })
    if err != nil {
        t.Fatalf("Chat: %v", err)
    }
    if resp.Content == "" {
        t.Error("empty content")
    }
    t.Logf("response: %q (tokens in=%d out=%d)", resp.Content, resp.TokensIn, resp.TokensOut)
}
```

- [ ] **Step 3: Run tests, verify result**

```bash
go test ./internal/provider/llm/gemini/... -v
```

Expected: TestNewRequiresAPIKey and TestNewConstructsWithKey pass. TestProviderChatWithLiveKey skips if no API key set.

- [ ] **Step 4: Write `provider.go`**

`internal/provider/llm/gemini/provider.go`:

```go
package gemini

import (
    "context"
    "fmt"
    "os"

    "github.com/zhurong/jianwu/internal/provider/llm"
    "google.golang.org/genai"
)

// Config configures a Gemini provider.
type Config struct {
    APIKey string
}

// Provider implements llm.Chatter and llm.Embedder via Google's official genai SDK.
type Provider struct {
    client *genai.Client
}

// New constructs a Gemini Provider. The genai.Client is initialized eagerly
// to validate the API key against Google's backend.
func New(cfg Config) (*Provider, error) {
    if cfg.APIKey == "" {
        return nil, fmt.Errorf("gemini: APIKey is required")
    }
    client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
        APIKey:  cfg.APIKey,
        Backend: genai.BackendGeminiAPI,
    })
    if err != nil {
        return nil, fmt.Errorf("gemini: init client: %w", err)
    }
    return &Provider{client: client}, nil
}

// Chat calls Gemini's GenerateContent endpoint.
func (p *Provider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
    contents := make([]*genai.Content, 0, len(req.Messages))
    for _, m := range req.Messages {
        role := m.Role
        if role == "assistant" {
            role = "model" // Gemini uses "model" not "assistant"
        }
        contents = append(contents, &genai.Content{
            Role:  role,
            Parts: []*genai.Part{{Text: m.Content}},
        })
    }
    config := &genai.GenerateContentConfig{}
    if req.Temperature != nil {
        config.Temperature = req.Temperature
    }
    if req.MaxTokens > 0 {
        config.MaxOutputTokens = int64(req.MaxTokens)
    }
    if len(req.JSONSchema) > 0 {
        config.ResponseMIMEType = "application/json"
        config.ResponseSchema = &genai.Schema{} // populated from req.JSONSchema; see schemaFromRaw
        if err := schemaFromRaw(req.JSONSchema, config.ResponseSchema); err != nil {
            return nil, fmt.Errorf("gemini: parse JSON schema: %w", err)
        }
    }
    resp, err := p.client.Models.GenerateContent(ctx, req.Model, contents, config)
    if err != nil {
        return nil, llm.ClassifyError(err, 0) // SDK errors are network/API; status mapping is approximate
    }
    out := &llm.ChatResponse{}
    if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
        if text := resp.Candidates[0].Content.Parts[0].Text; text != "" {
            out.Content = text
        }
        out.FinishReason = string(resp.Candidates[0].FinishReason)
    }
    if resp.UsageMetadata != nil {
        out.TokensIn = int(resp.UsageMetadata.PromptTokenCount)
        out.TokensOut = int(resp.UsageMetadata.CandidatesTokenCount)
    }
    return out, nil
}

// Embed calls Gemini's embedContent endpoint.
func (p *Provider) Embed(ctx context.Context, req llm.EmbedRequest) (*llm.EmbedResponse, error) {
    out := &llm.EmbedResponse{Embeddings: make([][]float32, 0, len(req.Inputs))}
    for _, input := range req.Inputs {
        resp, err := p.client.Models.EmbedContent(ctx, req.Model, &genai.EmbedContentConfig{
            TaskType: "RETRIEVAL_DOCUMENT",
        }, input)
        if err != nil {
            return nil, llm.ClassifyError(err, 0)
        }
        if resp.Embeddings != nil && len(resp.Embeddings.Values) > 0 {
            out.Embeddings = append(out.Embeddings, resp.Embeddings.Values)
        }
        // Gemini doesn't return per-call token count in EmbedContent response.
    }
    return out, nil
}

// schemaFromRaw populates a genai.Schema from a JSON Schema byte slice.
// For S2 we only support a minimal subset (type, properties). Full JSON Schema
// translation will land in S3 (outline) when structured outputs are first used.
func schemaFromRaw(raw []byte, s *genai.Schema) error {
    // Minimal: treat as a free-form object. Full impl deferred to S3.
    s.Type = "OBJECT"
    return nil
}

// apiKeyFromEnv reads the Gemini API key from environment (for tests).
func apiKeyFromEnv() string {
    return os.Getenv("GEMINI_API_KEY")
}
```

- [ ] **Step 5: Write `cache.go` (placeholder for S3+ integration)**

`internal/provider/llm/gemini/cache.go`:

```go
package gemini

import (
    "context"

    "google.golang.org/genai"
)

// CreateCache creates a context cache for a shared system prompt.
// Returns the cache name to use in subsequent GenerateContent calls.
//
// Usage (when wired into engine in S3+):
//   cacheName, _ := gemini.CreateCache(ctx, client, "gemini-2.5-pro", systemPrompt)
//   config.CachedContent = cacheName
//   resp, _ := client.Models.GenerateContent(ctx, model, contents, config)
//
// For S2 this is a helper that downstream tasks can use; it's not yet wired
// into Chat because we don't know the cache granularity (per-stage? per-book?).
func CreateCache(ctx context.Context, client *genai.Client, model, systemPrompt string) (string, error) {
    resp, err := client.Caches.Create(ctx, model, &genai.CreateCachedContentConfig{
        CachedContent: &genai.CachedContent{
            Contents: []*genai.Content{
                {Role: "user", Parts: []*genai.Part{{Text: systemPrompt}}},
            },
        },
    })
    if err != nil {
        return "", err
    }
    return resp.Name, nil
}
```

- [ ] **Step 6: Run tests**

```bash
go test ./internal/provider/llm/gemini/... -v
```

Expected: TestNewRequiresAPIKey + TestNewConstructsWithKey PASS. TestProviderChatWithLiveKey SKIPs without key.

- [ ] **Step 7: Commit**

```bash
git add internal/provider/llm/gemini/
git commit -m "feat(llm/gemini): provider via genai SDK + context cache helper"
```

---

## Task 7: LLM Factory

**Files:**
- Create: `internal/provider/llm/factory.go`
- Create: `internal/provider/llm/factory_test.go`

**Interfaces:**
- Consumes: `config.ModelRef`, `config.Secrets` from S1; `gemini.New`, `glm.New` from Tasks 5-6
- Produces: `llm.NewChatter(ref config.ModelRef, secrets *config.Secrets) (Chatter, error)`, `llm.NewEmbedder(...)`

- [ ] **Step 1: Write failing test**

`internal/provider/llm/factory_test.go`:

```go
package llm

import (
    "testing"

    "github.com/zhurong/jianwu/internal/config"
)

func TestNewChatterGemini(t *testing.T) {
    secrets := &config.Secrets{GeminiAPIKey: "fake"}
    _, err := NewChatter(config.ModelRef{Provider: "gemini", Model: "gemini-2.5-pro"}, secrets)
    if err != nil {
        t.Fatalf("gemini: %v", err)
    }
}

func TestNewChatterGLM(t *testing.T) {
    secrets := &config.Secrets{GLMAPIKey: "fake"}
    _, err := NewChatter(config.ModelRef{Provider: "glm", Model: "glm-4.6"}, secrets)
    if err != nil {
        t.Fatalf("glm: %v", err)
    }
}

func TestNewChatterUnknownProviderErrors(t *testing.T) {
    _, err := NewChatter(config.ModelRef{Provider: "unknown", Model: "x"}, &config.Secrets{})
    if err == nil {
        t.Error("expected error")
    }
}

func TestNewChatterMissingKeyErrors(t *testing.T) {
    _, err := NewChatter(config.ModelRef{Provider: "gemini", Model: "x"}, &config.Secrets{})
    if err == nil {
        t.Error("expected error for missing Gemini key")
    }
}
```

- [ ] **Step 2: Write `factory.go`**

`internal/provider/llm/factory.go`:

```go
package llm

import (
    "fmt"

    "github.com/zhurong/jianwu/internal/config"
    "github.com/zhurong/jianwu/internal/provider/llm/gemini"
    "github.com/zhurong/jianwu/internal/provider/llm/glm"
)

// NewChatter constructs a Chatter for the given provider/model.
// For S2: returns the bare provider (no retry/fallback wrapping yet).
// Engine layer in S3+ will wrap with RetryWrapper and FallbackWrapper per config.
func NewChatter(ref config.ModelRef, secrets *config.Secrets) (Chatter, error) {
    switch ref.Provider {
    case "gemini":
        if secrets.GeminiAPIKey == "" {
            return nil, fmt.Errorf("gemini provider requires GEMINI_API_KEY")
        }
        return gemini.New(gemini.Config{APIKey: secrets.GeminiAPIKey})
    case "glm":
        if secrets.GLMAPIKey == "" {
            return nil, fmt.Errorf("glm provider requires GLM_API_KEY")
        }
        return glm.New(glm.Config{APIKey: secrets.GLMAPIKey})
    default:
        return nil, fmt.Errorf("unknown LLM provider: %q", ref.Provider)
    }
}

// NewEmbedder constructs an Embedder. Same switch as Chatter since both providers
// implement both interfaces.
func NewEmbedder(ref config.ModelRef, secrets *config.Secrets) (Embedder, error) {
    switch ref.Provider {
    case "gemini":
        if secrets.GeminiAPIKey == "" {
            return nil, fmt.Errorf("gemini provider requires GEMINI_API_KEY")
        }
        return gemini.New(gemini.Config{APIKey: secrets.GeminiAPIKey})
    case "glm":
        if secrets.GLMAPIKey == "" {
            return nil, fmt.Errorf("glm provider requires GLM_API_KEY")
        }
        return glm.New(glm.Config{APIKey: secrets.GLMAPIKey})
    default:
        return nil, fmt.Errorf("unknown LLM provider: %q", ref.Provider)
    }
}
```

- [ ] **Step 3: Run tests, verify pass**

```bash
go test ./internal/provider/llm/... -v
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/provider/llm/factory.go internal/provider/llm/factory_test.go
git commit -m "feat(llm): factory wiring provider name -> impl, key from secrets"
```

---

## Task 8: Search Interface + Errors

**Files:**
- Create: `internal/provider/search/interface.go`
- Create: `internal/provider/search/errors.go`

**Interfaces:**
- Produces: `Searcher`, `SearchOpts`, `SearchResult`, `ErrSearchProvider`

- [ ] **Step 1: Write `interface.go`**

`internal/provider/search/interface.go`:

```go
package search

import (
    "context"
    "time"
)

// Searcher is the web-search interface. Implementations: brave.Searcher, serper.Searcher.
type Searcher interface {
    Search(ctx context.Context, query string, opts SearchOpts) ([]SearchResult, error)
}

// SearchOpts controls a single search query.
type SearchOpts struct {
    MaxResults int           // default 10
    TimeRange  TimeRange     // "any" (default) | "past_day" | "past_week" | "past_month" | "past_year"
    Language   string        // BCP-47 like "zh-CN", "en-US"; empty = no filter
}

// TimeRange is an enum for SearchOpts.TimeRange.
type TimeRange string

const (
    TimeAny   TimeRange = "any"
    TimeDay   TimeRange = "past_day"
    TimeWeek  TimeRange = "past_week"
    TimeMonth TimeRange = "past_month"
    TimeYear  TimeRange = "past_year"
)

// SearchResult is one hit from a search query.
type SearchResult struct {
    Title   string
    URL     string
    Snippet string
    // PublishedAt is set when the provider returns it (Brave does; Serper doesn't).
    PublishedAt *time.Time
}
```

- [ ] **Step 2: Write `errors.go`**

`internal/provider/search/errors.go`:

```go
package search

import "errors"

// ErrSearchProvider is returned for 4xx responses from a search API.
// Does NOT trigger retry (won't help).
var ErrSearchProvider = errors.New("search provider error")

// ErrSearchRateLimit is returned for 429. Triggers retry/fallback.
var ErrSearchRateLimit = errors.New("search rate limited")

// ErrSearchNetwork is returned for network/timeout errors. Triggers retry/fallback.
var ErrSearchNetwork = errors.New("search network error")
```

- [ ] **Step 3: Verify it compiles**

```bash
go build ./internal/provider/search/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/provider/search/
git commit -m "feat(search): interface, opts, result, error sentinels"
```

---

## Task 9: Brave Search Provider

**Files:**
- Create: `internal/provider/search/brave/brave.go`
- Create: `internal/provider/search/brave/brave_test.go`

**API spec:**
- Endpoint: `https://api.search.brave.com/res/v1/web/search`
- Auth: `X-Subscription-Token: <key>`
- Query params: `q`, `count`, `country`, `search_lang`, `freshness` (pd/pw/pm/py)

- [ ] **Step 1: Write failing test (with httptest)**

`internal/provider/search/brave/brave_test.go`:

```go
package brave

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/zhurong/jianwu/internal/provider/search"
)

func TestSearchSuccess(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("X-Subscription-Token") != "test-key" {
            t.Errorf("token: %q", r.Header.Get("X-Subscription-Token"))
        }
        if r.URL.Query().Get("q") != "hello world" {
            t.Errorf("q: %q", r.URL.Query().Get("q"))
        }
        if r.URL.Query().Get("count") != "5" {
            t.Errorf("count: %q", r.URL.Query().Get("count"))
        }
        json.NewEncoder(w).Encode(map[string]any{
            "web": map[string]any{
                "results": []map[string]any{
                    {"title": "Result 1", "url": "https://example.com/1", "description": "First"},
                    {"title": "Result 2", "url": "https://example.com/2", "description": "Second"},
                },
            },
        })
    }))
    defer srv.Close()

    s, err := New(Config{APIKey: "test-key", BaseURL: srv.URL})
    if err != nil {
        t.Fatal(err)
    }
    results, err := s.Search(context.Background(), "hello world", search.SearchOpts{MaxResults: 5})
    if err != nil {
        t.Fatal(err)
    }
    if len(results) != 2 {
        t.Fatalf("got %d results", len(results))
    }
    if results[0].Title != "Result 1" {
        t.Errorf("title: %q", results[0].Title)
    }
}

func TestSearch4xxReturnsError(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]any{"message": "invalid token"})
    }))
    defer srv.Close()

    s, _ := New(Config{APIKey: "bad", BaseURL: srv.URL})
    _, err := s.Search(context.Background(), "x", search.SearchOpts{})
    if err == nil {
        t.Fatal("expected error")
    }
}
```

- [ ] **Step 2: Write `brave.go`**

`internal/provider/search/brave/brave.go`:

```go
package brave

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strconv"
    "time"

    "github.com/zhurong/jianwu/internal/provider/search"
)

const DefaultBaseURL = "https://api.search.brave.com/res/v1/web/search"

type Config struct {
    APIKey  string
    BaseURL string // defaults to DefaultBaseURL
}

type Searcher struct {
    apiKey  string
    baseURL string
    http    *http.Client
}

func New(cfg Config) (*Searcher, error) {
    if cfg.APIKey == "" {
        return nil, fmt.Errorf("brave: APIKey required")
    }
    if cfg.BaseURL == "" {
        cfg.BaseURL = DefaultBaseURL
    }
    return &Searcher{
        apiKey:  cfg.APIKey,
        baseURL: cfg.BaseURL,
        http:    &http.Client{Timeout: 15 * time.Second},
    }, nil
}

func (s *Searcher) Search(ctx context.Context, query string, opts search.SearchOpts) ([]search.SearchResult, error) {
    if opts.MaxResults == 0 {
        opts.MaxResults = 10
    }
    params := url.Values{}
    params.Set("q", query)
    params.Set("count", strconv.Itoa(opts.MaxResults))
    if opts.Language != "" {
        params.Set("search_lang", opts.Language)
    }
    switch opts.TimeRange {
    case search.TimeDay:
        params.Set("freshness", "pd")
    case search.TimeWeek:
        params.Set("freshness", "pw")
    case search.TimeMonth:
        params.Set("freshness", "pm")
    case search.TimeYear:
        params.Set("freshness", "py")
    }

    req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"?"+params.Encode(), nil)
    if err != nil {
        return nil, fmt.Errorf("brave: build request: %w", err)
    }
    req.Header.Set("X-Subscription-Token", s.apiKey)
    req.Header.Set("Accept", "application/json")

    resp, err := s.http.Do(req)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", search.ErrSearchNetwork, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusTooManyRequests {
        b, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("%w: %s", search.ErrSearchRateLimit, string(b))
    }
    if resp.StatusCode >= 400 {
        b, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("%w: HTTP %d: %s", search.ErrSearchProvider, resp.StatusCode, string(b))
    }

    var body struct {
        Web struct {
            Results []struct {
                Title       string `json:"title"`
                URL         string `json:"url"`
                Description string `json:"description"`
                Age         string `json:"age"` // Brave returns ISO 8601 duration or timestamp
            } `json:"results"`
        } `json:"web"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
        return nil, fmt.Errorf("brave: decode: %w", err)
    }
    out := make([]search.SearchResult, len(body.Web.Results))
    for i, r := range body.Web.Results {
        out[i] = search.SearchResult{Title: r.Title, URL: r.URL, Snippet: r.Description}
    }
    return out, nil
}
```

- [ ] **Step 3: Run tests, verify pass**

```bash
go test ./internal/provider/search/brave/... -v
```

- [ ] **Step 4: Commit**

```bash
git add internal/provider/search/brave/
git commit -m "feat(search/brave): Brave search provider"
```

---

## Task 10: Serper Search Provider (Fallback)

**Files:**
- Create: `internal/provider/search/serper/serper.go`
- Create: `internal/provider/search/serper/serper_test.go`

**API spec:**
- Endpoint: `https://google.serper.dev/search`
- Auth: `X-API-KEY: <key>`
- POST with JSON `{"q": "...", "num": N}`

Same shape as Brave but Google SERP via Serper.dev.

- [ ] **Step 1: Write failing test (httptest)**

`internal/provider/search/serper/serper_test.go`:

```go
package serper

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/zhurong/jianwu/internal/provider/search"
)

func TestSearchSuccess(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("X-API-KEY") != "test-key" {
            t.Errorf("key: %q", r.Header.Get("X-API-KEY"))
        }
        var body map[string]any
        json.NewDecoder(r.Body).Decode(&body)
        if body["q"] != "hello" {
            t.Errorf("q: %v", body["q"])
        }
        json.NewEncoder(w).Encode(map[string]any{
            "organic": []map[string]any{
                {"title": "R1", "link": "https://example.com/1", "snippet": "First"},
            },
        })
    }))
    defer srv.Close()
    s, _ := New(Config{APIKey: "test-key", BaseURL: srv.URL})
    results, err := s.Search(context.Background(), "hello", search.SearchOpts{MaxResults: 5})
    if err != nil {
        t.Fatal(err)
    }
    if len(results) != 1 {
        t.Fatalf("got %d", len(results))
    }
    if results[0].Title != "R1" {
        t.Errorf("title: %q", results[0].Title)
    }
}
```

- [ ] **Step 2: Write `serper.go`**

`internal/provider/search/serper/serper.go`:

```go
package serper

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "github.com/zhurong/jianwu/internal/provider/search"
)

const DefaultBaseURL = "https://google.serper.dev/search"

type Config struct {
    APIKey  string
    BaseURL string
}

type Searcher struct {
    apiKey  string
    baseURL string
    http    *http.Client
}

func New(cfg Config) (*Searcher, error) {
    if cfg.APIKey == "" {
        return nil, fmt.Errorf("serper: APIKey required")
    }
    if cfg.BaseURL == "" {
        cfg.BaseURL = DefaultBaseURL
    }
    return &Searcher{apiKey: cfg.APIKey, baseURL: cfg.BaseURL, http: &http.Client{Timeout: 15 * time.Second}}, nil
}

func (s *Searcher) Search(ctx context.Context, query string, opts search.SearchOpts) ([]search.SearchResult, error) {
    if opts.MaxResults == 0 {
        opts.MaxResults = 10
    }
    body, _ := json.Marshal(map[string]any{
        "q":   query,
        "num": opts.MaxResults,
    })
    req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL, bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("serper: build request: %w", err)
    }
    req.Header.Set("X-API-KEY", s.apiKey)
    req.Header.Set("Content-Type", "application/json")
    resp, err := s.http.Do(req)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", search.ErrSearchNetwork, err)
    }
    defer resp.Body.Close()
    if resp.StatusCode == http.StatusTooManyRequests {
        b, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("%w: %s", search.ErrSearchRateLimit, string(b))
    }
    if resp.StatusCode >= 400 {
        b, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("%w: HTTP %d: %s", search.ErrSearchProvider, resp.StatusCode, string(b))
    }
    var respBody struct {
        Organic []struct {
            Title   string `json:"title"`
            Link    string `json:"link"`
            Snippet string `json:"snippet"`
        } `json:"organic"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
        return nil, fmt.Errorf("serper: decode: %w", err)
    }
    out := make([]search.SearchResult, len(respBody.Organic))
    for i, r := range respBody.Organic {
        out[i] = search.SearchResult{Title: r.Title, URL: r.Link, Snippet: r.Snippet}
    }
    return out, nil
}
```

- [ ] **Step 3: Run tests, verify pass**

- [ ] **Step 4: Commit**

```bash
git add internal/provider/search/serper/
git commit -m "feat(search/serper): Serper.dev search provider (fallback)"
```

---

## Task 11: URL Reader (Jina)

**Files:**
- Create: `internal/provider/reader/interface.go`
- Create: `internal/provider/reader/jina/jina.go`
- Create: `internal/provider/reader/jina/jina_test.go`

**API spec:**
- Endpoint: `https://r.jina.ai/<url>`
- Auth: `Authorization: Bearer <key>` (optional for free tier; required for higher limits)
- Returns: clean markdown of the URL's content

- [ ] **Step 1: Write `interface.go`**

`internal/provider/reader/interface.go`:

```go
package reader

import (
    "context"
    "errors"
)

// Reader fetches a URL and returns clean markdown content.
type Reader interface {
    Read(ctx context.Context, url string) (Content, error)
}

// Content is the result of a Read call.
type Content struct {
    URL     string
    Title   string // extracted from page if available
    Markdown string // cleaned content
}

// ErrReader is the sentinel for reader errors.
var ErrReader = errors.New("reader error")
```

- [ ] **Step 2: Write failing test**

`internal/provider/reader/jina/jina_test.go`:

```go
package jina

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestReadSuccess(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("Authorization") != "Bearer test-key" {
            t.Errorf("auth: %q", r.Header.Get("Authorization"))
        }
        w.Header().Set("Content-Type", "text/plain")
        w.Write([]byte("Title: Example\n\nThis is the cleaned content."))
    }))
    defer srv.Close()
    rdr, err := New(Config{APIKey: "test-key", BaseURL: srv.URL})
    if err != nil {
        t.Fatal(err)
    }
    content, err := rdr.Read(context.Background(), "https://example.com/foo")
    if err != nil {
        t.Fatal(err)
    }
    if content.Markdown == "" {
        t.Error("empty markdown")
    }
}

func TestRead4xxReturnsError(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusNotFound)
    }))
    defer srv.Close()
    rdr, _ := New(Config{APIKey: "k", BaseURL: srv.URL})
    _, err := rdr.Read(context.Background(), "https://example.com/missing")
    if err == nil {
        t.Fatal("expected error")
    }
}
```

- [ ] **Step 3: Write `jina.go`**

`internal/provider/reader/jina/jina.go`:

```go
package jina

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "time"

    "github.com/zhurong/jianwu/internal/provider/reader"
)

const DefaultBaseURL = "https://r.jina.ai"

type Config struct {
    APIKey  string // optional for free tier
    BaseURL string
}

type Reader struct {
    apiKey  string
    baseURL string
    http    *http.Client
}

func New(cfg Config) (*Reader, error) {
    if cfg.BaseURL == "" {
        cfg.BaseURL = DefaultBaseURL
    }
    return &Reader{
        apiKey:  cfg.APIKey,
        baseURL: cfg.BaseURL,
        http:    &http.Client{Timeout: 30 * time.Second},
    }, nil
}

func (r *Reader) Read(ctx context.Context, targetURL string) (reader.Content, error) {
    fullURL := r.baseURL + "/" + targetURL
    req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
    if err != nil {
        return reader.Content{}, fmt.Errorf("jina: build request: %w", err)
    }
    if r.apiKey != "" {
        req.Header.Set("Authorization", "Bearer "+r.apiKey)
    }
    req.Header.Set("Accept", "text/plain")
    resp, err := r.http.Do(req)
    if err != nil {
        return reader.Content{}, fmt.Errorf("jina: fetch: %w", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        b, _ := io.ReadAll(resp.Body)
        return reader.Content{}, fmt.Errorf("%w: HTTP %d for %s: %s", reader.ErrReader, resp.StatusCode, targetURL, string(b))
    }
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return reader.Content{}, fmt.Errorf("jina: read body: %w", err)
    }
    // Parse "Title: ..." prefix from Jina's response if present.
    markdown := string(body)
    title := ""
    if len(markdown) > 7 && markdown[:7] == "Title: " {
        // Find first newline
        for i := 7; i < len(markdown); i++ {
            if markdown[i] == '\n' {
                title = markdown[7:i]
                break
            }
        }
    }
    // Encode the target URL safely (already a URL; just verify).
    if _, err := url.Parse(targetURL); err != nil {
        return reader.Content{}, fmt.Errorf("jina: invalid target URL: %w", err)
    }
    return reader.Content{URL: targetURL, Title: title, Markdown: markdown}, nil
}
```

- [ ] **Step 4: Run tests, verify pass**

- [ ] **Step 5: Commit**

```bash
git add internal/provider/reader/
git commit -m "feat(reader/jina): URL reader via Jina r.jina.ai"
```

---

## Task 12: Search + Reader Factories

**Files:**
- Create: `internal/provider/search/factory.go`
- Create: `internal/provider/search/factory_test.go`
- Create: `internal/provider/reader/factory.go`
- Create: `internal/provider/reader/factory_test.go`

- [ ] **Step 1: Write `search/factory.go`**

```go
package search

import (
    "fmt"

    "github.com/zhurong/jianwu/internal/config"
    "github.com/zhurong/jianwu/internal/provider/search/brave"
    "github.com/zhurong/jianwu/internal/provider/search/serper"
)

// New constructs a Searcher by name. Names: "brave", "serper".
func New(name string, secrets *config.Secrets) (Searcher, error) {
    switch name {
    case "brave":
        if secrets.BraveAPIKey == "" {
            return nil, fmt.Errorf("brave requires BRAVE_API_KEY")
        }
        return brave.New(brave.Config{APIKey: secrets.BraveAPIKey})
    case "serper":
        if secrets.SerperAPIKey == "" {
            return nil, fmt.Errorf("serper requires SERPER_API_KEY")
        }
        return serper.New(serper.Config{APIKey: secrets.SerperAPIKey})
    default:
        return nil, fmt.Errorf("unknown search provider: %q", name)
    }
}
```

- [ ] **Step 2: Write `reader/factory.go`**

```go
package reader

import (
    "fmt"

    "github.com/zhurong/jianwu/internal/config"
    "github.com/zhurong/jianwu/internal/provider/reader/jina"
)

// New constructs a Reader by name. Names: "jina".
func New(name string, secrets *config.Secrets) (Reader, error) {
    switch name {
    case "jina":
        return jina.New(jina.Config{APIKey: secrets.JinaAPIKey})
    default:
        return nil, fmt.Errorf("unknown reader provider: %q", name)
    }
}
```

- [ ] **Step 3: Write factory tests**

`internal/provider/search/factory_test.go`:

```go
package search

import (
    "testing"

    "github.com/zhurong/jianwu/internal/config"
)

func TestNewBrave(t *testing.T) {
    s, err := New("brave", &config.Secrets{BraveAPIKey: "k"})
    if err != nil {
        t.Fatal(err)
    }
    if s == nil {
        t.Fatal("nil")
    }
}

func TestNewSerper(t *testing.T) {
    s, err := New("serper", &config.Secrets{SerperAPIKey: "k"})
    if err != nil {
        t.Fatal(err)
    }
    if s == nil {
        t.Fatal("nil")
    }
}

func TestNewUnknownErrors(t *testing.T) {
    _, err := New("nope", &config.Secrets{})
    if err == nil {
        t.Error("expected error")
    }
}
```

`internal/provider/reader/factory_test.go`:

```go
package reader

import (
    "testing"

    "github.com/zhurong/jianwu/internal/config"
)

func TestNewJina(t *testing.T) {
    r, err := New("jina", &config.Secrets{JinaAPIKey: "k"})
    if err != nil {
        t.Fatal(err)
    }
    if r == nil {
        t.Fatal("nil")
    }
}

func TestNewUnknownErrors(t *testing.T) {
    _, err := New("nope", &config.Secrets{})
    if err == nil {
        t.Error("expected error")
    }
}
```

- [ ] **Step 4: Run tests, verify pass**

```bash
go test ./internal/provider/... -v
```

Expected: all S2 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/search/factory.go internal/provider/search/factory_test.go internal/provider/reader/factory.go internal/provider/reader/factory_test.go
git commit -m "feat(provider): search + reader factories"
```

---

## Task 13: README Update + v0.2.0 Tag

**Files:**
- Modify: `README.md`
- Modify: `internal/cli/version.go`

- [ ] **Step 1: Bump version**

`internal/cli/version.go`:

```go
package cli

var Version = "0.2.0"
```

- [ ] **Step 2: Update README**

Append a Provider section to README.md (after the Configuration section):

```markdown

## Providers (v0.2.0)

LLM:
- **Gemini** via official `google.golang.org/genai` SDK (gemini-2.5-pro, gemini-2.5-flash, text-embedding-004)
- **GLM** via direct REST, OpenAI-compatible client (glm-4.6, glm-4-air, embedding-3). Reusable for Qwen/Moonshot/DeepSeek.

Search:
- **Brave Search API** (primary)
- **Serper.dev** (fallback)

URL Reader:
- **Jina Reader** (`r.jina.ai`)

Retry policy: 3 attempts with exponential backoff + jitter on network/429/5xx.
Fallback policy: primary fails after retry → fallback model takes over.

Both are abstracted behind small Go interfaces (`Chatter`, `Embedder`, `Searcher`, `Reader`) — engine layers (S3+) compose them.
```

- [ ] **Step 3: Final test sweep**

```bash
go test ./...
go vet ./...
find . -name '*.go' -not -path './vendor/*' | xargs gofmt -l
```

All should be clean.

- [ ] **Step 4: Commit + tag**

```bash
git add README.md internal/cli/version.go
git commit -m "docs: v0.2.0 README + version bump (S2 complete)"
git tag v0.2.0
```

---

## Self-Review

After writing this plan, I re-read the spec and the 26 grill decisions:

**Spec coverage:**
- Q5 (LLM SDK choice: Gemini official + GLM direct REST): Tasks 5, 6
- Q6 (caching: Gemini context cache + GLM native, no client-side hash): Task 6 cache.go helper; GLM uses native (transparent, no code)
- Q7 (retry + fallback semantics): Tasks 3, 4
- Q10 (LLM output parsing): ChatRequest carries JSONSchema field for structured output; full implementation deferred to S3 (outline) when first used
- Q15 (testing: pragmatic, mock provider): Tasks 2 (MockProvider), integration tests with httptest
- Q16 (errors: 5 exit codes): Tasks 1, 8 sentinels align with exit codes 4 (LLM) and 5 (network)
- Q20.2 (embedding real-time): Task 6 Embed uses real-time per call, no index
- DESIGN.md §5.3 Provider interface: split into Chatter/Embedder (Streamer/Tooler deferred to S5/S7)
- DESIGN.md §5.5 Search/Reader: Tasks 8-11

**Deferrals (called out):**
- Streamer interface (S5 grill)
- Tooler interface + agent loop (S7 expand)
- Full JSON Schema translation in gemini (S3 outline when first used)
- Engine layer wiring of providers (S3+)
- Live API integration tests (manual, not in plan — they're marked SKIP when API key absent)

**Placeholder scan:** Clean — no "TBD" or "implement later" outside the explicit deferrals noted above.

**Type consistency:**
- `llm.Chatter`, `llm.Embedder` — used in Tasks 2, 3, 4, 5, 6, 7 ✓
- `llm.ChatRequest`, `llm.ChatResponse` — consistent across Tasks 1-7 ✓
- `llm.ErrNetwork`, `ErrLLMProvider`, `ErrRateLimit`, `ErrServer` — used in retry/fallback classification ✓
- `search.Searcher`, `SearchOpts`, `SearchResult` — used in Tasks 8-10, 12 ✓
- `reader.Reader`, `Content` — used in Tasks 11, 12 ✓
- Factory signatures: `NewChatter(ref, secrets)`, `NewEmbedder(ref, secrets)`, `search.New(name, secrets)`, `reader.New(name, secrets)` — consistent ✓

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-21-s2-providers.md`. 14 tasks.

Execute via superpowers:subagent-driven-development (same as S1).
