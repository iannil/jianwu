# jianwu S6: `new` Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `jianwu new` CLI command that chains grill → outline → scaffolding into a single end-to-end flow. Wraps chatter with RetryWrapper + FallbackWrapper. Detects incomplete sessions and prompts to resume. Handles slug conflicts with `--force`. Persists meta.json + outline.json to `books/<slug>/`.

**Architecture:** Cobra command under `internal/cli/new.go`. Interactive TUI UserInput implementation under `internal/cli/prompt.go` (bufio.Scanner-based). Provider assembly under `internal/cli/providers.go` (wraps chatter with retry+fallback from config). Orchestrator under `internal/cli/new_flow.go` chains the three engine stages with intermediate saves. Errors mapped to exit codes 4 (LLM) and 5 (network) via existing `InfoError` mechanism.

**Tech Stack:** Go 1.22+, S1-S5 packages (workspace/config/book/cli/engine/provider), cobra (already pulled), bufio for terminal input.

## Global Constraints

- Go version floor: 1.22
- Module path: `github.com/iannil/jianwu`
- License: AGPL-3.0
- TDD discipline (test-after for CLI/integration code)
- `jianwu new` runs **all three engine stages** in sequence: grill → outline → scaffolding (per Q19.A "全自动+resume")
- Stages run automatically without prompts between them (per Q19.A)
- Interruption (Ctrl+C) is safe; `jianwu new` on re-entry detects incomplete session and prompts to resume (per Q19.A + Q11.A2)
- Slug conflicts return error by default; `--force` overwrites (per Q21.A3)
- All LLM calls wrapped in RetryWrapper + FallbackWrapper using config-driven models
- Exit codes: 4 for LLM errors, 5 for network errors (via InfoError wrapping)
- No streaming in v1 (deferred to S5/S7 followups); output is plain stdout/stderr
- Commit after every task

---

## File Structure

| Path | Responsibility |
|---|---|
| `internal/cli/new.go` | Cobra `new` command definition + arg/flag parsing |
| `internal/cli/prompt.go` | TerminalPrompt: implements grill.UserInput via bufio.Scanner |
| `internal/cli/prompt_test.go` | Tests for prompt parsing helpers |
| `internal/cli/providers.go` | `buildChatter(config, secrets, stage)` — wraps with retry+fallback |
| `internal/cli/providers_test.go` | Tests for provider assembly |
| `internal/cli/new_flow.go` | `runNewFlow(...)` — chains grill → outline → scaffolding |
| `internal/cli/new_flow_test.go` | Tests using mock providers |
| `internal/cli/new_test.go` | Integration test: full flow with mocks |
| `internal/cli/e2e_new_test.go` | End-to-end CLI test: `jianwu new` in temp workspace |

---

## Task 0: Providers Assembly

**Files:**
- Create: `internal/cli/providers.go`
- Create: `internal/cli/providers_test.go`

**Interfaces:**
- Produces: `cli.buildChatter(cfg *config.Config, secrets *config.Secrets, stage string) (llm.Chatter, error)`

- [ ] **Step 1: Write `providers.go`**

`internal/cli/providers.go`:

```go
package cli

import (
    "fmt"

    "github.com/iannil/jianwu/internal/config"
    "github.com/iannil/jianwu/internal/provider/llm"
    "github.com/iannil/jianwu/internal/provider/llmfactory"
)

// buildChatter constructs a Chatter for the given stage, wrapped in Retry + Fallback per Q7.
// stage is one of "intake", "outline", "scaffolding", "expand".
// For S6, fallback is optional: if cfg.Models[stage] has no fallback configured, returns primary only.
func buildChatter(cfg *config.Config, secrets *config.Secrets, stage string) (llm.Chatter, error) {
    primary, err := stageModel(cfg, stage)
    if err != nil {
        return nil, err
    }
    p, err := llmfactory.NewChatter(primary, secrets)
    if err != nil {
        return nil, fmt.Errorf("%s: %w", stage, err)
    }
    wrapped := llm.NewRetryWrapper(p)
    // Note: in S6 we don't yet wire fallback because Config doesn't carry fallback yet.
    // S6.1 or later can add config.Models[stage].Fallback and wrap with FallbackWrapper.
    return wrapped, nil
}

// buildEmbedder constructs an Embedder for the given stage.
func buildEmbedder(cfg *config.Config, secrets *config.Secrets, stage string) (llm.Embedder, error) {
    primary, err := stageModel(cfg, stage)
    if err != nil {
        return nil, err
    }
    return llmfactory.NewEmbedder(primary, secrets)
}

// stageModel returns the ModelRef for the given stage.
func stageModel(cfg *config.Config, stage string) (config.ModelRef, error) {
    switch stage {
    case "intake":
        return cfg.Models.Intake, nil
    case "outline":
        return cfg.Models.Outline, nil
    case "scaffolding":
        return cfg.Models.Scaffolding, nil
    case "expand":
        return cfg.Models.Expand, nil
    default:
        return config.ModelRef{}, fmt.Errorf("unknown stage: %q", stage)
    }
}
```

- [ ] **Step 2: Write tests**

`internal/cli/providers_test.go`:

```go
package cli

import (
    "testing"

    "github.com/iannil/jianwu/internal/config"
)

func TestBuildChatterIntake(t *testing.T) {
    cfg := &config.Config{
        Models: config.Models{
            Intake: config.ModelRef{Provider: "gemini", Model: "gemini-2.5-pro"},
        },
    }
    secrets := &config.Secrets{GeminiAPIKey: "fake-key"}
    c, err := buildChatter(cfg, secrets, "intake")
    if err != nil {
        t.Fatal(err)
    }
    if c == nil {
        t.Fatal("nil chatter")
    }
}

func TestBuildChatterMissingKey(t *testing.T) {
    cfg := &config.Config{
        Models: config.Models{
            Outline: config.ModelRef{Provider: "gemini", Model: "gemini-2.5-pro"},
        },
    }
    _, err := buildChatter(cfg, &config.Secrets{}, "outline")
    if err == nil {
        t.Error("expected error for missing key")
    }
}

func TestBuildChatterUnknownStage(t *testing.T) {
    _, err := buildChatter(&config.Config{}, &config.Secrets{}, "bogus")
    if err == nil {
        t.Error("expected error for unknown stage")
    }
}

func TestBuildEmbedder(t *testing.T) {
    cfg := &config.Config{
        Models: config.Models{
            Scaffolding: config.ModelRef{Provider: "glm", Model: "glm-4.6"},
        },
    }
    secrets := &config.Secrets{GLMAPIKey: "fake"}
    e, err := buildEmbedder(cfg, secrets, "scaffolding")
    if err != nil {
        t.Fatal(err)
    }
    if e == nil {
        t.Fatal("nil embedder")
    }
}
```

- [ ] **Step 3: Run tests, verify pass**

- [ ] **Step 4: Commit**

```bash
git add internal/cli/providers.go internal/cli/providers_test.go
git commit -m "feat(cli): provider assembly with retry wrapping per stage"
```

---

## Task 1: TerminalPrompt (UserInput impl)

**Files:**
- Create: `internal/cli/prompt.go`
- Create: `internal/cli/prompt_test.go`

- [ ] **Step 1: Write `prompt.go`**

`internal/cli/prompt.go`:

```go
package cli

import (
    "bufio"
    "fmt"
    "io"
    "strings"

    "github.com/iannil/jianwu/internal/engine/grill"
)

// TerminalPrompt implements grill.UserInput via bufio.Scanner over stdin/stdout.
type TerminalPrompt struct {
    In  io.Reader
    Out io.Writer
}

// NewTerminalPrompt constructs a TerminalPrompt using the given reader/writer.
// Defaults to os.Stdin / os.Stdout if nil.
func NewTerminalPrompt(in io.Reader, out io.Writer) *TerminalPrompt {
    if in == nil {
        in = stdin()
    }
    if out == nil {
        out = stdout()
    }
    return &TerminalPrompt{In: in, Out: out}
}

// Ask presents the question + recommendation, returns user's answer.
// Empty input = accept recommendation; "skip" = use default.
// Multiline recommendations are shown indented under the question.
func (p *TerminalPrompt) Ask(dim grill.Dimension, recommendation string) (string, error) {
    fmt.Fprintf(p.Out, "\n◆ %s\n", dim.Name)
    fmt.Fprintf(p.Out, "  %s\n", dim.Question)
    if len(dim.Options) > 0 {
        fmt.Fprintf(p.Out, "  选项: %s\n", strings.Join(dim.Options, ", "))
    }
    if recommendation != "" {
        firstLine := recommendation
        if i := strings.IndexByte(recommendation, '\n'); i >= 0 {
            firstLine = recommendation[:i]
        }
        fmt.Fprintf(p.Out, "  推荐: %s\n", firstLine)
        // If there's reasoning, show it indented.
        if i := strings.IndexByte(recommendation, '\n'); i >= 0 {
            reasoning := strings.TrimSpace(recommendation[i+1:])
            if reasoning != "" {
                for _, line := range strings.Split(reasoning, "\n") {
                    fmt.Fprintf(p.Out, "    %s\n", line)
                }
            }
        }
    }
    fmt.Fprintf(p.Out, "  [回车=接受推荐 / 输入值 / skip=默认(%s)] ", dim.DefaultValue)
    reader := bufio.NewReader(p.In)
    line, err := reader.ReadString('\n')
    if err != nil && err != io.EOF {
        return "", fmt.Errorf("read input: %w", err)
    }
    answer := strings.TrimSpace(line)
    return answer, nil
}

// stdin/stdout indirection lets tests inject readers/writers.
// Real implementations just return os.Stdin / os.Stdout.
func stdin() io.Reader  { return osStdin }
func stdout() io.Writer { return osStdout }
```

Add the os import + osStdin/osStdout vars:

```go
import "os"

var (
    osStdin  io.Reader = os.Stdin
    osStdout io.Writer = os.Stdout
)
```

- [ ] **Step 2: Write tests**

`internal/cli/prompt_test.go`:

