# jianwu S4: Scaffolding Phase Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the Scaffolding phase of jianwu's 4-stage LLM engine — N chapters in parallel, each generating `abstract`, `key_concepts`, `learning_objectives`, `suggested_examples`. Parallel via `errgroup.SetLimit` (per Q12). Continue-on-error with `--retry-failed` recovery. Per-chapter stateless.

**Architecture:** Per-chapter generation uses the same prompt-injection pattern as S3 outline (archetype YAML + style samples + chapter context from outline). `errgroup.SetLimit(n)` bounds concurrency (default 5, per Q12.A1). Each chapter is independent — failures don't abort siblings. Failed chapters marked `status=failed`; caller can re-run them. JSON Schema enforces per-chapter structured output.

**Tech Stack:** Go 1.22+, S2 provider layer, S3 prompt+schema patterns, `golang.org/x/sync/errgroup` (already pulled in).

## Global Constraints

- Go version floor: 1.22
- Module path: `github.com/zhurong/jianwu`
- License: AGPL-3.0
- TDD discipline (test-after for LLM-driven code)
- Scaffolding is **stateless per chapter**: each chapter call takes (archetype, outline context, chapter meta) → returns enriched chapter
- Parallel via `errgroup.SetLimit(n)` (default 5, per Q12.A1)
- Continue-on-error: one chapter failure doesn't abort siblings (per Q12.B2)
- `--retry-failed` recovery: public function to retry only `status=failed` chapters (per Q12.A3)
- Prompt injects: archetype YAML, style samples, chapter's part/role, chapter title (from outline)
- Structured output via JSON Schema for: abstract, key_concepts, learning_objectives, suggested_examples
- Commit after every task

---

## File Structure

| Path | Responsibility |
|---|---|
| `internal/engine/scaffolding/types.go` | `ChapterInput`, `ChapterOutput` (alias for `book.OutlineChapter`), promptData |
| `internal/engine/scaffolding/prompt/system.md.tmpl` | Per-chapter system prompt |
| `internal/engine/scaffolding/prompt/user.md.tmpl` | Per-chapter user prompt |
| `internal/engine/scaffolding/embed.go` | `//go:embed prompt/*.tmpl` |
| `internal/engine/scaffolding/schema.go` | JSON Schema for chapter output |
| `internal/engine/scaffolding/chapter.go` | `GenerateChapter(ctx, chatter, ChapterInput) (*book.OutlineChapter, error)` |
| `internal/engine/scaffolding/chapter_test.go` | Unit tests with mock |
| `internal/engine/scaffolding/scaffold.go` | `ScaffoldAll(ctx, chatter, outline, opts) (map[int]error)` — parallel orchestrator |
| `internal/engine/scaffolding/scaffold_test.go` | Tests for parallel + continue-on-error |
| `internal/engine/scaffolding/retry.go` | `RetryFailed(ctx, chatter, outline, ...) (map[int]error)` |
| `internal/engine/scaffolding/retry_test.go` | Tests for retry |
| `internal/engine/scaffolding/integration_test.go` | Live test (skips without API key) |

---

## Task 0: Skeleton + Templates + Schema

**Files:**
- Create: `internal/engine/scaffolding/` directory and files

- [ ] **Step 1: Create directory**

```bash
mkdir -p internal/engine/scaffolding/prompt
```

- [ ] **Step 2: Write `types.go`**

`internal/engine/scaffolding/types.go`:

```go
package scaffolding

import (
    "fmt"

    "github.com/zhurong/jianwu/internal/archetypes"
    "github.com/zhurong/jianwu/internal/book"
    "github.com/zhurong/jianwu/internal/style"
)

// ChapterInput is the input for generating one chapter's scaffold.
type ChapterInput struct {
    ArchetypeID string
    // Part context
    PartIndex int
    PartTitle string
    PartRole  string
    // Chapter context (from outline)
    ChapterIndex int
    ChapterTitle string
    // Book parameters
    Topic    string
    Audience string
    Depth    string
    Goal     string
    Length   string
    Language string
}

// ChapterOutput is the generated scaffold for one chapter.
// Aliased to book.OutlineChapter so callers can directly assign.
type ChapterOutput = book.OutlineChapter

// promptData is the template context.
type promptData struct {
    Archetype      string
    Samples        string
    PartRole       string
    PartTitle      string
    ChapterTitle   string
    Topic          string
    Audience       string
    Depth          string
    Goal           string
    Length         string
    Language       string
}

// buildPromptData assembles prompt data from a ChapterInput.
// Returns an error if archetype or samples can't be loaded.
func buildPromptData(in ChapterInput) (promptData, error) {
    if err := in.validate(); err != nil {
        return promptData{}, err
    }
    archs, err := archetypes.Load()
    if err != nil {
        return promptData{}, fmt.Errorf("load archetypes: %w", err)
    }
    arch, ok := archs[in.ArchetypeID]
    if !ok {
        return promptData{}, fmt.Errorf("archetype %q not found", in.ArchetypeID)
    }
    samples, err := style.LoadSamples()
    if err != nil {
        return promptData{}, fmt.Errorf("load samples: %w", err)
    }
    sampleText, ok := samples[in.ArchetypeID]
    if !ok {
        sampleText = "(no samples for this archetype)"
    }
    return promptData{
        Archetype:    yamlMarshalArchetype(arch),
        Samples:      sampleText,
        PartRole:     in.PartRole,
        PartTitle:    in.PartTitle,
        ChapterTitle: in.ChapterTitle,
        Topic:        in.Topic,
        Audience:     in.Audience,
        Depth:        in.Depth,
        Goal:         in.Goal,
        Length:       in.Length,
        Language:     in.Language,
    }, nil
}

func (in ChapterInput) validate() error {
    var missing []string
    if in.ArchetypeID == "" {
        missing = append(missing, "archetype_id")
    }
    if in.ChapterTitle == "" {
        missing = append(missing, "chapter_title")
    }
    if in.PartRole == "" {
        missing = append(missing, "part_role")
    }
    if in.Topic == "" {
        missing = append(missing, "topic")
    }
    if in.Language == "" {
        missing = append(missing, "language")
    }
    if len(missing) > 0 {
        return fmt.Errorf("missing required fields: %s", joinComma(missing))
    }
    return nil
}

func joinComma(xs []string) string {
    out := ""
    for i, x := range xs {
        if i > 0 {
            out += ", "
        }
        out += x
    }
    return out
}

// yamlMarshalArchetype produces a compact text rendering of an archetype.
// Same approach as outline package: minimal pretty-printer, not real YAML round-trip.
func yamlMarshalArchetype(a *archetypes.Archetype) string {
    out := ""
    out += "id: " + a.ID + "\n"
    out += "name_zh: " + a.Name.Zh + "\n"
    out += "name_en: " + a.Name.En + "\n"
    out += "description: " + a.Description + "\n"
    out += "\nparts:\n"
    for _, p := range a.Parts {
        out += "  - role: " + p.Role + "\n"
        out += "    guidance: " + p.Guidance + "\n"
        if len(p.ChapterRoleHints) > 0 {
            out += "    chapter_role_hints:\n"
            for _, h := range p.ChapterRoleHints {
                out += "      - " + h + "\n"
            }
        }
    }
    return out
}
```

- [ ] **Step 3: Write `embed.go`**

`internal/engine/scaffolding/embed.go`:

```go
package scaffolding

import "embed"

//go:embed prompt/*.tmpl
var promptFS embed.FS

func loadSystem() ([]byte, error) { return promptFS.ReadFile("prompt/system.md.tmpl") }
func loadUser() ([]byte, error)   { return promptFS.ReadFile("prompt/user.md.tmpl") }
```

- [ ] **Step 4: Write `system.md.tmpl`**

`internal/engine/scaffolding/prompt/system.md.tmpl`:

```
你是 jianwu 的 scaffolding 生成器。你的任务：为图书的某一章生成脚手架内容。

## 输出格式

严格按 JSON Schema 输出 JSON。Schema 描述了一章的脚手架字段：abstract（章节摘要）、
key_concepts（核心术语）、learning_objectives（学完这章读者能...）、suggested_examples
（建议的例子/案例/思想实验）。

不要输出 JSON 以外的内容。

## 内容要求

1. abstract 必须明确这一章在整本书中承担什么角色（与 part_role 和 chapter_role_hints 对应）。
2. key_concepts 列出 3-7 个该章必须出现的核心术语，每个术语用「」标记首次出现。
3. learning_objectives 用 2-4 条"读者能..."的陈述，描述读完这章应该掌握的能力/认知。
4. suggested_examples 列出 2-4 个该章可以使用的具体例子（可以是案例、思想实验、数据）。
5. 所有文本使用 {{.Language}} 语言。

## 风格规约（必须遵守）

{{.Samples}}

## Archetype 定义

{{.Archetype}}
```

- [ ] **Step 5: Write `user.md.tmpl`**

`internal/engine/scaffolding/prompt/user.md.tmpl`:

```
为以下章节生成脚手架。

图书主题：{{.Topic}}
受众：{{.Audience}}
深度：{{.Depth}}
目标：{{.Goal}}
篇幅：{{.Length}}
语言：{{.Language}}

该章所在部：{{.PartTitle}}（role: {{.PartRole}}）
章节标题：{{.ChapterTitle}}

按 schema 输出。abstract、key_concepts、learning_objectives、suggested_examples 都要填。
```

- [ ] **Step 6: Write `schema.go`**

`internal/engine/scaffolding/schema.go`:

```go
package scaffolding

import (
    "encoding/json"

    "github.com/invopop/jsonschema"
)

// chapterSchema describes the fields the LLM must populate for one chapter.
// It's a subset of book.OutlineChapter focused on scaffolding fields.
type chapterSchema struct {
    Abstract          string   `json:"abstract" jsonschema:"description=该章在整本书中承担的角色和核心论点"`
    KeyConcepts       []string `json:"key_concepts" jsonschema:"description=3-7 个核心术语"`
    LearningObjectives []string `json:"learning_objectives" jsonschema:"description=2-4 条'读者能...'陈述"`
    SuggestedExamples []string `json:"suggested_examples" jsonschema:"description=2-4 个例子/案例/思想实验"`
}

// JSONSchema returns the JSON Schema for chapter scaffolding output.
func JSONSchema() ([]byte, error) {
    r := new(jsonschema.Reflector)
    r.DoNotReference = true
    s := r.Reflect(&chapterSchema{})
    return json.Marshal(s)
}
```

- [ ] **Step 7: Verify build**

```bash
go build ./...
```

- [ ] **Step 8: Commit**

```bash
git add internal/engine/scaffolding/
git commit -m "feat(scaffolding): types, prompt templates, JSON schema"
```

---

## Task 1: Chapter Generator

**Files:**
- Create: `internal/engine/scaffolding/chapter.go`
- Create: `internal/engine/scaffolding/chapter_test.go`

- [ ] **Step 1: Write `chapter.go`**

```go
package scaffolding

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "text/template"

    "github.com/zhurong/jianwu/internal/book"
    "github.com/zhurong/jianwu/internal/provider/llm"
)

// GenerateChapter produces a scaffold for one chapter via LLM call.
func GenerateChapter(ctx context.Context, chatter llm.Chatter, in ChapterInput) (*ChapterOutput, error) {
    data, err := buildPromptData(in)
    if err != nil {
        return nil, err
    }
    sysBytes, err := loadSystem()
    if err != nil {
        return nil, fmt.Errorf("load system template: %w", err)
    }
    userBytes, err := loadUser()
    if err != nil {
        return nil, fmt.Errorf("load user template: %w", err)
    }
    sys, err := renderTemplate("system", sysBytes, data)
    if err != nil {
        return nil, err
    }
    user, err := renderTemplate("user", userBytes, data)
    if err != nil {
        return nil, err
    }
    schema, err := JSONSchema()
    if err != nil {
        return nil, fmt.Errorf("generate schema: %w", err)
    }

    req := llm.ChatRequest{
        Messages: []llm.Message{
            {Role: "system", Content: sys},
            {Role: "user", Content: user},
        },
        JSONSchema: schema,
    }
    resp, err := chatter.Chat(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("llm chat: %w", err)
    }

    var parsed chapterSchema
    if err := json.Unmarshal([]byte(resp.Content), &parsed); err != nil {
        return nil, fmt.Errorf("parse chapter JSON: %w (content was: %s)", err, truncate(resp.Content, 500))
    }
    return &ChapterOutput{
        Abstract:           parsed.Abstract,
        KeyConcepts:        parsed.KeyConcepts,
        LearningObjectives: parsed.LearningObjectives,
        SuggestedExamples:  parsed.SuggestedExamples,
        Status:             book.StatusScaffolded,
    }, nil
}

func renderTemplate(name string, raw []byte, data any) (string, error) {
    tmpl, err := template.New(name).Parse(string(raw))
    if err != nil {
        return "", fmt.Errorf("parse %s template: %w", name, err)
    }
    var buf strings.Builder
    if err := tmpl.Execute(&buf, data); err != nil {
        return "", fmt.Errorf("execute %s template: %w", name, err)
    }
    return buf.String(), nil
}

func truncate(s string, n int) string {
    if len(s) <= n {
        return s
    }
    return s[:n] + "..."
}
```

- [ ] **Step 2: Write tests**

`internal/engine/scaffolding/chapter_test.go`:

```go
package scaffolding

import (
    "context"
    "testing"

    "github.com/zhurong/jianwu/internal/book"
    "github.com/zhurong/jianwu/internal/provider/llm"
    "github.com/zhurong/jianwu/internal/provider/llm/mock"
)

func TestGenerateChapterValidatesInput(t *testing.T) {
    _, err := GenerateChapter(context.Background(), mock.New(llm.ChatResponse{}), ChapterInput{})
    if err == nil {
        t.Fatal("expected validation error")
    }
}

func TestGenerateChapterParsesResponse(t *testing.T) {
    sample := `{
        "abstract": "本章界定时间的本体地位。",
        "key_concepts": ["可能性基底", "收敛", "观察"],
        "learning_objectives": ["理解时间不是流动的", "区分时间与变化"],
        "suggested_examples": ["Zeno 悖论", "量子测量实验"]
    }`
    p := mock.New(llm.ChatResponse{Content: sample})
    out, err := GenerateChapter(context.Background(), p, ChapterInput{
        ArchetypeID:  "ontology-epistemology-practice",
        PartIndex:    1,
        PartTitle:    "第一部 本体",
        PartRole:     "ontology",
        ChapterIndex: 1,
        ChapterTitle: "第一章 可能性基底",
        Topic:        "时间的实在",
        Audience:     "educated-general",
        Depth:        "advanced",
        Goal:         "understanding",
        Length:       "long",
        Language:     "zh",
    })
    if err != nil {
        t.Fatalf("GenerateChapter: %v", err)
    }
    if out.Abstract != "本章界定时间的本体地位。" {
        t.Errorf("abstract: %q", out.Abstract)
    }
    if len(out.KeyConcepts) != 3 {
        t.Errorf("concepts: %d", len(out.KeyConcepts))
    }
    if out.Status != book.StatusScaffolded {
        t.Errorf("status: %q", out.Status)
    }
}

func TestGenerateChapterRejectsMalformedJSON(t *testing.T) {
    p := mock.New(llm.ChatResponse{Content: "not json"})
    _, err := GenerateChapter(context.Background(), p, ChapterInput{
        ArchetypeID:  "ontology-epistemology-practice",
        PartRole:     "ontology",
        ChapterTitle: "X",
        Topic:        "X",
        Language:     "zh",
    })
    if err == nil {
        t.Fatal("expected parse error")
    }
}
```

- [ ] **Step 3: Run tests, verify pass**

- [ ] **Step 4: Commit**

```bash
git add internal/engine/scaffolding/chapter.go internal/engine/scaffolding/chapter_test.go
git commit -m "feat(scaffolding): GenerateChapter with archetype+samples injection"
```

---

## Task 2: Parallel Orchestrator (ScaffoldAll)

**Files:**
- Create: `internal/engine/scaffolding/scaffold.go`
- Create: `internal/engine/scaffolding/scaffold_test.go`

- [ ] **Step 1: Write `scaffold.go`**

```go
package scaffolding

import (
    "context"
    "fmt"
    "sync"

    "github.com/zhurong/jianwu/internal/book"
    "github.com/zhurong/jianwu/internal/provider/llm"
    "golang.org/x/sync/errgroup"
)

// Options controls parallel scaffolding behavior.
type Options struct {
    // Concurrency limits parallel LLM calls. Default 5 per Q12.A1.
    Concurrency int
}

// Result captures the outcome of scaffolding one chapter.
type Result struct {
    PartIndex    int
    ChapterIndex int
    Chapter      *ChapterOutput
    Err          error
}

// ScaffoldAll runs GenerateChapter for every chapter in the outline, in parallel.
// Returns a map keyed by "partIndex-chapterIndex" with each chapter's result.
// Chapters that succeed have their book.OutlineChapter fields populated (in-place update).
// Chapters that fail have status=failed set on the outline entry and Err set in the result map.
//
// Per Q12.B2: continue-on-error. One failure does NOT abort other chapters.
func ScaffoldAll(
    ctx context.Context,
    chatter llm.Chatter,
    outline *book.Outline,
    archetypeID string,
    params ChapterParams,
    opts Options,
) map[string]Result {
    if opts.Concurrency <= 0 {
        opts.Concurrency = 5
    }

    // Collect all chapter inputs up-front.
    type job struct {
        key    string
        partIdx int
        chIdx   int
        input   ChapterInput
    }
    var jobs []job
    for _, p := range outline.Parts {
        for _, c := range p.Chapters {
            input := ChapterInput{
                ArchetypeID:  archetypeID,
                PartIndex:    p.Index,
                PartTitle:    p.Title,
                PartRole:     p.Role,
                ChapterIndex: c.Index,
                ChapterTitle: c.Title,
                Topic:        params.Topic,
                Audience:     params.Audience,
                Depth:        params.Depth,
                Goal:         params.Goal,
                Length:       params.Length,
                Language:     params.Language,
            }
            jobs = append(jobs, job{
                key:    fmtKey(p.Index, c.Index),
                partIdx: p.Index,
                chIdx:   c.Index,
                input:   input,
            })
        }
    }

    results := make(map[string]Result, len(jobs))
    var mu sync.Mutex

    g, gctx := errgroup.WithContext(ctx)
    g.SetLimit(opts.Concurrency)

    for _, j := range jobs {
        j := j
        g.Go(func() error {
            // Note: we use gctx for cancellation propagation but each chapter
            // still attempts even if a sibling failed (continue-on-error).
            // errgroup normally cancels on first error; we work around this by
            // always returning nil from g.Go (errors are captured per-chapter).
            chapCtx := gctx
            // If errgroup already cancelled due to context cancel, skip.
            if err := gctx.Err(); err != nil {
                mu.Lock()
                results[j.key] = Result{PartIndex: j.partIdx, ChapterIndex: j.chIdx, Err: err}
                mu.Unlock()
                return nil
            }

            out, err := GenerateChapter(chapCtx, chatter, j.input)

            mu.Lock()
            defer mu.Unlock()
            if err != nil {
                results[j.key] = Result{
                    PartIndex: j.partIdx, ChapterIndex: j.chIdx, Err: err,
                }
                return nil // don't propagate — continue-on-error
            }
            results[j.key] = Result{
                PartIndex: j.partIdx, ChapterIndex: j.chIdx, Chapter: out,
            }
            return nil
        })
    }
    _ = g.Wait()

    // Apply successful results back to the outline.
    for i := range outline.Parts {
        for j := range outline.Parts[i].Chapters {
            c := &outline.Parts[i].Chapters[j]
            key := fmtKey(outline.Parts[i].Index, c.Index)
            r, ok := results[key]
            if !ok {
                continue
            }
            if r.Err != nil {
                c.Status = book.StatusFailed
                continue
            }
            c.Abstract = r.Chapter.Abstract
            c.KeyConcepts = r.Chapter.KeyConcepts
            c.LearningObjectives = r.Chapter.LearningObjectives
            c.SuggestedExamples = r.Chapter.SuggestedExamples
            c.Status = book.StatusScaffolded
        }
    }
    return results
}

// ChapterParams is the book-level context (topic, audience, depth, goal, length, language).
type ChapterParams struct {
    Topic    string
    Audience string
    Depth    string
    Goal     string
    Length   string
    Language string
}

func fmtKey(partIdx, chIdx int) string {
    return fmt.Sprintf("%d-%d", partIdx, chIdx)
}
```