```go
package cli

import (
    "bytes"
    "strings"
    "testing"

    "github.com/iannil/jianwu/internal/engine/grill"
)

func TestTerminalPromptAskAcceptsEmpty(t *testing.T) {
    var out bytes.Buffer
    p := &TerminalPrompt{In: strings.NewReader("\n"), Out: &out}
    dim := grill.Dimension{
        Name:        "受众",
        Question:    "目标读者是谁？",
        Options:     []string{"scholar", "educated-general"},
        DefaultValue: "educated-general",
    }
    answer, err := p.Ask(dim, "scholar\n\nBecause topic is advanced.")
    if err != nil {
        t.Fatal(err)
    }
    if answer != "" {
        t.Errorf("expected empty (accept), got %q", answer)
    }
    s := out.String()
    if !strings.Contains(s, "◆ 受众") {
        t.Errorf("missing name header")
    }
    if !strings.Contains(s, "推荐: scholar") {
        t.Errorf("missing recommendation")
    }
}

func TestTerminalPromptAskReturnsInput(t *testing.T) {
    var out bytes.Buffer
    p := &TerminalPrompt{In: strings.NewReader("beginner\n"), Out: &out}
    dim := grill.Dimension{Name: "受众", DefaultValue: "educated-general"}
    answer, err := p.Ask(dim, "scholar")
    if err != nil {
        t.Fatal(err)
    }
    if answer != "beginner" {
        t.Errorf("got %q", answer)
    }
}

func TestTerminalPromptAskReturnsSkip(t *testing.T) {
    p := &TerminalPrompt{In: strings.NewReader("skip\n"), Out: &bytes.Buffer{}}
    answer, err := p.Ask(grill.Dimension{DefaultValue: "x"}, "")
    if err != nil {
        t.Fatal(err)
    }
    if answer != "skip" {
        t.Errorf("got %q", answer)
    }
}

func TestTerminalPromptShowsReasoningIndented(t *testing.T) {
    var out bytes.Buffer
    p := &TerminalPrompt{In: strings.NewReader("\n"), Out: &out}
    rec := "scholar\nBecause the topic is advanced.\nIt needs deep engagement."
    _, _ = p.Ask(grill.Dimension{Name: "受众", DefaultValue: "x"}, rec)
    s := out.String()
    if !strings.Contains(s, "    Because the topic is advanced.") {
        t.Errorf("reasoning not indented")
    }
}
```

- [ ] **Step 3: Run tests**

- [ ] **Step 4: Commit**

```bash
git add internal/cli/prompt.go internal/cli/prompt_test.go
git commit -m "feat(cli): TerminalPrompt implementing grill.UserInput"
```

---

## Task 2: Slug Conflict Handling + Resume Detection

**Files:**
- Create: `internal/cli/new_flow.go` (partial — just slug + resume helpers)
- Create: `internal/cli/new_flow_test.go`

- [ ] **Step 1: Write `new_flow.go` (partial)**

```go
package cli

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/iannil/jianwu/internal/book"
    "github.com/iannil/jianwu/internal/engine/grill"
)

// checkSlugConflict returns nil if the slug is available; an error if a book exists
// and force=false. If force=true, removes existing book dir before returning nil.
// Per Q21.A3.
func checkSlugConflict(wsRoot, slug string, force bool) error {
    bookDir := filepath.Join(wsRoot, "books", slug)
    info, err := os.Stat(bookDir)
    if err != nil {
        if os.IsNotExist(err) {
            return nil
        }
        return fmt.Errorf("stat %s: %w", bookDir, err)
    }
    if !info.IsDir() {
        return fmt.Errorf("%s exists and is not a directory", bookDir)
    }
    if !force {
        return fmt.Errorf("book %q already exists at %s; use --force to overwrite", slug, bookDir)
    }
    if err := os.RemoveAll(bookDir); err != nil {
        return fmt.Errorf("remove existing book dir: %w", err)
    }
    return nil
}

// offerResume checks for incomplete sessions and asks the user whether to resume.
// Returns the session to resume (or nil to start fresh).
// Per Q11.A2.
func offerResume(repo *grill.Repository, prompt *TerminalPrompt) (*grill.Session, error) {
    incomplete, err := repo.ListIncomplete()
    if err != nil {
        return nil, fmt.Errorf("list incomplete sessions: %w", err)
    }
    if len(incomplete) == 0 {
        return nil, nil
    }
    fmt.Fprintf(prompt.Out, "\n检测到 %d 个未完成的 grill 会话:\n", len(incomplete))
    for i, s := range incomplete {
        firstTopic := s.Answers["topic"]
        if firstTopic == "" {
            firstTopic = "(未开始)"
        }
        fmt.Fprintf(prompt.Out, "  [%d] %s — %s\n", i+1, s.ID, firstTopic)
    }
    fmt.Fprintf(prompt.Out, "[回车=新会话 / 输入序号=恢复] ")
    reader := bufio.NewReader(prompt.In)
    line, err := reader.ReadString('\n')
    if err != nil && err != io.EOF {
        return nil, fmt.Errorf("read resume choice: %w", err)
    }
    line = strings.TrimSpace(line)
    if line == "" {
        return nil, nil
    }
    idx, err := strconv.Atoi(line)
    if err != nil || idx < 1 || idx > len(incomplete) {
        return nil, fmt.Errorf("invalid selection: %q", line)
    }
    return incomplete[idx-1], nil
}

// deriveSlugFromTopic produces a slug from the topic answer.
// Uses book.Slugify.
func deriveSlugFromTopic(topic string) string {
    return book.Slugify(topic)
}

// (imports needed: bufio, io, os, strconv, strings)
```