- [ ] **Step 2: Write tests**

`scaffold_test.go`:

```go
package scaffolding

import (
    "context"
    "errors"
    "testing"

    "github.com/zhurong/jianwu/internal/book"
    "github.com/zhurong/jianwu/internal/provider/llm"
    "github.com/zhurong/jianwu/internal/provider/llm/mock"
)

func TestScaffoldAllUpdatesOutline(t *testing.T) {
    outline := &book.Outline{
        Parts: []book.OutlinePart{
            {Index: 1, Title: "P1", Role: "ontology", Chapters: []book.OutlineChapter{
                {Index: 1, Title: "C1"},
                {Index: 2, Title: "C2"},
            }},
        },
    }
    sample := `{"abstract":"X","key_concepts":["a"],"learning_objectives":["y"],"suggested_examples":["z"]}`
    p := mock.New(llm.ChatResponse{Content: sample})
    results := ScaffoldAll(context.Background(), p, outline, "ontology-epistemology-practice",
        ChapterParams{Topic: "T", Audience: "scholar", Depth: "advanced", Goal: "understanding", Length: "long", Language: "zh"},
        Options{Concurrency: 2})
    if len(results) != 2 {
        t.Fatalf("got %d results, want 2", len(results))
    }
    for _, c := range outline.Parts[0].Chapters {
        if c.Abstract != "X" {
            t.Errorf("chapter %d abstract: %q", c.Index, c.Abstract)
        }
        if c.Status != book.StatusScaffolded {
            t.Errorf("chapter %d status: %q", c.Index, c.Status)
        }
    }
}

func TestScaffoldAllContinueOnError(t *testing.T) {
    outline := &book.Outline{
        Parts: []book.OutlinePart{
            {Index: 1, Title: "P1", Role: "ontology", Chapters: []book.OutlineChapter{
                {Index: 1, Title: "C1"},
                {Index: 2, Title: "C2"},
                {Index: 3, Title: "C3"},
            }},
        },
    }
    // Always-error chatter
    p := mock.NewError(errors.New("LLM down"))
    results := ScaffoldAll(context.Background(), p, outline, "ontology-epistemology-practice",
        ChapterParams{Topic: "T", Audience: "scholar", Depth: "advanced", Goal: "understanding", Length: "long", Language: "zh"},
        Options{Concurrency: 2})
    if len(results) != 3 {
        t.Fatalf("got %d results, want 3", len(results))
    }
    for _, c := range outline.Parts[0].Chapters {
        if c.Status != book.StatusFailed {
            t.Errorf("chapter %d status: %q (want failed)", c.Index, c.Status)
        }
    }
}

func TestScaffoldAllEmptyOutlineNoOp(t *testing.T) {
    outline := &book.Outline{}
    p := mock.New(llm.ChatResponse{Content: "{}"})
    results := ScaffoldAll(context.Background(), p, outline, "x",
        ChapterParams{Topic: "T", Audience: "scholar", Depth: "advanced", Goal: "understanding", Length: "long", Language: "zh"},
        Options{})
    if len(results) != 0 {
        t.Errorf("got %d results", len(results))
    }
}
```

- [ ] **Step 3: Run tests, verify pass**