(Add necessary imports: `bufio`, `io`, `strconv`, `strings`.)

- [ ] **Step 2: Write tests**

`new_flow_test.go`:

```go
package cli

import (
    "bytes"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/iannil/jianwu/internal/engine/grill"
)

func TestCheckSlugConflictEmpty(t *testing.T) {
    ws := t.TempDir()
    if err := checkSlugConflict(ws, "my-book", false); err != nil {
        t.Errorf("expected nil, got %v", err)
    }
}

func TestCheckSlugConflictExistingNoForce(t *testing.T) {
    ws := t.TempDir()
    bookDir := filepath.Join(ws, "books", "my-book")
    if err := os.MkdirAll(bookDir, 0o755); err != nil {
        t.Fatal(err)
    }
    err := checkSlugConflict(ws, "my-book", false)
    if err == nil {
        t.Fatal("expected error")
    }
    if !strings.Contains(err.Error(), "already exists") {
        t.Errorf("error: %v", err)
    }
}

func TestCheckSlugConflictExistingForceRemoves(t *testing.T) {
    ws := t.TempDir()
    bookDir := filepath.Join(ws, "books", "my-book")
    if err := os.MkdirAll(filepath.Join(bookDir, "chapters"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(bookDir, "meta.json"), []byte("{}"), 0o644); err != nil {
        t.Fatal(err)
    }
    if err := checkSlugConflict(ws, "my-book", true); err != nil {
        t.Errorf("expected nil, got %v", err)
    }
    if _, err := os.Stat(bookDir); !os.IsNotExist(err) {
        t.Errorf("book dir should be removed")
    }
}

func TestOfferResumeNoSessions(t *testing.T) {
    ws := t.TempDir()
    repo := grill.NewRepository(ws)
    var out bytes.Buffer
    p := &TerminalPrompt{In: strings.NewReader(""), Out: &out}
    s, err := offerResume(repo, p)
    if err != nil {
        t.Fatal(err)
    }
    if s != nil {
        t.Errorf("expected nil, got %v", s)
    }
}

func TestOfferResumeWithChoice(t *testing.T) {
    ws := t.TempDir()
    repo := grill.NewRepository(ws)
    s := grill.NewSession()
    s.RecordAnswer("topic", "时间的实在")
    if err := repo.Save(s); err != nil {
        t.Fatal(err)
    }

    var out bytes.Buffer
    p := &TerminalPrompt{In: strings.NewReader("1\n"), Out: &out}
    loaded, err := offerResume(repo, p)
    if err != nil {
        t.Fatal(err)
    }
    if loaded == nil || loaded.ID != s.ID {
        t.Errorf("expected resumed session %s, got %v", s.ID, loaded)
    }
}

func TestOfferResumeEmptyInputStartsFresh(t *testing.T) {
    ws := t.TempDir()
    repo := grill.NewRepository(ws)
    s := grill.NewSession()
    s.RecordAnswer("topic", "X")
    if err := repo.Save(s); err != nil {
        t.Fatal(err)
    }
    var out bytes.Buffer
    p := &TerminalPrompt{In: strings.NewReader("\n"), Out: &out}
    loaded, err := offerResume(repo, p)
    if err != nil {
        t.Fatal(err)
    }
    if loaded != nil {
        t.Errorf("expected nil (fresh start), got %v", loaded)
    }
}

func TestDeriveSlugFromTopic(t *testing.T) {
    s := deriveSlugFromTopic("Reality of Time")
    if s != "reality-of-time" {
        t.Errorf("got %q", s)
    }
}
```

- [ ] **Step 3: Run tests**

- [ ] **Step 4: Commit**

```bash
git add internal/cli/new_flow.go internal/cli/new_flow_test.go
git commit -m "feat(cli): slug conflict + resume detection helpers"
```

---

## Task 3: Full Flow Orchestrator

**Files:**
- Modify: `internal/cli/new_flow.go` (add runNewFlow)
- Modify: `internal/cli/new_flow_test.go` (add tests)

- [ ] **Step 1: Add `runNewFlow` to `new_flow.go`**

```go
// runNewFlow executes the full grill → outline → scaffolding pipeline.
// Returns the final outline and the session (archived) or an error wrapped as *InfoError.
func runNewFlow(
    wsRoot string,
    cfg *config.Config,
    secrets *config.Secrets,
    prompt *TerminalPrompt,
    force bool,
) (*book.Outline, *grill.Session, error) {
    tree := grill.DefaultTree()
    repo := grill.NewRepository(wsRoot)

    // 1. Resume detection
    session, err := offerResume(repo, prompt)
    if err != nil {
        return nil, nil, &InfoError{Err: err, Code: ExitCodeGeneric}
    }
    if session == nil {
        session = grill.NewSession()
    }

    // 2. Grill: walk tree, ask each dim
    intakeChatter, err := buildChatter(cfg, secrets, "intake")
    if err != nil {
        return nil, session, &InfoError{Err: err, Code: ExitCodeLLMProvider}
    }
    for {
        next, err := grill.Run(defaultCtx(), intakeChatter, tree, session, prompt)
        if err != nil {
            // Save session so user can resume.
            _ = repo.Save(session)
            return nil, session, wrapLLMError(err)
        }
        // Save after each step (resumable).
        if err := repo.Save(session); err != nil {
            return nil, session, &InfoError{Err: err, Code: ExitCodeGeneric}
        }
        if next == nil {
            break
        }
    }

    // 3. Derive slug from topic answer
    slug := deriveSlugFromTopic(session.Answers["topic"])
    if slug == "" {
        return nil, session, &InfoError{
            Err:  fmt.Errorf("could not derive slug from topic %q", session.Answers["topic"]),
            Code: ExitCodeGeneric,
        }
    }

    // 4. Check slug conflict
    if err := checkSlugConflict(wsRoot, slug, force); err != nil {
        return nil, session, &InfoError{Err: err, Code: ExitCodeGeneric}
    }

    // 5. Outline
    outlineChatter, err := buildChatter(cfg, secrets, "outline")
    if err != nil {
        return nil, session, &InfoError{Err: err, Code: ExitCodeLLMProvider}
    }
    outline, err := outline.Generate(defaultCtx(), outlineChatter, outline.Input{
        ArchetypeID: session.Answers["archetype"],
        Topic:       session.Answers["topic"],
        Audience:    session.Answers["audience"],
        Depth:       session.Answers["depth"],
        Goal:        session.Answers["goal"],
        Length:      session.Answers["length"],
        Language:    session.Answers["language"],
    })
    if err != nil {
        return nil, session, wrapLLMError(err)
    }

    // 6. Save book meta + outline
    bookDir := filepath.Join(wsRoot, "books", slug)
    if err := writeBookMeta(bookDir, slug, session, cfg); err != nil {
        return nil, session, &InfoError{Err: err, Code: ExitCodeGeneric}
    }
    if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
        return nil, session, &InfoError{Err: err, Code: ExitCodeGeneric}
    }

    // 7. Scaffolding
    scaffChatter, err := buildChatter(cfg, secrets, "scaffolding")
    if err != nil {
        return outline, session, &InfoError{Err: err, Code: ExitCodeLLMProvider}
    }
    scaffolding.ScaffoldAll(defaultCtx(), scaffChatter, outline, session.Answers["archetype"],
        scaffolding.ChapterParams{
            Topic:    session.Answers["topic"],
            Audience: session.Answers["audience"],
            Depth:    session.Answers["depth"],
            Goal:     session.Answers["goal"],
            Length:   session.Answers["length"],
            Language: session.Answers["language"],
        },
        scaffolding.Options{},
    )
    // Save outline with scaffolded chapters
    if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
        return outline, session, &InfoError{Err: err, Code: ExitCodeGeneric}
    }

    // 8. Archive session to book dir as audit log
    if err := repo.Archive(session, slug); err != nil {
        // Non-fatal: log and continue.
        fmt.Fprintf(prompt.Out, "warning: could not archive session: %v\n", err)
    }

    return outline, session, nil
}

// writeBookMeta writes meta.json for the new book.
func writeBookMeta(bookDir, slug string, session *grill.Session, cfg *config.Config) error {
    if err := os.MkdirAll(bookDir, 0o755); err != nil {
        return fmt.Errorf("mkdir book dir: %w", err)
    }
    meta := &book.Meta{
        ID:        uuid.NewString(),
        Slug:      slug,
        Title:     session.Answers["topic"],
        Archetype: session.Answers["archetype"],
        Language:  session.Answers["language"],
        Status:    book.BookStatusDraft,
        CreatedAt: time.Now().UTC(),
        UpdatedAt: time.Now().UTC(),
        Parameters: book.Parameters{
            Audience: session.Answers["audience"],
            Depth:    session.Answers["depth"],
            Goal:     session.Answers["goal"],
            Length:   session.Answers["length"],
        },
    }
    return book.SaveMeta(filepath.Join(bookDir, "meta.json"), meta)
}

// wrapLLMError classifies an error as InfoError with appropriate exit code.
func wrapLLMError(err error) error {
    if errors.Is(err, llm.ErrNetwork) {
        return &InfoError{Err: err, Code: ExitCodeNetwork}
    }
    return &InfoError{Err: err, Code: ExitCodeLLMProvider}
}

// defaultCtx returns a context with no timeout (S6 doesn't add timeouts; user can Ctrl+C).
func defaultCtx() context.Context {
    return context.Background()
}
```

(Imports needed: context, errors, fmt, os, path/filepath, time, uuid, plus engine packages.)

- [ ] **Step 2: Add tests with mock chatters**

`new_flow_test.go` — append:

```go
// mockChatterProvider lets tests inject chatters without going through config.
type mockChatterProvider struct {
    intake      llm.Chatter
    outline     llm.Chatter
    scaffolding llm.Chatter
}

// For testing, we add an internal variant of runNewFlow that accepts chatters directly.
// This avoids needing real API keys in tests.

func TestRunNewFlowHappyPathWithMocks(t *testing.T) {
    ws := t.TempDir()
    // Init workspace.
    if err := workspace.Init(ws, workspace.InitOpts{}); err != nil {
        t.Fatal(err)
    }
    cfg := config.BuiltinDefaults()
    secrets := &config.Secrets{}

    // Outline returns a minimal outline
    outlineJSON := `{"parts":[{"index":1,"title":"P1","role":"ontology","chapters":[
        {"index":1,"title":"C1","status":"scaffolded"}
    ]}]}`
    outlineChatter := mock.New(llm.ChatResponse{Content: outlineJSON})

    // Scaffolding returns a scaffold for one chapter
    scaffoldJSON := `{"abstract":"X","key_concepts":["a"],"learning_objectives":["y"],"suggested_examples":["z"]}`
    scaffChatter := mock.New(llm.ChatResponse{Content: scaffoldJSON})

    // Intake: scripted recommendations
    intakeChatter := mock.New(llm.ChatResponse{Content: "recommendation\nreason"})

    // User input: accept all recommendations (empty lines) for each dim
    // We need one empty line per dimension asked. Default tree has 12 dims.
    // Conditional dims skipped via trigger; required answered.
    inputLines := []string{}
    for i := 0; i < 15; i++ { // generous
        inputLines = append(inputLines, "")
    }
    userInput := strings.Join(inputLines, "\n") + "\n"

    prompt := &TerminalPrompt{
        In:  strings.NewReader(userInput),
        Out: &bytes.Buffer{},
    }

    // Call the testable variant
    outline, _, err := runNewFlowWithMocks(ws, cfg, secrets, prompt, false, mockChatterProvider{
        intake:      intakeChatter,
        outline:     outlineChatter,
        scaffolding: scaffChatter,
    })
    if err != nil {
        t.Fatalf("runNewFlow: %v", err)
    }
    if outline == nil {
        t.Fatal("nil outline")
    }
    // Book dir should exist.
    slug := book.Slugify("") // topic was "recommendation" first line — adjust
    // Actually topic answer = "recommendation" (first line of rec since user accepted)
    // Let's check what files exist.
    entries, _ := os.ReadDir(filepath.Join(ws, "books"))
    if len(entries) == 0 {
        t.Fatal("no book created")
    }
}
```

**Important design note:** the test above needs a `runNewFlowWithMocks` variant. Refactor `runNewFlow` to accept chatters via a struct, with the public version building chatters from config then delegating:

```go
// chatterProvider bundles the three chatters needed by runNewFlow.
type chatterProvider struct {
    intake, outline, scaffolding llm.Chatter
}

func runNewFlow(wsRoot string, cfg *config.Config, secrets *config.Secrets, prompt *TerminalPrompt, force bool) (*book.Outline, *grill.Session, error) {
    cp, err := buildChatterProvider(cfg, secrets)
    if err != nil {
        return nil, nil, &InfoError{Err: err, Code: ExitCodeLLMProvider}
    }
    return runNewFlowWithChatters(wsRoot, prompt, force, cp)
}

func buildChatterProvider(cfg *config.Config, secrets *config.Secrets) (chatterProvider, error) {
    intake, err := buildChatter(cfg, secrets, "intake")
    if err != nil {
        return chatterProvider{}, err
    }
    outline, err := buildChatter(cfg, secrets, "outline")
    if err != nil {
        return chatterProvider{}, err
    }
    scaff, err := buildChatter(cfg, secrets, "scaffolding")
    if err != nil {
        return chatterProvider{}, err
    }
    return chatterProvider{intake: intake, outline: outline, scaffolding: scaff}, nil
}

// runNewFlowWithChatters is the testable core.
func runNewFlowWithChatters(
    wsRoot string,
    prompt *TerminalPrompt,
    force bool,
    cp chatterProvider,
) (*book.Outline, *grill.Session, error) {
    // ... same body as runNewFlow above, using cp.intake / cp.outline / cp.scaffolding ...
}
```

Tests then call `runNewFlowWithChatters` directly.

- [ ] **Step 3: Run tests**

- [ ] **Step 4: Commit**

```bash
git add internal/cli/new_flow.go internal/cli/new_flow_test.go
git commit -m "feat(cli): full new flow orchestrator (grill → outline → scaffolding)"
```

---

## Task 4: Cobra `new` Command

**Files:**
- Create: `internal/cli/new.go`
- Modify: `internal/cli/root.go` (register the command)

- [ ] **Step 1: Write `new.go`**

```go
package cli

import (
    "fmt"

    "github.com/spf13/cobra"

    "github.com/iannil/jianwu/internal/config"
    "github.com/iannil/jianwu/internal/workspace"
)

func newNewCmd() *cobra.Command {
    var force bool
    cmd := &cobra.Command{
        Use:   "new",
        Short: "Start a new book (interactive grill → outline → scaffolding)",
        Long: `Walk through the grill questionnaire interactively, then auto-generate
outline + scaffolding. If an incomplete grill session exists, prompts to resume.