- [ ] **Step 4: Commit**

```bash
git add internal/engine/scaffolding/scaffold.go internal/engine/scaffolding/scaffold_test.go
git commit -m "feat(scaffolding): parallel ScaffoldAll with continue-on-error (Q12)"
```

---

## Task 3: Retry-Failed

**Files:**
- Create: `internal/engine/scaffolding/retry.go`
- Create: `internal/engine/scaffolding/retry_test.go`

- [ ] **Step 1: Write `retry.go`**

```go
package scaffolding

import (
    "context"

    "github.com/zhurong/jianwu/internal/book"
    "github.com/zhurong/jianwu/internal/provider/llm"
)

// RetryFailed re-runs GenerateChapter only for chapters whose status is book.StatusFailed.
// Returns a result map (same shape as ScaffoldAll) for the retried chapters only.
func RetryFailed(
    ctx context.Context,
    chatter llm.Chatter,
    outline *book.Outline,
    archetypeID string,
    params ChapterParams,
    opts Options,
) map[string]Result {
    if opts.Concurrency <= 0 {
        opts.Concurrency = 5
    }
    // Build a temporary outline containing only failed chapters.
    filtered := &book.Outline{}
    for _, p := range outline.Parts {
        fp := book.OutlinePart{Index: p.Index, Title: p.Title, Role: p.Role}
        for _, c := range p.Chapters {
            if c.Status == book.StatusFailed {
                fp.Chapters = append(fp.Chapters, c)
            }
        }
        if len(fp.Chapters) > 0 {
            filtered.Parts = append(filtered.Parts, fp)
        }
    }
    if len(filtered.Parts) == 0 {
        return map[string]Result{}
    }
    return ScaffoldAll(ctx, chatter, filtered, archetypeID, params, opts)
    // Note: ScaffoldAll only updates the filtered outline. Caller must merge back.
}
```

**Important design note:** `RetryFailed` scaffolds against a *filtered* outline, so `ScaffoldAll`'s in-place update doesn't touch the original. The caller (CLI) must merge results back. This is awkward.

**Simpler alternative:** Make `ScaffoldAll` accept the outline and only update chapters that need it. But that changes ScaffoldAll's contract.

**Cleanest fix:** Have RetryFailed iterate results and apply them to the original outline directly:

Replace the body with:

```go
func RetryFailed(
    ctx context.Context,
    chatter llm.Chatter,
    outline *book.Outline,
    archetypeID string,
    params ChapterParams,
    opts Options,
) map[string]Result {
    // Collect failed-chapter jobs.
    type job struct {
        key    string
        partIdx int
        chIdx   int
        input   ChapterInput
    }
    var jobs []job
    for _, p := range outline.Parts {
        for _, c := range p.Chapters {
            if c.Status != book.StatusFailed {
                continue
            }
            input := ChapterInput{
                ArchetypeID:  archetypeID,
                PartIndex:    p.Index,
                PartTitle:    p.Title,
                PartRole:     p.Role,
                ChapterIndex: c.Index,
                ChapterTitle: c.Title,
                Topic:        params.Topic,
                Audience:     params.Audience,
                Depth:        params.Depth,
                Goal:         params.Goal,
                Length:       params.Length,
                Language:     params.Language,
            }
            jobs = append(jobs, job{
                key: fmtKey(p.Index, c.Index),
                partIdx: p.Index, chIdx: c.Index,
                input: input,
            })
        }
    }
    if len(jobs) == 0 {
        return map[string]Result{}
    }

    // Reuse ScaffoldAll's parallel machinery by building a temp outline.
    filtered := &book.Outline{}
    partMap := map[int]*book.OutlinePart{} // tracks part index → pointer in filtered
    for _, p := range outline.Parts {
        // Only include this part if it has at least one failed chapter.
        hasFailed := false
        for _, c := range p.Chapters {
            if c.Status == book.StatusFailed {
                hasFailed = true
                break
            }
        }
        if !hasFailed {
            continue
        }
        fp := book.OutlinePart{Index: p.Index, Title: p.Title, Role: p.Role}
        for _, c := range p.Chapters {
            if c.Status == book.StatusFailed {
                fp.Chapters = append(fp.Chapters, c)
            }
        }
        filtered.Parts = append(filtered.Parts, fp)
        partMap[p.Index] = &filtered.Parts[len(filtered.Parts)-1]
    }

    results := ScaffoldAll(ctx, chatter, filtered, archetypeID, params, opts)

    // Merge filtered results back into the original outline.
    for i := range outline.Parts {
        for j := range outline.Parts[i].Chapters {
            c := &outline.Parts[i].Chapters[j]
            if c.Status != book.StatusFailed {
                continue
            }
            key := fmtKey(outline.Parts[i].Index, c.Index)
            r, ok := results[key]
            if !ok {
                continue
            }
            if r.Err == nil && r.Chapter != nil {
                c.Abstract = r.Chapter.Abstract
                c.KeyConcepts = r.Chapter.KeyConcepts
                c.LearningObjectives = r.Chapter.LearningObjectives
                c.SuggestedExamples = r.Chapter.SuggestedExamples
                c.Status = book.StatusScaffolded
            }
            // else: leave as failed; result map carries the new error
        }
    }
    return results
}
```