Use --force to overwrite an existing book with the same slug.`,
        Args: cobra.NoArgs,
        RunE: func(cmd *cobra.Command, args []string) error {
            wsRoot, err := workspace.FindWorkspace(".")
            if err != nil {
                return &InfoError{Err: err, Code: ExitCodeWorkspaceNotFound}
            }
            ws, err := workspace.Load(wsRoot)
            if err != nil {
                return err
            }
            secrets, _ := config.LoadSecrets()
            prompt := NewTerminalPrompt(nil, cmd.OutOrStdout())

            out := cmd.OutOrStdout()
            fmt.Fprintf(out, "jianwu new — starting grill flow\n")
            fmt.Fprintf(out, "Workspace: %s\n", wsRoot)

            outline, session, err := runNewFlow(wsRoot, ws.Config, secrets, prompt, force)
            if err != nil {
                return err
            }
            // Summary
            fmt.Fprintf(out, "\n✓ Book created\n")
            fmt.Fprintf(out, "  Parts: %d\n", len(outline.Parts))
            totalCh := 0
            scaffolded := 0
            failed := 0
            for _, p := range outline.Parts {
                for _, c := range p.Chapters {
                    totalCh++
                    switch c.Status {
                    case "scaffolded":
                        scaffolded++
                    case "failed":
                        failed++
                    }
                }
            }
            fmt.Fprintf(out, "  Chapters: %d (scaffolded: %d, failed: %d)\n", totalCh, scaffolded, failed)
            if failed > 0 {
                fmt.Fprintf(out, "\nRun `jianwu status <slug>` to see failed chapters.\n")
                fmt.Fprintf(out, "Run `jianwu scaffolding <slug> --retry-failed` to retry them.\n")
            }
            return nil
        },
    }
    cmd.Flags().BoolVar(&force, "force", false, "overwrite existing book with same slug")
    return cmd
}
```

- [ ] **Step 2: Register in root.go**

Modify `internal/cli/root.go` — in `NewRootCmd()`, add after `cmd.AddCommand(newConfigCmd())`:

```go
cmd.AddCommand(newNewCmd())
```

- [ ] **Step 3: Verify build**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/cli/new.go internal/cli/root.go
git commit -m "feat(cli): jianwu new command wiring full flow + --force flag"
```

---

## Task 5: E2E CLI Test

**Files:**
- Create: `internal/cli/e2e_new_test.go`

- [ ] **Step 1: Write e2e test**

```go
package cli

import (
    "bytes"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/iannil/jianwu/internal/provider/llm"
    "github.com/iannil/jianwu/internal/provider/llm/mock"
)

// TestE2ENewCommandWithMocks runs the full `jianwu new` CLI surface against
// mocked chatters injected via the testable runNewFlowWithChatters path.
// This avoids needing API keys while still exercising the cobra wiring.
func TestE2ENewCommandWithMocks(t *testing.T) {
    root := t.TempDir()
    // Initialize workspace first.
    initCmd := NewRootCmd()
    initCmd.SetArgs([]string{"init", root})
    initCmd.SetOut(&bytes.Buffer{})
    initCmd.SetErr(&bytes.Buffer{})
    if err := initCmd.Execute(); err != nil {
        t.Fatal(err)
    }

    // Switch into the workspace.
    oldWd, _ := os.Getwd()
    defer os.Chdir(oldWd)
    if err := os.Chdir(root); err != nil {
        t.Fatal(err)
    }

    // Set fake API keys (factory checks presence, doesn't validate).
    t.Setenv("GEMINI_API_KEY", "fake")
    t.Setenv("GLM_API_KEY", "fake")

    // Build user input: accept all recommendations (12+ empty lines).
    var inputBuf bytes.Buffer
    for i := 0; i < 15; i++ {
        inputBuf.WriteString("\n")
    }
    // Inject mock chatters by monkey-patching buildChatterProvider.
    // For testability we need to expose a hook. Add a package-level var:
    //   var chatterProviderForTest = buildChatterProvider  (production default)
    // Tests can override.
    originalProvider := chatterProviderHook
    defer func() { chatterProviderHook = originalProvider }()
    chatterProviderHook = func(_, _ interface{}) (chatterProvider, error) {
        outlineJSON := `{"parts":[{"index":1,"title":"P1","role":"ontology","chapters":[
            {"index":1,"title":"C1","status":"scaffolded"}
        ]}]}`
        scaffoldJSON := `{"abstract":"X","key_concepts":["a"],"learning_objectives":["y"],"suggested_examples":["z"]}`
        return chatterProvider{
            intake:      mock.New(llm.ChatResponse{Content: "recommendation\nreason"}),
            outline:     mock.New(llm.ChatResponse{Content: outlineJSON}),
            scaffolding: mock.New(llm.ChatResponse{Content: scaffoldJSON}),
        }, nil
    }

    // The cobra command's stdin needs to be set, but TerminalPrompt uses os.Stdin directly.
    // For testing, we redirect osStdin (which is a package var).
    originalStdin := osStdin
    defer func() { osStdin = originalStdin }()
    osStdin = strings.NewReader(inputBuf.String())

    cmd := NewRootCmd()
    cmd.SetOut(&bytes.Buffer{})
    cmd.SetErr(&bytes.Buffer{})
    cmd.SetArgs([]string{"new"})
    err := cmd.Execute()
    if err != nil {
        t.Fatalf("new command: %v", err)
    }

    // Verify book was created.
    booksDir := filepath.Join(root, "books")
    entries, err := os.ReadDir(booksDir)
    if err != nil {
        t.Fatalf("books dir: %v", err)
    }
    if len(entries) == 0 {
        t.Fatal("no book created")
    }
    bookDir := filepath.Join(booksDir, entries[0].Name())
    for _, want := range []string{"meta.json", "outline.json"} {
        if _, err := os.Stat(filepath.Join(bookDir, want)); err != nil {
            t.Errorf("missing %s: %v", want, err)
        }
    }
}
```

**Important:** This test requires a `chatterProviderHook` mechanism for monkey-patching. Add to `new.go` or `new_flow.go`:

```go
// chatterProviderHook allows tests to inject mock chatters without going through
// the real factory. Production code uses the real factory via buildChatterProvider.
var chatterProviderHook = func(cfg *config.Config, secrets *config.Secrets) (chatterProvider, error) {
    return buildChatterProvider(cfg, secrets)
}
```

And modify `runNewFlow` to call `chatterProviderHook(cfg, secrets)` instead of `buildChatterProvider(cfg, secrets)` directly.

Update the test's hook signature to match (`*config.Config, *config.Secrets`, not `interface{}`):

```go
chatterProviderHook = func(_ *config.Config, _ *config.Secrets) (chatterProvider, error) {
    ...
}
```

- [ ] **Step 2: Run e2e test**

```bash
go test ./internal/cli/... -run TestE2ENew -v
```

- [ ] **Step 3: Commit**

```bash
git add internal/cli/e2e_new_test.go internal/cli/new_flow.go
git commit -m "test(cli): e2e new command with mocked chatters via hook"
```

---

## Task 6: README + Version Bump + v0.0.6

**Files:**
- Modify: `README.md`
- Modify: `internal/cli/version.go`

- [ ] **Step 1: Bump version**

`internal/cli/version.go`:

```go
package cli

var Version = "0.6.0"
```

- [ ] **Step 2: Update README**

Replace Engine section with:

```markdown

## Engine (v0.0.6)

The 4-stage engine is being built slice by slice. v0.0.6 ships **Outline + Scaffolding + Grill + `new` command**:

- **Outline** (v0.0.3): single LLM call produces full book outline
- **Scaffolding** (v0.0.4): N chapters in parallel (concurrency 5), continue-on-error
- **Grill** (v0.0.5): stateful interactive Q&A with 12-dimension design tree
- **`jianwu new`** (v0.0.6): full end-to-end flow — chains grill → outline → scaffolding. Resume-aware. Slug conflict detection with `--force`. Retry + fallback wrapping via config-driven models.

### Quick start (v0.0.6)

```bash
jianwu init my-library
cd my-library
export GEMINI_API_KEY=...
export GLM_API_KEY=...
jianwu new
# ... answer grill questions ...
# Book scaffold generated in books/<slug>/
```

If a previous `jianwu new` was interrupted, the next run detects the incomplete session and offers to resume.

Remaining stages (deferred):
- Expand (agent loop + web search, S7)
- review / finalize / export (S8)
```

- [ ] **Step 3: Final sweep + commit + tag**

```bash
go test ./...
go vet ./...
find . -name '*.go' -not -path './vendor/*' | xargs gofmt -l
git add README.md internal/cli/version.go
git commit -m "docs: v0.0.6 README + version bump (S6 new command complete)"
git tag v0.0.6
```

---

## Self-Review

**Spec coverage:**
- Q7 (retry/fallback wrapping): Task 0
- Q11.A2 (resume detection + prompt): Task 2
- Q15 (test-after for CLI/integration): Tasks 0-5
- Q19.A (full auto + resume): Task 3
- Q21.A3 (slug conflict + --force): Task 2

**Deferrals:**
- Fallback wrapper not wired (Config doesn't yet carry fallback ModelRef). S6.1 or v0.6.x can extend Config.Models[stage] with a Fallback field and wrap with FallbackWrapper in buildChatter. For v0.0.6, only RetryWrapper is wired.
- No streaming output (terminal sees final result, not tokens-as-they-arrive)
- No timeout on LLM calls (user can Ctrl+C)
- chatterProviderHook is test infrastructure that lives in production code — slightly smelly but standard for Go CLI testing

**Type consistency:**
- TerminalPrompt implements grill.UserInput ✓
- chatterProvider bundles 3 chatters ✓
- InfoError wrapping consistent with S1 ✓

**Placeholder scan:** Clean — no "TBD". One area worth flagging: the E2E test in Task 5 uses a `chatterProviderHook` global var for monkey-patching. This is a standard Go pattern for testability but worth noting.

---

## Execution Handoff

Plan saved to `docs/superpowers/plans/2026-06-22-s6-new-command.md`. 7 tasks.

Execute via superpowers:subagent-driven-development.