- [ ] **Step 2: Write tests**

`retry_test.go`:

```go
package scaffolding

import (
    "context"
    "errors"
    "testing"

    "github.com/zhurong/jianwu/internal/book"
    "github.com/zhurong/jianwu/internal/provider/llm"
    "github.com/zhurong/jianwu/internal/provider/llm/mock"
)

func TestRetryFailedOnlyTouchesFailedChapters(t *testing.T) {
    outline := &book.Outline{
        Parts: []book.OutlinePart{
            {Index: 1, Title: "P1", Role: "ontology", Chapters: []book.OutlineChapter{
                {Index: 1, Title: "C1", Status: book.StatusScaffolded, Abstract: "already done"},
                {Index: 2, Title: "C2", Status: book.StatusFailed},
            }},
        },
    }
    sample := `{"abstract":"recovered","key_concepts":["a"],"learning_objectives":["y"],"suggested_examples":["z"]}`
    p := mock.New(llm.ChatResponse{Content: sample})

    results := RetryFailed(context.Background(), p, outline, "ontology-epistemology-practice",
        ChapterParams{Topic: "T", Audience: "scholar", Depth: "advanced", Goal: "understanding", Length: "long", Language: "zh"},
        Options{})

    if len(results) != 1 {
        t.Fatalf("got %d results, want 1 (only failed)", len(results))
    }
    // Original successful chapter untouched.
    if outline.Parts[0].Chapters[0].Abstract != "already done" {
        t.Errorf("existing chapter was modified: %q", outline.Parts[0].Chapters[0].Abstract)
    }
    // Failed chapter recovered.
    if outline.Parts[0].Chapters[1].Status != book.StatusScaffolded {
        t.Errorf("failed chapter not recovered: %q", outline.Parts[0].Chapters[1].Status)
    }
    if outline.Parts[0].Chapters[1].Abstract != "recovered" {
        t.Errorf("recovered abstract: %q", outline.Parts[0].Chapters[1].Abstract)
    }
}

func TestRetryFailedNoFailedChaptersIsNoOp(t *testing.T) {
    outline := &book.Outline{
        Parts: []book.OutlinePart{
            {Index: 1, Title: "P1", Role: "ontology", Chapters: []book.OutlineChapter{
                {Index: 1, Title: "C1", Status: book.StatusScaffolded},
            }},
        },
    }
    p := mock.New(llm.ChatResponse{Content: "{}"})
    results := RetryFailed(context.Background(), p, outline, "x",
        ChapterParams{Topic: "T", Audience: "scholar", Depth: "advanced", Goal: "understanding", Length: "long", Language: "zh"},
        Options{})
    if len(results) != 0 {
        t.Errorf("expected 0 results, got %d", len(results))
    }
}

func TestRetryFailedStillFailsReturnsErrorInResult(t *testing.T) {
    outline := &book.Outline{
        Parts: []book.OutlinePart{
            {Index: 1, Title: "P1", Role: "ontology", Chapters: []book.OutlineChapter{
                {Index: 1, Title: "C1", Status: book.StatusFailed},
            }},
        },
    }
    p := mock.NewError(errors.New("still down"))
    results := RetryFailed(context.Background(), p, outline, "ontology-epistemology-practice",
        ChapterParams{Topic: "T", Audience: "scholar", Depth: "advanced", Goal: "understanding", Length: "long", Language: "zh"},
        Options{})
    if len(results) != 1 {
        t.Fatalf("got %d results", len(results))
    }
    if outline.Parts[0].Chapters[0].Status != book.StatusFailed {
        t.Errorf("status should remain failed: %q", outline.Parts[0].Chapters[0].Status)
    }
}
```

- [ ] **Step 3: Run tests, verify pass**

- [ ] **Step 4: Commit**

```bash
git add internal/engine/scaffolding/retry.go internal/engine/scaffolding/retry_test.go
git commit -m "feat(scaffolding): RetryFailed for --retry-failed recovery (Q12.A3)"
```

---

## Task 4: Live Integration Test

**Files:**
- Create: `internal/engine/scaffolding/integration_test.go`

- [ ] **Step 1: Write integration test**

```go
package scaffolding

import (
    "context"
    "os"
    "testing"
    "time"

    "github.com/zhurong/jianwu/internal/book"
    "github.com/zhurong/jianwu/internal/provider/llm"
    "github.com/zhurong/jianwu/internal/provider/llm/gemini"
    "github.com/zhurong/jianwu/internal/provider/llm/glm"
)

func TestGenerateChapterLiveGemini(t *testing.T) {
    key := os.Getenv("GEMINI_API_KEY")
    if key == "" {
        t.Skip("GEMINI_API_KEY not set")
    }
    p, err := gemini.New(gemini.Config{APIKey: key})
    if err != nil {
        t.Fatal(err)
    }
    runLiveChapter(t, p)
}

func TestGenerateChapterLiveGLM(t *testing.T) {
    key := os.Getenv("GLM_API_KEY")
    if key == "" {
        t.Skip("GLM_API_KEY not set")
    }
    p, err := glm.New(glm.Config{APIKey: key})
    if err != nil {
        t.Fatal(err)
    }
    runLiveChapter(t, p)
}

func runLiveChapter(t *testing.T, chatter llm.Chatter) {
    t.Helper()
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    out, err := GenerateChapter(ctx, chatter, ChapterInput{
        ArchetypeID:  "ontology-epistemology-practice",
        PartIndex:    1,
        PartTitle:    "第一部 本体",
        PartRole:     "ontology",
        ChapterIndex: 1,
        ChapterTitle: "第一章 可能性基底",
        Topic:        "人工智能时代的真实与虚幻",
        Audience:     "educated-general",
        Depth:        "intermediate",
        Goal:         "understanding",
        Length:       "medium",
        Language:     "zh",
    })
    if err != nil {
        t.Fatalf("GenerateChapter: %v", err)
    }
    t.Logf("abstract: %s", out.Abstract)
    t.Logf("key_concepts: %v", out.KeyConcepts)
    if out.Status != book.StatusScaffolded {
        t.Errorf("status: %q", out.Status)
    }
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/engine/scaffolding/integration_test.go
git commit -m "test(scaffolding): live integration tests for Gemini + GLM"
```

---

## Task 5: README + Version Bump

**Files:**
- Modify: `README.md`
- Modify: `internal/cli/version.go`

- [ ] **Step 1: Bump version**

`internal/cli/version.go`:

```go
package cli

var Version = "0.4.0"
```

- [ ] **Step 2: Update README Engine section**

Replace the v0.3.0 Engine section with:

```markdown

## Engine (v0.4.0)

The 4-stage engine is being built slice by slice. v0.4.0 ships **Outline + Scaffolding**:

- **Outline** (v0.3.0): single LLM call produces full book outline (parts × chapters)
- **Scaffolding** (v0.4.0): N chapters in parallel (default concurrency 5), each generates abstract / key_concepts / learning_objectives / suggested_examples. Continue-on-error: failed chapters marked `status=failed` without aborting siblings. `RetryFailed` re-runs only failed chapters.

Both stages are stateless per call. Caller (S6 `new` command) will wrap with RetryWrapper + FallbackWrapper.

Remaining stages (deferred):
- Grill (interactive stateful, S5)
- Expand (agent loop + web search, S7)
```

- [ ] **Step 3: Final sweep**

```bash
go test ./...
go vet ./...
find . -name '*.go' -not -path './vendor/*' | xargs gofmt -l
```

- [ ] **Step 4: Commit + tag**

```bash
git add README.md internal/cli/version.go
git commit -m "docs: v0.4.0 README + version bump (S4 scaffolding complete)"
git tag v0.4.0
```

---

## Self-Review

**Spec coverage:**
- Q9 (prompt templates embedded .md.tmpl): Task 0
- Q10 (structured output via JSON Schema): Tasks 0, 1
- Q12 (parallel concurrency, continue-on-error, retry-failed): Tasks 2, 3
- Q15 (test-after for LLM code): Tasks 1, 2, 3, 4
- Q18 (archetype骨架+填充): Task 1's buildPromptData

**Deferrals:**
- No CLI command for scaffolding (S6 `new` will chain grill → outline → scaffolding)
- No retry/fallback wrapping inside scaffolding package (caller wraps)
- Prompt templates are v1
- RetryFailed merges results back via a filtered-outline round-trip — works but slightly awkward; could be cleaner if ScaffoldAll took a "skip if not failed" filter, but the current shape is fine for v0.4.0

**Placeholder scan:** clean. RetryFailed has an "Important design note" in the brief that presents the awkward first version then the cleaner second version — implementer should use the SECOND (cleaner) version.

**Type consistency:**
- `ChapterInput` defined Task 0, used Tasks 1, 2, 3 ✓
- `ChapterOutput = book.OutlineChapter` alias ✓
- `ChapterParams` defined Task 2, used Task 3 ✓
- `Options` defined Task 2, used Task 3 ✓
- `Result` defined Task 2, used Task 3 ✓

---

## Execution Handoff

Plan saved to `docs/superpowers/plans/2026-06-22-s4-scaffolding.md`. 6 tasks.

Execute via superpowers:subagent-driven-development.
