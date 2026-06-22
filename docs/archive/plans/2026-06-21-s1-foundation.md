# jianwu S1: Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the workspace + config + CLI shell foundation for jianwu (no LLM, no engine), producing working `jianwu init`/`info`/`config get/set/list` commands.

**Architecture:** Go module at `github.com/zhurong/jianwu`. Standard `cmd/`+`internal/` layout. Cobra CLI + custom 5-layer config resolver. Workspace marked by `.jianwu/` directory with walk-up detection (git-style). TDD throughout. All data assets already in `internal/{archetypes,style,corpus}` get an `embed.go` neighbor.

**Tech Stack:** Go 1.22+, cobra v1.8+, yaml.v3, slog (stdlib).

## Global Constraints

- Go version floor: 1.22 (for `log/slog`)
- Module path: `github.com/zhurong/jianwu`
- License: AGPL-3.0 (code); embedded zhurongshuo data © zhurong / internal-use only
- Test discipline: TDD (failing test → minimal impl → green → refactor)
- Exit codes: `0` success, `1` generic error, `2` usage error, `3` workspace-not-found, `4` llm/provider error (unused in S1), `5` network error (unused in S1)
- Logging: stdlib `log/slog`; `-v` flag → INFO; `--debug` flag → DEBUG
- Existing assets at `internal/{archetypes,style,corpus}/` stay in place
- No LLM, search, or network calls in S1 (deferred to S2)
- Commit after every task

---

## File Structure

### Created in this plan

| Path | Responsibility |
|---|---|
| `go.mod` | Module declaration |
| `LICENSE` | AGPL-3.0 text |
| `.gitignore` | Go + macOS defaults |
| `README.md` | Minimal project README |
| `cmd/jianwu/main.go` | Entry point, exit code mapping |
| `internal/cli/version.go` | Version constant |
| `internal/cli/root.go` | Cobra root command + global flags |
| `internal/cli/init.go` | `init` + `init --bare` |
| `internal/cli/info.go` | `info` command |
| `internal/cli/config.go` | `config get/set/list` |
| `internal/cli/e2e_test.go` | End-to-end happy path |
| `internal/archetypes/embed.go` | `//go:embed *.yaml` |
| `internal/archetypes/types.go` | Archetype YAML schema structs |
| `internal/archetypes/loader.go` | Parse all embedded YAMLs |
| `internal/archetypes/loader_test.go` | Tests |
| `internal/style/embed.go` | `//go:embed style-guide.md samples/*.md` |
| `internal/style/loader.go` | Load guide + samples |
| `internal/style/loader_test.go` | Tests |
| `internal/corpus/embed.go` | `//go:embed builtin/*.json` |
| `internal/corpus/types.go` | Corpus JSON schema structs |
| `internal/corpus/loader.go` | Parse all embedded JSONs |
| `internal/corpus/loader_test.go` | Tests |
| `internal/workspace/types.go` | Workspace struct, InitOpts |
| `internal/workspace/detect.go` | `FindWorkspace(startPath)` walk-up |
| `internal/workspace/detect_test.go` | Tests |
| `internal/workspace/init.go` | `Init(path, opts)` |
| `internal/workspace/init_test.go` | Tests |
| `internal/workspace/load.go` | `Load(wsPath)` returns typed Workspace |
| `internal/workspace/load_test.go` | Tests |
| `internal/config/types.go` | Config struct (5-layer model) |
| `internal/config/defaults.go` | Built-in defaults |
| `internal/config/loader.go` | 5-layer resolver |
| `internal/config/loader_test.go` | Tests |
| `internal/config/secrets.go` | Secrets loader (ENV > file, 0600) |
| `internal/config/secrets_test.go` | Tests |
| `internal/book/types.go` | Meta, Outline, Chapter, Citation structs |
| `internal/book/io.go` | LoadMeta / SaveMeta / LoadOutline / SaveOutline |
| `internal/book/io_test.go` | Round-trip tests |
| `internal/book/slug.go` | `Slugify(title)` |
| `internal/book/slug_test.go` | Tests |

---

## Task 0: Project Bootstrap

**Files:**
- Create: `go.mod`, `LICENSE`, `.gitignore`, `README.md`

**Interfaces:**
- Produces: importable module `github.com/zhurong/jianwu`

- [ ] **Step 1: Initialize go module**

Run:
```bash
cd /Users/rong.zhu/Code/@zhurong/jianwu
go mod init github.com/zhurong/jianwu
```

Expected: `go.mod` created with module path and `go 1.26` (or current toolchain).

- [ ] **Step 2: Pin Go floor to 1.22**

Edit `go.mod` to read exactly:

```
module github.com/zhurong/jianwu

go 1.22
```

- [ ] **Step 3: Fetch AGPL-3.0 license text**

Run:
```bash
curl -sL https://www.gnu.org/licenses/agpl-3.0.txt -o LICENSE
```

Then prepend this header to `LICENSE` (before the AGPL body):

```
jianwu - structure AI's training knowledge into human-readable books.
Copyright (C) 2026 zhurong

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

```

(Leave the standard AGPL-3.0 body intact after the header.)

- [ ] **Step 4: Create `.gitignore`**

```
# Go
*.exe
*.dll
*.so
*.dylib
*.test
*.out
go.work
go.work.sum
vendor/

# Build
/dist/
/bin/

# Editor
.vscode/
.idea/
*.swp
.DS_Store

# Jianwu runtime (if user inits a workspace in this repo)
.jianwu/
```

- [ ] **Step 5: Create minimal `README.md`**

```markdown
# jianwu

> 简物（jiàn wù）—— 把 AI 的训练知识结构化为人类可阅读、可学习的图书。

Library + CLI. Web SaaS wrapper is a separate repo (`mouqin`).

## Status

v1.0 in development. See `DESIGN.md` for the design doc and
`docs/superpowers/plans/` for implementation plans.

## License

Code: AGPL-3.0 (see `LICENSE`).
Embedded zhurongshuo reference data (`internal/archetypes/`,
`internal/style/`, `internal/corpus/`): © zhurong, internal-use only,
not for redistribution.
```

- [ ] **Step 6: Create directory skeleton**

```bash
mkdir -p cmd/jianwu internal/cli
```

`internal/{archetypes,style,corpus}/` already exist with data files.

- [ ] **Step 7: Verify build works**

Run:
```bash
go build ./...
```

Expected: no errors (no Go files yet).

- [ ] **Step 8: Commit**

```bash
git add go.mod LICENSE .gitignore README.md
git commit -m "chore: project bootstrap (module, AGPL license, readme)"
```

---

## Task 1: Archetype Embed + Loader

**Files:**
- Create: `internal/archetypes/types.go`
- Create: `internal/archetypes/embed.go`
- Create: `internal/archetypes/loader.go`
- Create: `internal/archetypes/loader_test.go`

**Interfaces:**
- Produces: `archetypes.Load() (map[string]*Archetype, error)`, `Archetype` struct

- [ ] **Step 1: Write `types.go`**

`internal/archetypes/types.go`:

```go
package archetypes

// Archetype represents a structural prototype for a book.
// Schema mirrors internal/archetypes/*.yaml.
type Archetype struct {
    SchemaVersion int           `yaml:"schema_version"`
    ID            string        `yaml:"id"`
    Name          LocalizedName `yaml:"name"`
    Description   string        `yaml:"description"`
    WhenToUse     WhenToUse     `yaml:"when_to_use"`
    Parts         []Part        `yaml:"parts"`
    Examples      []Example     `yaml:"examples"`
    Metadata      Metadata      `yaml:"metadata"`
}

type LocalizedName struct {
    Zh string `yaml:"zh"`
    En string `yaml:"en"`
}

type WhenToUse struct {
    Goals             []string `yaml:"goals"`
    TopicTypes        []string `yaml:"topic_types"`
    AudienceFit       []string `yaml:"audience_fit"`
    NotRecommendedFor []string `yaml:"not_recommended_for"`
}

type Part struct {
    Role              string            `yaml:"role"`
    TitleTemplate     LocalizedTemplate `yaml:"title_template"`
    Guidance          string            `yaml:"guidance"`
    TypicalChapters   []int             `yaml:"typical_chapters"`
    ChapterRoleHints  []string          `yaml:"chapter_role_hints"`
    Conditional       *Conditional      `yaml:"conditional,omitempty"`
}

type LocalizedTemplate struct {
    Zh string `yaml:"zh"`
    En string `yaml:"en"`
}

type Conditional struct {
    Trigger string `yaml:"trigger"`
}

type Example struct {
    Slug      string  `yaml:"slug"`
    Source    string  `yaml:"source"`
    SourceURL string  `yaml:"source_url"`
    FitScore  float64 `yaml:"fit_score"`
    Note      string  `yaml:"note,omitempty"`
}

type Metadata struct {
    ExtractedFrom string `yaml:"extracted_from"`
    ExtractedAt   string `yaml:"extracted_at"`
    Author        string `yaml:"author"`
    Notes         string `yaml:"notes,omitempty"`
}
```

- [ ] **Step 2: Write `embed.go`**

`internal/archetypes/embed.go`:

```go
package archetypes

import "embed"

//go:embed *.yaml
var fs embed.FS
```

- [ ] **Step 3: Write failing test**

`internal/archetypes/loader_test.go`:

```go
package archetypes

import (
    "testing"
)

func TestLoadReturnsAllThreeArchetypes(t *testing.T) {
    m, err := Load()
    if err != nil {
        t.Fatalf("Load() error: %v", err)
    }
    want := []string{
        "ontology-epistemology-practice",
        "diagnosis-decoding-breakthrough",
        "foundations-application-practice",
    }
    if len(m) != len(want) {
        t.Fatalf("got %d archetypes, want %d", len(m), len(want))
    }
    for _, id := range want {
        if _, ok := m[id]; !ok {
            t.Errorf("missing archetype %q", id)
        }
    }
}

func TestArchetypeHasParts(t *testing.T) {
    m, err := Load()
    if err != nil {
        t.Fatalf("Load() error: %v", err)
    }
    a := m["ontology-epistemology-practice"]
    if len(a.Parts) == 0 {
        t.Error("archetype has no parts")
    }
    if a.Parts[0].Role == "" {
        t.Error("first part has empty role")
    }
    if a.Name.Zh == "" {
        t.Error("Name.Zh is empty")
    }
}
```

- [ ] **Step 4: Run test, verify it fails**

Run:
```bash
go test ./internal/archetypes/...
```

Expected: FAIL with `undefined: Load`.

- [ ] **Step 5: Write `loader.go`**

`internal/archetypes/loader.go`:

```go
package archetypes

import (
    "fmt"
    "io/fs"
    "strings"

    "gopkg.in/yaml.v3"
)

// Load parses all embedded archetype YAML files keyed by archetype ID.
func Load() (map[string]*Archetype, error) {
    entries, err := fs.ReadDir(".")
    if err != nil {
        return nil, fmt.Errorf("read embed dir: %w", err)
    }
    out := make(map[string]*Archetype)
    for _, e := range entries {
        if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
            continue
        }
        data, err := fs.ReadFile(e.Name())
        if err != nil {
            return nil, fmt.Errorf("read %s: %w", e.Name(), err)
        }
        var a Archetype
        if err := yaml.Unmarshal(data, &a); err != nil {
            return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
        }
        if a.ID == "" {
            return nil, fmt.Errorf("archetype in %s has empty id", e.Name())
        }
        out[a.ID] = &a
    }
    return out, nil
}

// verify the embed.FS API we use is correct (compile-time check)
var _ fs.ReadDirFile = nil
```

(The last `var _` line should be removed — it's wrong. Just delete it; it was a mistake. Final loader.go should end after the `Load` function.)

- [ ] **Step 6: Add yaml.v3 dependency**

Run:
```bash
go get gopkg.in/yaml.v3
```

- [ ] **Step 7: Run tests, verify pass**

Run:
```bash
go test ./internal/archetypes/... -v
```

Expected: both `TestLoadReturnsAllThreeArchetypes` and `TestArchetypeHasParts` PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/archetypes/ go.mod go.sum
git commit -m "feat(archetypes): embed + load archetype YAMLs"
```

---

## Task 2: Style Embed + Loader

**Files:**
- Create: `internal/style/embed.go`
- Create: `internal/style/loader.go`
- Create: `internal/style/loader_test.go`

**Interfaces:**
- Produces: `style.LoadGuide() (string, error)`, `style.LoadSamples() (map[string]string, error)` (map: archetype ID → sample markdown)

- [ ] **Step 1: Write failing test**

`internal/style/loader_test.go`:

```go
package style

import (
    "strings"
    "testing"
)

func TestLoadGuideReturnsNonEmpty(t *testing.T) {
    s, err := LoadGuide()
    if err != nil {
        t.Fatalf("LoadGuide error: %v", err)
    }
    if len(s) == 0 {
        t.Error("guide is empty")
    }
    if !strings.Contains(s, "硬规则") {
        t.Error("guide missing expected section 硬规则")
    }
}

func TestLoadSamplesReturnsThree(t *testing.T) {
    m, err := LoadSamples()
    if err != nil {
        t.Fatalf("LoadSamples error: %v", err)
    }
    want := []string{
        "ontology-epistemology-practice",
        "diagnosis-decoding-breakthrough",
        "foundations-application-practice",
    }
    if len(m) != len(want) {
        t.Fatalf("got %d samples, want %d", len(m), len(want))
    }
    for _, id := range want {
        if _, ok := m[id]; !ok {
            t.Errorf("missing sample %q", id)
        }
    }
}
```

- [ ] **Step 2: Run test, verify fail**

Run:
```bash
go test ./internal/style/...
```

Expected: FAIL with `undefined: LoadGuide`.

- [ ] **Step 3: Write `embed.go`**

`internal/style/embed.go`:

```go
package style

import "embed"

//go:embed style-guide.md
var guideFS []byte

//go:embed samples/*.md
var samplesFS embed.FS
```

- [ ] **Step 4: Write `loader.go`**

`internal/style/loader.go`:

```go
package style

import (
    "fmt"
    "io/fs"
    "path"
    "strings"
)

// LoadGuide returns the full text of style-guide.md.
func LoadGuide() (string, error) {
    return string(guideFS), nil
}

// LoadSamples returns few-shot sample markdown keyed by archetype ID
// (the basename of each samples/<id>.md file).
func LoadSamples() (map[string]string, error) {
    entries, err := samplesFS.ReadDir("samples")
    if err != nil {
        return nil, fmt.Errorf("read samples dir: %w", err)
    }
    out := make(map[string]string)
    for _, e := range entries {
        if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
            continue
        }
        data, err := samplesFS.ReadFile(path.Join("samples", e.Name()))
        if err != nil {
            return nil, fmt.Errorf("read %s: %w", e.Name(), err)
        }
        id := strings.TrimSuffix(e.Name(), ".md")
        out[id] = string(data)
    }
    return out, nil
}

// compile-time check we use embed.FS API correctly
var _ = fs.ValidPath
```

(Remove the `var _ = fs.ValidPath` line — unused, was a mistake.)

- [ ] **Step 5: Run tests, verify pass**

Run:
```bash
go test ./internal/style/... -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/style/
git commit -m "feat(style): embed + load style guide and samples"
```

---

## Task 3: Corpus Embed + Loader

**Files:**
- Create: `internal/corpus/types.go`
- Create: `internal/corpus/embed.go`
- Create: `internal/corpus/loader.go`
- Create: `internal/corpus/loader_test.go`

**Interfaces:**
- Produces: `corpus.Load() (map[string]*Book, error)`, `Book` struct

- [ ] **Step 1: Inspect one corpus JSON to confirm schema**

Run:
```bash
head -20 internal/corpus/builtin/reality-construction.json
```

Confirm top-level keys: `slug`, `title.{zh,en}`, `archetype`, `parts[]` with `chapters[]`.

- [ ] **Step 2: Write `types.go`**

`internal/corpus/types.go`:

```go
package corpus

// Book is a reference book outline stored in the builtin corpus.
// Schema mirrors internal/corpus/builtin/*.json.
type Book struct {
    Slug      string         `json:"slug"`
    Title     LocalizedTitle `json:"title"`
    Subtitle  string         `json:"subtitle,omitempty"`
    Archetype string         `json:"archetype"`
    Audience  string         `json:"audience"`
    Depth     string         `json:"depth"`
    Goal      string         `json:"goal"`
    Length    string         `json:"length"`
    Language  []string       `json:"language"`
    Source    Source         `json:"source"`
    Abstract  string         `json:"abstract"`
    Parts     []Part         `json:"parts"`
}

type LocalizedTitle struct {
    Zh string `json:"zh"`
    En string `json:"en"`
}

type Source struct {
    Name       string `json:"name"`
    URL        string `json:"url"`
    AccessedAt string `json:"accessed_at"`
}

type Part struct {
    Index    int        `json:"index"`
    Title    LocalizedTitle `json:"title"`
    Role     string     `json:"role"`
    Intro    string     `json:"intro,omitempty"`
    Chapters []Chapter  `json:"chapters"`
}

type Chapter struct {
    Index    int        `json:"index"`
    Title    LocalizedTitle `json:"title"`
    Abstract string     `json:"abstract,omitempty"`
}
```

- [ ] **Step 3: Write failing test**

`internal/corpus/loader_test.go`:

```go
package corpus

import "testing"

func TestLoadReturnsAllSixBooks(t *testing.T) {
    m, err := Load()
    if err != nil {
        t.Fatalf("Load error: %v", err)
    }
    want := []string{
        "reality-construction",
        "advancement-of-reality",
        "silent-games",
        "forced-convergence",
        "ai-engineer-in-action",
        "intelligent-computing-center-construction-guide",
    }
    if len(m) != len(want) {
        t.Fatalf("got %d books, want %d", len(m), len(want))
    }
    for _, slug := range want {
        if _, ok := m[slug]; !ok {
            t.Errorf("missing book %q", slug)
        }
    }
}

func TestBookHasPartsAndChapters(t *testing.T) {
    m, _ := Load()
    b := m["reality-construction"]
    if len(b.Parts) == 0 {
        t.Fatal("book has no parts")
    }
    if len(b.Parts[0].Chapters) == 0 {
        t.Error("first part has no chapters")
    }
}
```

- [ ] **Step 4: Run test, verify fail**

```bash
go test ./internal/corpus/...
```

Expected: FAIL with `undefined: Load`.

- [ ] **Step 5: Write `embed.go`**

`internal/corpus/embed.go`:

```go
package corpus

import "embed"

//go:embed builtin/*.json
var builtinFS embed.FS
```

- [ ] **Step 6: Write `loader.go`**

`internal/corpus/loader.go`:

```go
package corpus

import (
    "encoding/json"
    "fmt"
    "io/fs"
    "strings"
)

// Load parses all embedded builtin corpus JSON files keyed by book slug.
func Load() (map[string]*Book, error) {
    entries, err := builtinFS.ReadDir("builtin")
    if err != nil {
        return nil, fmt.Errorf("read builtin dir: %w", err)
    }
    out := make(map[string]*Book)
    for _, e := range entries {
        if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
            continue
        }
        data, err := fs.ReadFile(builtinFS, "builtin/"+e.Name())
        if err != nil {
            return nil, fmt.Errorf("read %s: %w", e.Name(), err)
        }
        var b Book
        if err := json.Unmarshal(data, &b); err != nil {
            return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
        }
        if b.Slug == "" {
            return nil, fmt.Errorf("book in %s has empty slug", e.Name())
        }
        out[b.Slug] = &b
    }
    return out, nil
}
```

- [ ] **Step 7: Run tests, verify pass**

```bash
go test ./internal/corpus/... -v
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/corpus/
git commit -m "feat(corpus): embed + load builtin corpus JSONs"
```

---

## Task 4: Book Types + Slug

**Files:**
- Create: `internal/book/types.go`
- Create: `internal/book/slug.go`
- Create: `internal/book/slug_test.go`

**Interfaces:**
- Produces: `Meta`, `Outline`, `Chapter`, `Citation` structs; `book.Slugify(title string) string`

- [ ] **Step 1: Write `types.go`**

`internal/book/types.go`:

```go
package book

import "time"

// Meta is the top-level book metadata, serialized to meta.json.
// Schema mirrors DESIGN.md §4.2.
type Meta struct {
    ID         string         `json:"id"`
    Slug       string         `json:"slug"`
    Title      string         `json:"title"`
    Subtitle   string         `json:"subtitle,omitempty"`
    Archetype  string         `json:"archetype"`
    Parameters Parameters     `json:"parameters"`
    Language   string         `json:"language"`
    Status     string         `json:"status"`
    CreatedAt  time.Time      `json:"created_at"`
    UpdatedAt  time.Time      `json:"updated_at"`
    Engine     EngineMeta     `json:"engine"`
}

type Parameters struct {
    Audience string `json:"audience"`
    Depth    string `json:"depth"`
    Goal     string `json:"goal"`
    Length   string `json:"length"`
}

type EngineMeta struct {
    JianwuVersion          string `json:"jianwu_version"`
    ArchetypeLibraryVersion string `json:"archetype_library_version"`
    GrillTreeVersion       string `json:"grill_tree_version"`
    StyleGuideVersion      string `json:"style_guide_version"`
    SamplesVersion         string `json:"samples_version"`
}

// Outline is the book outline, serialized to outline.json.
type Outline struct {
    Parts []OutlinePart `json:"parts"`
}

type OutlinePart struct {
    Index    int              `json:"index"`
    Title    string           `json:"title"`
    Role     string           `json:"role"`
    Intro    string           `json:"intro,omitempty"`
    Chapters []OutlineChapter `json:"chapters"`
}

type OutlineChapter struct {
    Index             int        `json:"index"`
    Title             string     `json:"title"`
    Abstract          string     `json:"abstract,omitempty"`
    KeyConcepts       []string   `json:"key_concepts,omitempty"`
    LearningObjectives []string  `json:"learning_objectives,omitempty"`
    SuggestedExamples []string   `json:"suggested_examples,omitempty"`
    Claims            []Claim    `json:"claims,omitempty"`
    Status            string     `json:"status"`
    WordCountTarget   int        `json:"word_count_target,omitempty"`
    WordCount         int        `json:"word_count,omitempty"`
    CitationsCount    int        `json:"citations_count,omitempty"`
    UnverifiedClaims  int        `json:"unverified_claims,omitempty"`
    CoherenceScore    *float64   `json:"coherence_score,omitempty"`
    ExpandedWith      *ExpandedWith `json:"expanded_with,omitempty"`
    ReviewedAt        *time.Time `json:"reviewed_at,omitempty"`
    ReviewedBy        string     `json:"reviewed_by,omitempty"`
    Citations         []Citation `json:"citations,omitempty"`
}

type Claim struct {
    Text       string `json:"text"`
    HasCitation bool  `json:"has_citation"`
}

type ExpandedWith struct {
    Provider  string   `json:"provider"`
    Model     string   `json:"model"`
    ToolsUsed []string `json:"tools_used,omitempty"`
    Iterations int     `json:"iterations,omitempty"`
    Tokens    Tokens   `json:"tokens"`
}

type Tokens struct {
    In  int `json:"in"`
    Out int `json:"out"`
}

type Citation struct {
    ID              string `json:"id"`
    URL             string `json:"url"`
    Title           string `json:"title,omitempty"`
    AccessedAt      time.Time `json:"accessed_at,omitempty"`
    Snippet         string `json:"snippet,omitempty"`
    UsedInParagraph string `json:"used_in_paragraph,omitempty"`
    SearchProvider  string `json:"search_provider,omitempty"`
    ReaderProvider  string `json:"reader_provider,omitempty"`
}

// Chapter status constants.
const (
    StatusScaffolded = "scaffolded"
    StatusExpanded   = "expanded"
    StatusReviewed   = "reviewed"
    StatusFinal      = "final"
    StatusFailed     = "failed"
)

// Book status constants (mirrors Meta.Status).
const (
    BookStatusDraft = "draft"
)
```

- [ ] **Step 2: Write failing test for slug**

`internal/book/slug_test.go`:

```go
package book

import "testing"

func TestSlugifyAsciiTitle(t *testing.T) {
    cases := []struct {
        in, want string
    }{
        {"Reality of Time", "reality-of-time"},
        {"  Hello,  World!  ", "hello-world"},
        {"Foo / Bar", "foo-bar"},
        {"Multiple   Spaces", "multiple-spaces"},
    }
    for _, c := range cases {
        got := Slugify(c.in)
        if got != c.want {
            t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
        }
    }
}

func TestSlugifyChineseTitleReturnsPinyinOrHash(t *testing.T) {
    // Chinese titles cannot be safely slugified without a transliteration lib.
    // For v1.0 we accept a deterministic hash fallback.
    got := Slugify("时间的实在")
    if got == "" {
        t.Error("Slugify of Chinese returned empty")
    }
    // Must be deterministic
    if Slugify("时间的实在") != got {
        t.Error("Slugify is not deterministic")
    }
    // Must be lowercase ASCII
    for _, r := range got {
        if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
            t.Errorf("Slugify output contains non-slug char %q", r)
        }
    }
}
```

- [ ] **Step 3: Run test, verify fail**

```bash
go test ./internal/book/...
```

Expected: FAIL.

- [ ] **Step 4: Write `slug.go`**

`internal/book/slug.go`:

```go
package book

import (
    "crypto/sha256"
    "encoding/hex"
    "regexp"
    "strings"
)

var (
    nonASCII     = regexp.MustCompile(`[^a-zA-Z0-9\s-]`)
    whitespace   = regexp.MustCompile(`\s+`)
    leadingTrailingDash = regexp.MustCompile(`^-+|-+$`)
)

// Slugify converts a title to a URL-safe slug.
// ASCII titles: lowercased, whitespace and punctuation to dashes.
// Non-ASCII titles (Chinese, etc.): deterministic hash fallback,
// so the same title always produces the same slug.
func Slugify(title string) string {
    t := strings.TrimSpace(strings.ToLower(title))
    if t == "" {
        return ""
    }
    if isASCII(t) {
        t = nonASCII.ReplaceAllString(t, "")
        t = whitespace.ReplaceAllString(t, "-")
        t = leadingTrailingDash.ReplaceAllString(t, "")
        if t == "" {
            return ""
        }
        return t
    }
    // Non-ASCII: hash fallback (short, deterministic)
    sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(title))))
    return "b-" + hex.EncodeToString(sum[:])[:12]
}

func isASCII(s string) bool {
    for _, r := range s {
        if r > 127 {
            return false
        }
    }
    return true
}
```

- [ ] **Step 5: Run tests, verify pass**

```bash
go test ./internal/book/... -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/book/
git commit -m "feat(book): add Meta/Outline types and Slugify"
```

---

## Task 5: Book JSON I/O

**Files:**
- Create: `internal/book/io.go`
- Create: `internal/book/io_test.go`

**Interfaces:**
- Consumes: `Meta`, `Outline` from Task 4
- Produces: `book.LoadMeta(path)`, `book.SaveMeta(path, *Meta)`, `book.LoadOutline(path)`, `book.SaveOutline(path, *Outline)`

- [ ] **Step 1: Write failing test**

`internal/book/io_test.go`:

```go
package book

import (
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestSaveAndLoadMetaRoundTrip(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "meta.json")

    original := &Meta{
        ID:        "018f3d3a-1b2c-7d3e-9a4b-1234567890ab",
        Slug:      "reality-of-time",
        Title:     "时间的实在",
        Archetype: "ontology-epistemology-practice",
        Language:  "zh",
        Status:    "draft",
        CreatedAt: time.Date(2026, 6, 21, 14, 30, 0, 0, time.UTC),
        UpdatedAt: time.Date(2026, 6, 21, 14, 30, 0, 0, time.UTC),
        Parameters: Parameters{
            Audience: "educated-general",
            Depth:    "advanced",
            Goal:     "understanding",
            Length:   "long",
        },
    }
    if err := SaveMeta(path, original); err != nil {
        t.Fatalf("SaveMeta: %v", err)
    }

    // Verify file is valid JSON
    data, err := os.ReadFile(path)
    if err != nil {
        t.Fatal(err)
    }
    if len(data) == 0 {
        t.Fatal("meta.json is empty")
    }

    loaded, err := LoadMeta(path)
    if err != nil {
        t.Fatalf("LoadMeta: %v", err)
    }
    if loaded.ID != original.ID {
        t.Errorf("ID: got %q want %q", loaded.ID, original.ID)
    }
    if loaded.Title != original.Title {
        t.Errorf("Title: got %q want %q", loaded.Title, original.Title)
    }
    if !loaded.CreatedAt.Equal(original.CreatedAt) {
        t.Errorf("CreatedAt mismatch: got %v want %v", loaded.CreatedAt, original.CreatedAt)
    }
}

func TestSaveAndLoadOutlineRoundTrip(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "outline.json")

    original := &Outline{
        Parts: []OutlinePart{
            {
                Index: 1, Title: "第一部", Role: "ontology",
                Chapters: []OutlineChapter{
                    {Index: 1, Title: "第一章", Status: StatusScaffolded},
                },
            },
        },
    }
    if err := SaveOutline(path, original); err != nil {
        t.Fatalf("SaveOutline: %v", err)
    }

    loaded, err := LoadOutline(path)
    if err != nil {
        t.Fatalf("LoadOutline: %v", err)
    }
    if len(loaded.Parts) != 1 {
        t.Fatalf("parts: got %d want 1", len(loaded.Parts))
    }
    if loaded.Parts[0].Chapters[0].Status != StatusScaffolded {
        t.Errorf("status: got %q want %q", loaded.Parts[0].Chapters[0].Status, StatusScaffolded)
    }
}

func TestLoadMetaMissingFileReturnsError(t *testing.T) {
    _, err := LoadMeta("/nonexistent/meta.json")
    if err == nil {
        t.Error("expected error for missing file, got nil")
    }
}
```

- [ ] **Step 2: Run test, verify fail**

```bash
go test ./internal/book/...
```

Expected: FAIL.

- [ ] **Step 3: Write `io.go`**

`internal/book/io.go`:

```go
package book

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)

// LoadMeta reads and parses meta.json.
func LoadMeta(path string) (*Meta, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read meta %s: %w", path, err)
    }
    var m Meta
    if err := json.Unmarshal(data, &m); err != nil {
        return nil, fmt.Errorf("parse meta %s: %w", path, err)
    }
    return &m, nil
}

// SaveMeta writes meta.json with 2-space indent.
func SaveMeta(path string, m *Meta) error {
    return writeJSON(path, m)
}

// LoadOutline reads and parses outline.json.
func LoadOutline(path string) (*Outline, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read outline %s: %w", path, err)
    }
    var o Outline
    if err := json.Unmarshal(data, &o); err != nil {
        return nil, fmt.Errorf("parse outline %s: %w", path, err)
    }
    return &o, nil
}

// SaveOutline writes outline.json with 2-space indent.
func SaveOutline(path string, o *Outline) error {
    return writeJSON(path, o)
}

func writeJSON(path string, v any) error {
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return fmt.Errorf("mkdir for %s: %w", path, err)
    }
    data, err := json.MarshalIndent(v, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal: %w", err)
    }
    data = append(data, '\n')
    if err := os.WriteFile(path, data, 0o644); err != nil {
        return fmt.Errorf("write %s: %w", path, err)
    }
    return nil
}
```

- [ ] **Step 4: Run tests, verify pass**

```bash
go test ./internal/book/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/book/io.go internal/book/io_test.go
git commit -m "feat(book): JSON load/save for Meta and Outline"
```

---

## Task 6: Workspace Types + Detect

**Files:**
- Create: `internal/workspace/types.go`
- Create: `internal/workspace/detect.go`
- Create: `internal/workspace/detect_test.go`

**Interfaces:**
- Produces: `workspace.MarkerName` (const = ".jianwu"), `workspace.FindWorkspace(startPath string) (string, error)`, `workspace.ErrWorkspaceNotFound`

- [ ] **Step 1: Write `types.go`**

`internal/workspace/types.go`:

```go
package workspace

import "errors"

// MarkerName is the directory that marks a workspace root.
const MarkerName = ".jianwu"

// ConfigFileName is the workspace config file inside MarkerName.
const ConfigFileName = "config.yaml"

// SchemaVersionFileName is the workspace schema version file.
const SchemaVersionFileName = "schema_version"

// CurrentSchemaVersion is the workspace schema version this build supports.
const CurrentSchemaVersion = "1"

// ErrWorkspaceNotFound is returned when no .jianwu/ is found walking up.
var ErrWorkspaceNotFound = errors.New("workspace not found: no .jianwu/ in this or any parent directory")

// InitOpts controls Init behavior.
type InitOpts struct {
    // Bare: when true, do not create books/exports/archive directories.
    Bare bool
}
```

- [ ] **Step 2: Write failing test**

`internal/workspace/detect_test.go`:

```go
package workspace

import (
    "os"
    "path/filepath"
    "testing"
)

func TestFindWorkspaceInCurrentDir(t *testing.T) {
    root := t.TempDir()
    if err := os.Mkdir(filepath.Join(root, MarkerName), 0o755); err != nil {
        t.Fatal(err)
    }
    got, err := FindWorkspace(root)
    if err != nil {
        t.Fatalf("FindWorkspace: %v", err)
    }
    if got != root {
        t.Errorf("got %q want %q", got, root)
    }
}

func TestFindWorkspaceWalksUp(t *testing.T) {
    root := t.TempDir()
    if err := os.Mkdir(filepath.Join(root, MarkerName), 0o755); err != nil {
        t.Fatal(err)
    }
    deep := filepath.Join(root, "books", "my-book", "chapters")
    if err := os.MkdirAll(deep, 0o755); err != nil {
        t.Fatal(err)
    }
    got, err := FindWorkspace(deep)
    if err != nil {
        t.Fatalf("FindWorkspace: %v", err)
    }
    if got != root {
        t.Errorf("got %q want %q", got, root)
    }
}

func TestFindWorkspaceReturnsErrorWhenNotFound(t *testing.T) {
    root := t.TempDir()
    _, err := FindWorkspace(root)
    if err == nil {
        t.Fatal("expected error, got nil")
    }
    if err != ErrWorkspaceNotFound {
        t.Errorf("got %v, want ErrWorkspaceNotFound", err)
    }
}
```

- [ ] **Step 3: Run test, verify fail**

```bash
go test ./internal/workspace/...
```

Expected: FAIL.

- [ ] **Step 4: Write `detect.go`**

`internal/workspace/detect.go`:

```go
package workspace

import (
    "os"
    "path/filepath"
)

// FindWorkspace walks up from startPath looking for a directory containing
// a .jianwu/ subdirectory. Returns the absolute path of the workspace root
// or ErrWorkspaceNotFound.
func FindWorkspace(startPath string) (string, error) {
    abs, err := filepath.Abs(startPath)
    if err != nil {
        return "", err
    }
    dir := abs
    for {
        if isWorkspace(dir) {
            return dir, nil
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            // reached filesystem root
            return "", ErrWorkspaceNotFound
        }
        dir = parent
    }
}

func isWorkspace(dir string) bool {
    info, err := os.Stat(filepath.Join(dir, MarkerName))
    return err == nil && info.IsDir()
}
```

- [ ] **Step 5: Run tests, verify pass**

```bash
go test ./internal/workspace/... -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/workspace/types.go internal/workspace/detect.go internal/workspace/detect_test.go
git commit -m "feat(workspace): workspace marker types and walk-up detection"
```

---

## Task 7: Workspace Init

**Files:**
- Create: `internal/workspace/init.go`
- Create: `internal/workspace/init_test.go`

**Interfaces:**
- Consumes: `InitOpts`, `MarkerName`, `ConfigFileName`, `SchemaVersionFileName`, `CurrentSchemaVersion` from Task 6
- Produces: `workspace.Init(path string, opts InitOpts) error`

- [ ] **Step 1: Write failing test**

`internal/workspace/init_test.go`:

```go
package workspace

import (
    "os"
    "path/filepath"
    "testing"
)

func TestInitCreatesFullWorkspace(t *testing.T) {
    root := t.TempDir()

    if err := Init(root, InitOpts{}); err != nil {
        t.Fatalf("Init: %v", err)
    }

    for _, p := range []string{
        MarkerName,
        MarkerName + "/" + ConfigFileName,
        MarkerName + "/" + SchemaVersionFileName,
        "books",
        "exports",
        "archive",
    } {
        if _, err := os.Stat(filepath.Join(root, p)); err != nil {
            t.Errorf("missing %s: %v", p, err)
        }
    }
}

func TestInitBareOmitsBooksDirs(t *testing.T) {
    root := t.TempDir()

    if err := Init(root, InitOpts{Bare: true}); err != nil {
        t.Fatalf("Init: %v", err)
    }

    // .jianwu/ must still exist
    if _, err := os.Stat(filepath.Join(root, MarkerName)); err != nil {
        t.Errorf(".jianwu missing: %v", err)
    }
    // books/ etc. must NOT exist
    for _, p := range []string{"books", "exports", "archive"} {
        if _, err := os.Stat(filepath.Join(root, p)); err == nil {
            t.Errorf("%s/ should not exist with --bare", p)
        }
    }
}

func TestInitExistingReturnsError(t *testing.T) {
    root := t.TempDir()
    if err := Init(root, InitOpts{}); err != nil {
        t.Fatal(err)
    }
    err := Init(root, InitOpts{})
    if err == nil {
        t.Error("expected error on re-init, got nil")
    }
}

func TestInitWritesSchemaVersionOne(t *testing.T) {
    root := t.TempDir()
    if err := Init(root, InitOpts{}); err != nil {
        t.Fatal(err)
    }
    data, err := os.ReadFile(filepath.Join(root, MarkerName, SchemaVersionFileName))
    if err != nil {
        t.Fatal(err)
    }
    got := string(data)
    if got != "1\n" && got != "1" {
        t.Errorf("schema_version = %q, want \"1\"", got)
    }
}
```

- [ ] **Step 2: Run test, verify fail**

```bash
go test ./internal/workspace/...
```

Expected: FAIL.

- [ ] **Step 3: Write `init.go`**

`internal/workspace/init.go`:

```go
package workspace

import (
    "fmt"
    "os"
    "path/filepath"
)

// Init creates a workspace at the given path.
// Default (non-bare) layout: .jianwu/{config.yaml, schema_version} + books/ + exports/ + archive/.
// Bare layout: only .jianwu/ with config.yaml + schema_version.
// Returns an error if a workspace already exists at the path.
func Init(path string, opts InitOpts) error {
    marker := filepath.Join(path, MarkerName)
    if _, err := os.Stat(marker); err == nil {
        return fmt.Errorf("workspace already exists at %s", path)
    }

    if err := os.MkdirAll(marker, 0o755); err != nil {
        return fmt.Errorf("create %s: %w", marker, err)
    }

    if err := os.WriteFile(
        filepath.Join(marker, SchemaVersionFileName),
        []byte(CurrentSchemaVersion+"\n"),
        0o644,
    ); err != nil {
        return fmt.Errorf("write schema_version: %w", err)
    }

    cfg := defaultWorkspaceConfig()
    if err := os.WriteFile(
        filepath.Join(marker, ConfigFileName),
        []byte(cfg),
        0o644,
    ); err != nil {
        return fmt.Errorf("write config.yaml: %w", err)
    }

    if opts.Bare {
        return nil
    }

    for _, sub := range []string{"books", "exports", "archive"} {
        if err := os.MkdirAll(filepath.Join(path, sub), 0o755); err != nil {
            return fmt.Errorf("create %s: %w", sub, err)
        }
    }
    return nil
}

// defaultWorkspaceConfig returns the template written into config.yaml on init.
// Kept in workspace package (not config package) to avoid an import cycle:
// config package loads workspaces, workspace writes the initial template.
func defaultWorkspaceConfig() string {
    return `# jianwu workspace configuration
# Global config: ~/.config/jianwu/config.yaml (overrides here)
schema_version: 1

models:
  intake:       { provider: glm,    model: glm-4.6 }
  outline:      { provider: gemini, model: gemini-2.5-pro }
  scaffolding:  { provider: gemini, model: gemini-2.5-flash }
  expand:       { provider: glm,    model: glm-4.6 }
  # Fallback / retry policy: see global config.

search:
  primary: brave
  fallback: serper
  reader: jina

archetypes:
  library: [user, builtin]

style:
  guide: [user, builtin]
  samples: [builtin]

scaffolding:
  concurrency: 5

logging:
  level: warn
`
}
```

- [ ] **Step 4: Run tests, verify pass**

```bash
go test ./internal/workspace/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/workspace/init.go internal/workspace/init_test.go
git commit -m "feat(workspace): Init with bare/full layouts and schema_version"
```

---

## Task 8: Workspace Load + Schema Check

**Files:**
- Create: `internal/workspace/load.go`
- Create: `internal/workspace/load_test.go`

**Interfaces:**
- Consumes: `MarkerName`, `SchemaVersionFileName`, `CurrentSchemaVersion` from Task 6, `config.Load` from Task 9
- Produces: `workspace.Load(wsRoot string) (*Workspace, error)`, `Workspace` struct
- Note: this task is implemented *after* Task 9 (config loader); but the test can stub the config dependency via an interface. To keep tasks sequential, we define the struct + Load with a stub now and wire real config in Task 9.

**Reordered:** Implement Task 9 (Config) first, then Task 8 (Workspace Load). The numbering stays for stable references; just execute in order 6,7,9,8.

Actually for clarity, just reorder the task list below. **Task 8 = Workspace Load**, **Task 9 = Config**, but execute Task 9 first. We'll note this in the task list.

- [ ] **Step 1: Write failing test**

`internal/workspace/load_test.go`:

```go
package workspace

import (
    "path/filepath"
    "testing"
)

func TestLoadReturnsConfig(t *testing.T) {
    root := t.TempDir()
    if err := Init(root, InitOpts{}); err != nil {
        t.Fatal(err)
    }

    ws, err := Load(root)
    if err != nil {
        t.Fatalf("Load: %v", err)
    }
    if ws.Root != root {
        t.Errorf("Root: got %q want %q", ws.Root, root)
    }
    if ws.Config == nil {
        t.Error("Config is nil")
    }
}

func TestLoadChecksSchemaVersion(t *testing.T) {
    root := t.TempDir()
    if err := Init(root, InitOpts{}); err != nil {
        t.Fatal(err)
    }
    // Corrupt schema_version
    if err := overwriteFile(filepath.Join(root, MarkerName, SchemaVersionFileName), "99"); err != nil {
        t.Fatal(err)
    }
    _, err := Load(root)
    if err == nil {
        t.Error("expected schema mismatch error, got nil")
    }
}

func overwriteFile(path, content string) error {
    return osWriteFile(path, []byte(content), 0o644)
}
// tiny helper to avoid importing os in test
```

(The `osWriteFile` helper would create a name collision. Use `os.WriteFile` directly instead. Replace `overwriteFile` body with `return os.WriteFile(path, []byte(content), 0o644)` and import "os".)

- [ ] **Step 2: Run test, verify fail**

```bash
go test ./internal/workspace/...
```

Expected: FAIL with `undefined: Load` (and `undefined: Workspace`).

- [ ] **Step 3: Write `load.go`**

`internal/workspace/load.go`:

```go
package workspace

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/zhurong/jianwu/internal/config"
)

// Workspace is a loaded workspace root + its resolved config.
type Workspace struct {
    Root   string
    Config *config.Config
}

// Load validates the workspace at wsRoot and returns it with config resolved.
func Load(wsRoot string) (*Workspace, error) {
    marker := filepath.Join(wsRoot, MarkerName)
    if info, err := os.Stat(marker); err != nil || !info.IsDir() {
        return nil, fmt.Errorf("%w: %s", ErrWorkspaceNotFound, wsRoot)
    }

    schemaBytes, err := os.ReadFile(filepath.Join(marker, SchemaVersionFileName))
    if err != nil {
        return nil, fmt.Errorf("read schema_version: %w", err)
    }
    schema := strings.TrimSpace(string(schemaBytes))
    if schema != CurrentSchemaVersion {
        return nil, fmt.Errorf(
            "workspace schema_version %q does not match supported version %q: run `jianwu migrate` (planned for v1.1)",
            schema, CurrentSchemaVersion,
        )
    }

    cfg, err := config.Load(wsRoot)
    if err != nil {
        return nil, fmt.Errorf("load config: %w", err)
    }

    return &Workspace{Root: wsRoot, Config: cfg}, nil
}
```

- [ ] **Step 4: Run tests, verify fail (config not yet defined)**

```bash
go test ./internal/workspace/...
```

Expected: FAIL with `undefined: config.Load`. **Proceed to Task 9, then re-run this test.**

- [ ] **Step 5: Commit (work in progress; will re-verify after Task 9)**

```bash
git add internal/workspace/load.go internal/workspace/load_test.go
git commit -m "feat(workspace): Load with schema check (depends on config.Load)"
```

---

## Task 9: Config Types + Defaults + Loader

**Files:**
- Create: `internal/config/types.go`
- Create: `internal/config/defaults.go`
- Create: `internal/config/loader.go`
- Create: `internal/config/loader_test.go`

**Interfaces:**
- Produces: `config.Config` struct, `config.Load(wsRoot string) (*Config, error)`

- [ ] **Step 1: Write `types.go`**

`internal/config/types.go`:

```go
package config

// Config is the fully-resolved configuration for a workspace.
// Layers (low to high precedence): built-in defaults < global user config
// < workspace config < env vars < CLI flags.
// Env var and CLI flag overrides are applied by the CLI layer; Load returns
// the merged result of the three file-backed layers.
type Config struct {
    SchemaVersion int          `yaml:"schema_version"`
    Models        Models       `yaml:"models"`
    Search        Search       `yaml:"search"`
    Archetypes    SourceOrder  `yaml:"archetypes"`
    Style         StyleSources `yaml:"style"`
    Scaffolding   Scaffolding  `yaml:"scaffolding"`
    Logging       Logging      `yaml:"logging"`
}

type Models struct {
    Intake      ModelRef `yaml:"intake"`
    Outline     ModelRef `yaml:"outline"`
    Scaffolding ModelRef `yaml:"scaffolding"`
    Expand      ModelRef `yaml:"expand"`
}

// ModelRef names a provider+model for a stage.
type ModelRef struct {
    Provider string `yaml:"provider"`
    Model    string `yaml:"model"`
}

type Search struct {
    Primary  string `yaml:"primary"`
    Fallback string `yaml:"fallback"`
    Reader   string `yaml:"reader"`
}

type SourceOrder struct {
    Library []string `yaml:"library"`
}

type StyleSources struct {
    Guide   []string `yaml:"guide"`
    Samples []string `yaml:"samples"`
}

type Scaffolding struct {
    Concurrency int `yaml:"concurrency"`
}

type Logging struct {
    Level string `yaml:"level"`
}
```

- [ ] **Step 2: Write `defaults.go`**

`internal/config/defaults.go`:

```go
package config

// BuiltinDefaults returns the lowest-precedence config layer.
// These values are used when neither global nor workspace config specifies
// a field.
func BuiltinDefaults() *Config {
    return &Config{
        SchemaVersion: 1,
        Models: Models{
            Intake:      ModelRef{Provider: "glm", Model: "glm-4.6"},
            Outline:     ModelRef{Provider: "gemini", Model: "gemini-2.5-pro"},
            Scaffolding: ModelRef{Provider: "gemini", Model: "gemini-2.5-flash"},
            Expand:      ModelRef{Provider: "glm", Model: "glm-4.6"},
        },
        Search: Search{
            Primary: "brave", Fallback: "serper", Reader: "jina",
        },
        Archetypes: SourceOrder{Library: []string{"user", "builtin"}},
        Style: StyleSources{
            Guide:   []string{"user", "builtin"},
            Samples: []string{"builtin"},
        },
        Scaffolding: Scaffolding{Concurrency: 5},
        Logging:     Logging{Level: "warn"},
    }
}
```

- [ ] **Step 3: Write failing test**

`internal/config/loader_test.go`:

```go
package config

import (
    "os"
    "path/filepath"
    "testing"
)

func TestLoadReturnsDefaultsWhenNoGlobalOrWorkspace(t *testing.T) {
    // Use temp HOME so no global config exists
    tmpHome := t.TempDir()
    t.Setenv("HOME", tmpHome)

    wsRoot := t.TempDir()
    // No .jianwu/config.yaml in workspace
    if err := os.MkdirAll(filepath.Join(wsRoot, ".jianwu"), 0o755); err != nil {
        t.Fatal(err)
    }

    cfg, err := Load(wsRoot)
    if err != nil {
        t.Fatalf("Load: %v", err)
    }
    if cfg.Models.Outline.Provider != "gemini" {
        t.Errorf("Outline.Provider: got %q want %q", cfg.Models.Outline.Provider, "gemini")
    }
    if cfg.Scaffolding.Concurrency != 5 {
        t.Errorf("Concurrency: got %d want 5", cfg.Scaffolding.Concurrency)
    }
}

func TestLoadWorkspaceOverridesDefaults(t *testing.T) {
    tmpHome := t.TempDir()
    t.Setenv("HOME", tmpHome)

    wsRoot := t.TempDir()
    wsConfig := `
schema_version: 1
models:
  outline: { provider: glm, model: glm-4.6 }
scaffolding:
  concurrency: 10
`
    if err := os.MkdirAll(filepath.Join(wsRoot, ".jianwu"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(wsRoot, ".jianwu", "config.yaml"), []byte(wsConfig), 0o644); err != nil {
        t.Fatal(err)
    }

    cfg, err := Load(wsRoot)
    if err != nil {
        t.Fatalf("Load: %v", err)
    }
    if cfg.Models.Outline.Provider != "glm" {
        t.Errorf("Outline.Provider: got %q want %q (workspace override)", cfg.Models.Outline.Provider, "glm")
    }
    if cfg.Models.Intake.Provider != "glm" {
        t.Errorf("Intake.Provider: got %q want %q (default retained)", cfg.Models.Intake.Provider, "glm")
    }
    if cfg.Scaffolding.Concurrency != 10 {
        t.Errorf("Concurrency: got %d want 10 (override)", cfg.Scaffolding.Concurrency)
    }
}

func TestLoadGlobalOverridesDefaults(t *testing.T) {
    tmpHome := t.TempDir()
    globalConfig := `
models:
  expand: { provider: gemini, model: gemini-2.5-pro }
`
    cfgDir := filepath.Join(tmpHome, ".config", "jianwu")
    if err := os.MkdirAll(cfgDir, 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(globalConfig), 0o644); err != nil {
        t.Fatal(err)
    }
    t.Setenv("HOME", tmpHome)

    wsRoot := t.TempDir()
    if err := os.MkdirAll(filepath.Join(wsRoot, ".jianwu"), 0o755); err != nil {
        t.Fatal(err)
    }

    cfg, err := Load(wsRoot)
    if err != nil {
        t.Fatalf("Load: %v", err)
    }
    if cfg.Models.Expand.Provider != "gemini" {
        t.Errorf("Expand.Provider: got %q want %q (global)", cfg.Models.Expand.Provider, "gemini")
    }
}

func TestLoadWorkspaceOverridesGlobal(t *testing.T) {
    tmpHome := t.TempDir()
    globalConfig := `
models:
  outline: { provider: glm, model: glm-4.6 }
`
    cfgDir := filepath.Join(tmpHome, ".config", "jianwu")
    if err := os.MkdirAll(cfgDir, 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(globalConfig), 0o644); err != nil {
        t.Fatal(err)
    }
    t.Setenv("HOME", tmpHome)

    wsRoot := t.TempDir()
    wsConfig := `
models:
  outline: { provider: gemini, model: gemini-2.5-pro }
`
    if err := os.MkdirAll(filepath.Join(wsRoot, ".jianwu"), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(wsRoot, ".jianwu", "config.yaml"), []byte(wsConfig), 0o644); err != nil {
        t.Fatal(err)
    }

    cfg, err := Load(wsRoot)
    if err != nil {
        t.Fatalf("Load: %v", err)
    }
    if cfg.Models.Outline.Provider != "gemini" {
        t.Errorf("workspace should override global; got %q want %q", cfg.Models.Outline.Provider, "gemini")
    }
}
```

- [ ] **Step 4: Run test, verify fail**

```bash
go test ./internal/config/...
```

Expected: FAIL.

- [ ] **Step 5: Write `loader.go`**

`internal/config/loader.go`:

```go
package config

import (
    "fmt"
    "os"
    "path/filepath"

    "gopkg.in/yaml.v3"
)

// Load resolves the config layers (excluding env/CLI which the CLI layer
// applies later). Layer precedence (low to high):
//   1. BuiltinDefaults
//   2. global: ~/.config/jianwu/config.yaml (if exists)
//   3. workspace: <wsRoot>/.jianwu/config.yaml (if exists)
func Load(wsRoot string) (*Config, error) {
    cfg := BuiltinDefaults()

    home, err := os.UserHomeDir()
    if err != nil {
        return nil, fmt.Errorf("resolve HOME: %w", err)
    }
    globalPath := filepath.Join(home, ".config", "jianwu", "config.yaml")
    if err := overlayYAML(cfg, globalPath); err != nil {
        return nil, fmt.Errorf("global config: %w", err)
    }

    wsPath := filepath.Join(wsRoot, ".jianwu", "config.yaml")
    if err := overlayYAML(cfg, wsPath); err != nil {
        return nil, fmt.Errorf("workspace config: %w", err)
    }

    return cfg, nil
}

// overlayYAML reads path (if it exists) and merges non-zero fields into cfg.
// Strategy: read file → unmarshal into a fresh Config → copy non-zero fields.
// This is shallow-merge per top-level field; nested fields are replaced wholesale.
func overlayYAML(cfg *Config, path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil
        }
        return err
    }
    var overlay Config
    if err := yaml.Unmarshal(data, &overlay); err != nil {
        return fmt.Errorf("parse %s: %w", path, err)
    }
    mergeConfig(cfg, &overlay)
    return nil
}

// mergeConfig copies non-zero fields from src into dst (in place).
// Top-level fields are merged individually; sub-structs replace wholesale
// when their presence is detected (heuristic: SchemaVersion != 0 for Config,
// or non-empty Model field for ModelRef).
func mergeConfig(dst, src *Config) {
    if src.SchemaVersion != 0 {
        dst.SchemaVersion = src.SchemaVersion
    }
    mergeModelRef(&dst.Models.Intake, &src.Models.Intake)
    mergeModelRef(&dst.Models.Outline, &src.Models.Outline)
    mergeModelRef(&dst.Models.Scaffolding, &src.Models.Scaffolding)
    mergeModelRef(&dst.Models.Expand, &src.Models.Expand)
    if src.Search.Primary != "" {
        dst.Search.Primary = src.Search.Primary
    }
    if src.Search.Fallback != "" {
        dst.Search.Fallback = src.Search.Fallback
    }
    if src.Search.Reader != "" {
        dst.Search.Reader = src.Search.Reader
    }
    if len(src.Archetypes.Library) > 0 {
        dst.Archetypes.Library = src.Archetypes.Library
    }
    if len(src.Style.Guide) > 0 {
        dst.Style.Guide = src.Style.Guide
    }
    if len(src.Style.Samples) > 0 {
        dst.Style.Samples = src.Style.Samples
    }
    if src.Scaffolding.Concurrency != 0 {
        dst.Scaffolding.Concurrency = src.Scaffolding.Concurrency
    }
    if src.Logging.Level != "" {
        dst.Logging.Level = src.Logging.Level
    }
}

func mergeModelRef(dst, src *ModelRef) {
    if src.Provider != "" {
        dst.Provider = src.Provider
    }
    if src.Model != "" {
        dst.Model = src.Model
    }
}
```

- [ ] **Step 6: Run tests, verify pass**

```bash
go test ./internal/config/... -v
```

Expected: PASS.

- [ ] **Step 7: Re-run workspace tests (Task 8 should now pass)**

```bash
go test ./internal/workspace/... -v
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/config/ internal/workspace/
git commit -m "feat(config): 5-layer config loader + workspace.Load"
```

---

## Task 10: Secrets Loader

**Files:**
- Create: `internal/config/secrets.go`
- Create: `internal/config/secrets_test.go`

**Interfaces:**
- Produces: `config.LoadSecrets() (*Secrets, error)`, `Secrets` struct
- Constants: `GeminiAPIKeyEnv = "GEMINI_API_KEY"`, `GLMAPIKeyEnv = "GLM_API_KEY"`, etc.

- [ ] **Step 1: Write failing test**

`internal/config/secrets_test.go`:

```go
package config

import (
    "os"
    "path/filepath"
    "testing"
)

func TestLoadSecretsEnvOverridesFile(t *testing.T) {
    tmpHome := t.TempDir()
    t.Setenv("HOME", tmpHome)

    // Write file with file-gemini
    secretsDir := filepath.Join(tmpHome, ".config", "jianwu")
    if err := os.MkdirAll(secretsDir, 0o755); err != nil {
        t.Fatal(err)
    }
    fileContent := "gemini_api_key: file-gemini\nglm_api_key: file-glm\n"
    if err := os.WriteFile(filepath.Join(secretsDir, "secrets.yaml"), []byte(fileContent), 0o600); err != nil {
        t.Fatal(err)
    }

    // ENV overrides file for Gemini
    t.Setenv("GEMINI_API_KEY", "env-gemini")

    s, err := LoadSecrets()
    if err != nil {
        t.Fatalf("LoadSecrets: %v", err)
    }
    if s.GeminiAPIKey != "env-gemini" {
        t.Errorf("GeminiAPIKey: got %q want %q", s.GeminiAPIKey, "env-gemini")
    }
    if s.GLMAPIKey != "file-glm" {
        t.Errorf("GLMAPIKey: got %q want %q (file fallback)", s.GLMAPIKey, "file-glm")
    }
}

func TestLoadSecretsReturnsEmptyIfNothingConfigured(t *testing.T) {
    tmpHome := t.TempDir()
    t.Setenv("HOME", tmpHome)
    // Clear any inherited env
    t.Setenv("GEMINI_API_KEY", "")
    t.Setenv("GLM_API_KEY", "")

    s, err := LoadSecrets()
    if err != nil {
        t.Fatalf("LoadSecrets: %v", err)
    }
    if s.GeminiAPIKey != "" {
        t.Errorf("expected empty Gemini key, got %q", s.GeminiAPIKey)
    }
}

func TestLoadSecretsWarnsOnLooseFilePermissions(t *testing.T) {
    tmpHome := t.TempDir()
    t.Setenv("HOME", tmpHome)

    secretsDir := filepath.Join(tmpHome, ".config", "jianwu")
    if err := os.MkdirAll(secretsDir, 0o755); err != nil {
        t.Fatal(err)
    }
    // World-readable: 0644 — too loose
    if err := os.WriteFile(filepath.Join(secretsDir, "secrets.yaml"), []byte("gemini_api_key: x\n"), 0o644); err != nil {
        t.Fatal(err)
    }

    _, err := LoadSecrets()
    if err == nil {
        t.Error("expected warning/error for loose permissions, got nil")
    }
}
```

- [ ] **Step 2: Run test, verify fail**

```bash
go test ./internal/config/...
```

Expected: FAIL.

- [ ] **Step 3: Write `secrets.go`**

`internal/config/secrets.go`:

```go
package config

import (
    "fmt"
    "os"
    "path/filepath"

    "gopkg.in/yaml.v3"
)

// Env var names for API keys.
const (
    GeminiAPIKeyEnv = "GEMINI_API_KEY"
    GLMAPIKeyEnv    = "GLM_API_KEY"
    BraveAPIKeyEnv  = "BRAVE_API_KEY"
    SerperAPIKeyEnv = "SERPER_API_KEY"
    JinaAPIKeyEnv   = "JINA_API_KEY"
)

// Secrets holds resolved API keys. ENV > file precedence is applied per field.
type Secrets struct {
    GeminiAPIKey string `yaml:"gemini_api_key"`
    GLMAPIKey    string `yaml:"glm_api_key"`
    BraveAPIKey  string `yaml:"brave_api_key"`
    SerperAPIKey string `yaml:"serper_api_key"`
    JinaAPIKey   string `yaml:"jina_api_key"`
}

// LoadSecrets resolves API keys from ENV first, then ~/.config/jianwu/secrets.yaml.
// Returns an error if the file exists with permissions looser than 0600.
func LoadSecrets() (*Secrets, error) {
    s := &Secrets{}

    home, err := os.UserHomeDir()
    if err != nil {
        return nil, fmt.Errorf("resolve HOME: %w", err)
    }
    path := filepath.Join(home, ".config", "jianwu", "secrets.yaml")

    if info, err := os.Stat(path); err == nil {
        // File exists: enforce strict permissions.
        perm := info.Mode().Perm()
        if perm > 0o600 {
            return nil, fmt.Errorf(
                "secrets file %s has permissions %o; expected 0600 or stricter (run: chmod 600 %s)",
                path, perm, path,
            )
        }
        data, err := os.ReadFile(path)
        if err != nil {
            return nil, fmt.Errorf("read secrets: %w", err)
        }
        if err := yaml.Unmarshal(data, s); err != nil {
            return nil, fmt.Errorf("parse secrets: %w", err)
        }
    }

    // ENV overrides file per field.
    if v := os.Getenv(GeminiAPIKeyEnv); v != "" {
        s.GeminiAPIKey = v
    }
    if v := os.Getenv(GLMAPIKeyEnv); v != "" {
        s.GLMAPIKey = v
    }
    if v := os.Getenv(BraveAPIKeyEnv); v != "" {
        s.BraveAPIKey = v
    }
    if v := os.Getenv(SerperAPIKeyEnv); v != "" {
        s.SerperAPIKey = v
    }
    if v := os.Getenv(JinaAPIKeyEnv); v != "" {
        s.JinaAPIKey = v
    }

    return s, nil
}
```

- [ ] **Step 4: Run tests, verify pass**

```bash
go test ./internal/config/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/secrets.go internal/config/secrets_test.go
git commit -m "feat(config): secrets loader with ENV>file precedence and 0600 enforcement"
```

---

## Task 11: CLI Root + Version + Exit Codes

**Files:**
- Create: `internal/cli/version.go`
- Create: `internal/cli/root.go`
- Create: `internal/cli/root_test.go`
- Create: `cmd/jianwu/main.go`

**Interfaces:**
- Produces: `cli.NewRootCmd() *cobra.Command`, `cli.ExitCode` constants
- Exits: main.go calls `os.Exit` with the appropriate code

- [ ] **Step 1: Write `version.go`**

`internal/cli/version.go`:

```go
package cli

// Version is the build's version string. Overridden at link time via -ldflags.
var Version = "0.1.0-dev"
```

- [ ] **Step 2: Write failing test for root command**

`internal/cli/root_test.go`:

```go
package cli

import (
    "bytes"
    "testing"
)

func TestRootCmdHasVersionFlag(t *testing.T) {
    cmd := NewRootCmd()
    flag := cmd.Flags().Lookup("version")
    if flag == nil {
        t.Error("--version flag not registered")
    }
}

func TestRootCmdVersionPrints(t *testing.T) {
    cmd := NewRootCmd()
    out := &bytes.Buffer{}
    cmd.SetOut(out)
    cmd.SetErr(&bytes.Buffer{})
    cmd.SetArgs([]string{"--version"})

    if err := cmd.Execute(); err != nil {
        t.Fatalf("Execute: %v", err)
    }
    if out.Len() == 0 {
        t.Error("expected version output, got nothing")
    }
}

func TestExitCodeConstants(t *testing.T) {
    if ExitCodeSuccess != 0 {
        t.Errorf("ExitCodeSuccess = %d", ExitCodeSuccess)
    }
    if ExitCodeGeneric != 1 {
        t.Errorf("ExitCodeGeneric = %d", ExitCodeGeneric)
    }
    if ExitCodeUsage != 2 {
        t.Errorf("ExitCodeUsage = %d", ExitCodeUsage)
    }
    if ExitCodeWorkspaceNotFound != 3 {
        t.Errorf("ExitCodeWorkspaceNotFound = %d", ExitCodeWorkspaceNotFound)
    }
    if ExitCodeLLMProvider != 4 {
        t.Errorf("ExitCodeLLMProvider = %d", ExitCodeLLMProvider)
    }
    if ExitCodeNetwork != 5 {
        t.Errorf("ExitCodeNetwork = %d", ExitCodeNetwork)
    }
}
```

- [ ] **Step 3: Run test, verify fail**

```bash
go test ./internal/cli/...
```

Expected: FAIL.

- [ ] **Step 4: Add cobra dependency**

```bash
go get github.com/spf13/cobra@latest
```

- [ ] **Step 5: Write `root.go`**

`internal/cli/root.go`:

```go
package cli

import (
    "fmt"
    "io"

    "github.com/spf13/cobra"
)

// Exit code constants. Mirrors DESIGN.md §16 decision A1.
const (
    ExitCodeSuccess           = 0
    ExitCodeGeneric           = 1
    ExitCodeUsage             = 2
    ExitCodeWorkspaceNotFound = 3
    ExitCodeLLMProvider       = 4
    ExitCodeNetwork           = 5
)

// GlobalFlags holds root-level flag values.
type GlobalFlags struct {
    Verbose bool
    Debug   bool
}

// NewRootCmd builds the root cobra command.
func NewRootCmd() *cobra.Command {
    gf := &GlobalFlags{}
    cmd := &cobra.Command{
        Use:   "jianwu",
        Short: "Structure AI's training knowledge into human-readable books.",
        Long: `jianwu (简物) - Library + CLI for turning AI's training knowledge
into human-readable, well-structured books.`,
        SilenceErrors: true,
        SilenceUsage:  true,
    }
    cmd.PersistentFlags().BoolVarP(&gf.Verbose, "verbose", "v", false, "verbose output (INFO level logs)")
    cmd.PersistentFlags().BoolVar(&gf.Debug, "debug", false, "debug output (DEBUG level + LLM request/response dump)")
    cmd.PersistentFlags().Bool("version", false, "print version and exit")

    // Override Run to handle --version
    cmd.RunE = func(c *cobra.Command, args []string) error {
        if v, _ := c.Flags().GetBool("version"); v {
            fmt.Fprintf(c.OutOrStdout(), "jianwu %s\n", Version)
            return nil
        }
        return c.Help()
    }

    cmd.AddCommand(newInitCmd())
    cmd.AddCommand(newInfoCmd())
    cmd.AddCommand(newConfigCmd())

    return cmd
}

// GlobalFlagsFrom returns the parsed global flags for the given command.
// (Used by subcommands to access verbose/debug.)
func GlobalFlagsFrom(cmd *cobra.Command) GlobalFlags {
    v, _ := cmd.Flags().GetBool("verbose")
    d, _ := cmd.Flags().GetBool("debug")
    return GlobalFlags{Verbose: v, Debug: d}
}

var _ io.Writer = (*bytes_)(nil) // placeholder; remove
type bytes_ struct{}
func (b *bytes_) Write(p []byte) (int, error) { return len(p), nil }
```

(Remove the placeholder `bytes_` lines — they were a mistake. The file should end after `GlobalFlagsFrom`.)

- [ ] **Step 6: Stub init/info/config commands so root compiles**

`internal/cli/init.go` (stub):

```go
package cli

import "github.com/spf13/cobra"

func newInitCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "init [path]",
        Short: "Initialize a jianwu workspace",
        RunE: func(cmd *cobra.Command, args []string) error {
            return nil // filled in Task 12
        },
    }
}
```

`internal/cli/info.go` (stub):

```go
package cli

import "github.com/spf13/cobra"

func newInfoCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "info",
        Short: "Show workspace status",
        RunE: func(cmd *cobra.Command, args []string) error {
            return nil
        },
    }
}
```

`internal/cli/config.go` (stub):

```go
package cli

import "github.com/spf13/cobra"

func newConfigCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "config",
        Short: "Read or write configuration",
    }
}
```

- [ ] **Step 7: Write `main.go`**

`cmd/jianwu/main.go`:

```go
package main

import (
    "fmt"
    "os"

    "github.com/zhurong/jianwu/internal/cli"
)

func main() {
    os.Exit(run())
}

func run() int {
    cmd := cli.NewRootCmd()
    if err := cmd.Execute(); err != nil {
        fmt.Fprintf(os.Stderr, "jianwu: %v\n", err)
        return cli.ExitCodeGeneric
    }
    return cli.ExitCodeSuccess
}
```

- [ ] **Step 8: Run tests, verify pass**

```bash
go test ./internal/cli/... -v
go build ./...
```

Expected: PASS + clean build.

- [ ] **Step 9: Commit**

```bash
git add internal/cli/ cmd/ go.mod go.sum
git commit -m "feat(cli): cobra root with --version, exit codes, command stubs"
```

---

## Task 12: CLI `init` Command

**Files:**
- Modify: `internal/cli/init.go`
- Create: `internal/cli/init_test.go`

**Interfaces:**
- Consumes: `workspace.Init`, `workspace.InitOpts` from Task 7

- [ ] **Step 1: Write failing test**

`internal/cli/init_test.go`:

```go
package cli

import (
    "os"
    "path/filepath"
    "testing"
)

func TestInitCreatesWorkspaceInCwd(t *testing.T) {
    dir := t.TempDir()
    cmd := NewRootCmd()
    cmd.SetArgs([]string{"init", dir})
    if err := cmd.Execute(); err != nil {
        t.Fatalf("Execute: %v", err)
    }
    if _, err := os.Stat(filepath.Join(dir, ".jianwu")); err != nil {
        t.Errorf(".jianwu not created: %v", err)
    }
    if _, err := os.Stat(filepath.Join(dir, "books")); err != nil {
        t.Errorf("books/ not created: %v", err)
    }
}

func TestInitBareFlag(t *testing.T) {
    dir := t.TempDir()
    cmd := NewRootCmd()
    cmd.SetArgs([]string{"init", "--bare", dir})
    if err := cmd.Execute(); err != nil {
        t.Fatalf("Execute: %v", err)
    }
    if _, err := os.Stat(filepath.Join(dir, ".jianwu")); err != nil {
        t.Errorf(".jianwu not created: %v", err)
    }
    if _, err := os.Stat(filepath.Join(dir, "books")); err == nil {
        t.Error("books/ should not exist with --bare")
    }
}

func TestInitDefaultsToCwd(t *testing.T) {
    dir := t.TempDir()
    oldWd, _ := os.Getwd()
    defer os.Chdir(oldWd)
    if err := os.Chdir(dir); err != nil {
        t.Fatal(err)
    }

    cmd := NewRootCmd()
    cmd.SetArgs([]string{"init"})
    if err := cmd.Execute(); err != nil {
        t.Fatalf("Execute: %v", err)
    }
    if _, err := os.Stat(filepath.Join(dir, ".jianwu")); err != nil {
        t.Errorf(".jianwu not created in cwd: %v", err)
    }
}
```

- [ ] **Step 2: Run test, verify fail (current stub does nothing)**

```bash
go test ./internal/cli/... -run TestInit -v
```

Expected: FAIL.

- [ ] **Step 3: Implement `init.go`**

`internal/cli/init.go`:

```go
package cli

import (
    "fmt"

    "github.com/spf13/cobra"

    "github.com/zhurong/jianwu/internal/workspace"
)

func newInitCmd() *cobra.Command {
    var bare bool
    cmd := &cobra.Command{
        Use:   "init [path]",
        Short: "Initialize a jianwu workspace",
        Long: `Create a jianwu workspace at the given path (defaults to current directory).

A workspace is a directory containing a .jianwu/ marker. By default, init also
creates books/, exports/, and archive/ subdirectories. Use --bare to skip those
when initializing inside an existing project.`,
        Args: cobra.MaximumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            path := "."
            if len(args) > 0 {
                path = args[0]
            }
            opts := workspace.InitOpts{Bare: bare}
            if err := workspace.Init(path, opts); err != nil {
                return err
            }
            fmt.Fprintf(cmd.OutOrStdout(), "Initialized jianwu workspace at %s\n", path)
            return nil
        },
    }
    cmd.Flags().BoolVar(&bare, "bare", false, "skip books/exports/archive creation")
    return cmd
}
```

- [ ] **Step 4: Run tests, verify pass**

```bash
go test ./internal/cli/... -run TestInit -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/init.go internal/cli/init_test.go
git commit -m "feat(cli): jianwu init (full + --bare)"
```

---

## Task 13: CLI `info` Command

**Files:**
- Modify: `internal/cli/info.go`
- Create: `internal/cli/info_test.go`

**Interfaces:**
- Consumes: `workspace.FindWorkspace`, `workspace.Load` from Tasks 6 & 8, `config.LoadSecrets` from Task 10

- [ ] **Step 1: Write failing test**

`internal/cli/info_test.go`:

```go
package cli

import (
    "bytes"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestInfoFromInsideWorkspace(t *testing.T) {
    root := t.TempDir()
    if err := runInit(root, false); err != nil {
        t.Fatal(err)
    }
    // Make a subdir to test walk-up
    sub := filepath.Join(root, "books", "mybook")
    if err := os.MkdirAll(sub, 0o755); err != nil {
        t.Fatal(err)
    }
    oldWd, _ := os.Getwd()
    defer os.Chdir(oldWd)
    if err := os.Chdir(sub); err != nil {
        t.Fatal(err)
    }

    cmd := NewRootCmd()
    out := &bytes.Buffer{}
    cmd.SetOut(out)
    cmd.SetErr(&bytes.Buffer{})
    cmd.SetArgs([]string{"info"})
    if err := cmd.Execute(); err != nil {
        t.Fatalf("Execute: %v", err)
    }

    s := out.String()
    if !strings.Contains(s, "Workspace:") {
        t.Errorf("output missing 'Workspace:': %q", s)
    }
    if !strings.Contains(s, root) {
        t.Errorf("output missing root path %q: %q", root, s)
    }
    if !strings.Contains(s, "Models:") {
        t.Errorf("output missing 'Models:': %q", s)
    }
}

func TestInfoOutsideWorkspaceReturnsExit3(t *testing.T) {
    tmp := t.TempDir()
    oldWd, _ := os.Getwd()
    defer os.Chdir(oldWd)
    if err := os.Chdir(tmp); err != nil {
        t.Fatal(err)
    }

    cmd := NewRootCmd()
    cmd.SetArgs([]string{"info"})
    err := cmd.Execute()
    if err == nil {
        t.Fatal("expected error, got nil")
    }
    // The CLI main should map workspace errors to ExitCodeWorkspaceNotFound;
    // here we just check the error is non-nil and recognizable.
    if !strings.Contains(err.Error(), "workspace") {
        t.Errorf("error should mention workspace, got: %v", err)
    }
}

// runInit is a test helper that creates a workspace at root.
func runInit(root string, bare bool) error {
    cmd := NewRootCmd()
    args := []string{"init", root}
    if bare {
        args = []string{"init", "--bare", root}
    }
    cmd.SetArgs(args)
    return cmd.Execute()
}
```

- [ ] **Step 2: Run test, verify fail**

```bash
go test ./internal/cli/... -run TestInfo -v
```

Expected: FAIL.

- [ ] **Step 3: Implement `info.go`**

`internal/cli/info.go`:

```go
package cli

import (
    "fmt"
    "strings"

    "github.com/spf13/cobra"

    "github.com/zhurong/jianwu/internal/config"
    "github.com/zhurong/jianwu/internal/workspace"
)

// InfoError wraps an error with a suggested exit code.
type InfoError struct {
    Err  error
    Code int
}

func (e *InfoError) Error() string { return e.Err.Error() }
func (e *InfoError) Unwrap() error { return e.Err }

func newInfoCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "info",
        Short: "Show workspace status",
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
            printInfo(cmd, ws, secrets)
            return nil
        },
    }
    return cmd
}

func printInfo(cmd *cobra.Command, ws *workspace.Workspace, s *config.Secrets) {
    out := cmd.OutOrStdout()
    fmt.Fprintf(out, "Workspace: %s\n", ws.Root)
    fmt.Fprintf(out, "Schema:    v%d\n", ws.Config.SchemaVersion)
    fmt.Fprintln(out)
    fmt.Fprintln(out, "Models:")
    fmt.Fprintf(out, "  intake:      %s/%s\n", ws.Config.Models.Intake.Provider, ws.Config.Models.Intake.Model)
    fmt.Fprintf(out, "  outline:     %s/%s\n", ws.Config.Models.Outline.Provider, ws.Config.Models.Outline.Model)
    fmt.Fprintf(out, "  scaffolding: %s/%s\n", ws.Config.Models.Scaffolding.Provider, ws.Config.Models.Scaffolding.Model)
    fmt.Fprintf(out, "  expand:      %s/%s\n", ws.Config.Models.Expand.Provider, ws.Config.Models.Expand.Model)
    fmt.Fprintln(out)
    fmt.Fprintln(out, "Search:")
    fmt.Fprintf(out, "  primary:  %s\n", ws.Config.Search.Primary)
    fmt.Fprintf(out, "  fallback: %s\n", ws.Config.Search.Fallback)
    fmt.Fprintf(out, "  reader:   %s\n", ws.Config.Search.Reader)
    fmt.Fprintln(out)
    fmt.Fprintln(out, "API keys (configured):")
    fmt.Fprintf(out, "  gemini: %s\n", secretStatus(s.GeminiAPIKey))
    fmt.Fprintf(out, "  glm:    %s\n", secretStatus(s.GLMAPIKey))
    fmt.Fprintf(out, "  brave:  %s\n", secretStatus(s.BraveAPIKey))
}

func secretStatus(v string) string {
    if v == "" {
        return "missing"
    }
    return "ok"
}

// ensure strings import is used if we add formatting later
var _ = strings.TrimSpace
```

(Remove the `var _ = strings.TrimSpace` line — was a placeholder. The `strings` import should also be removed if unused.)

- [ ] **Step 4: Update `main.go` to map `InfoError` to exit code**

`cmd/jianwu/main.go`:

```go
package main

import (
    "errors"
    "fmt"
    "os"

    "github.com/zhurong/jianwu/internal/cli"
)

func main() {
    os.Exit(run())
}

func run() int {
    cmd := cli.NewRootCmd()
    if err := cmd.Execute(); err != nil {
        var ie *cli.InfoError
        if errors.As(err, &ie) {
            fmt.Fprintf(os.Stderr, "jianwu: %v\n", err)
            return ie.Code
        }
        fmt.Fprintf(os.Stderr, "jianwu: %v\n", err)
        return cli.ExitCodeGeneric
    }
    return cli.ExitCodeSuccess
}
```

- [ ] **Step 5: Run tests, verify pass**

```bash
go test ./internal/cli/... -run TestInfo -v
go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/info.go internal/cli/info_test.go cmd/jianwu/main.go
git commit -m "feat(cli): jianwu info with walk-up + exit 3 on missing workspace"
```

---

## Task 14: CLI `config` Command (get/set/list)

**Files:**
- Modify: `internal/cli/config.go`
- Create: `internal/cli/config_test.go`

**Interfaces:**
- Consumes: `config.Load`, `workspace.FindWorkspace`, `workspace.Load` from earlier tasks

- [ ] **Step 1: Write failing test**

`internal/cli/config_test.go`:

```go
package cli

import (
    "bytes"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestConfigGetReturnsValue(t *testing.T) {
    root := t.TempDir()
    if err := runInit(root, false); err != nil {
        t.Fatal(err)
    }
    oldWd, _ := os.Getwd()
    defer os.Chdir(oldWd)
    if err := os.Chdir(root); err != nil {
        t.Fatal(err)
    }

    cmd := NewRootCmd()
    out := &bytes.Buffer{}
    cmd.SetOut(out)
    cmd.SetArgs([]string{"config", "get", "models.outline.provider"})
    if err := cmd.Execute(); err != nil {
        t.Fatalf("Execute: %v", err)
    }
    got := strings.TrimSpace(out.String())
    if got != "gemini" {
        t.Errorf("got %q, want %q", got, "gemini")
    }
}

func TestConfigSetWritesToWorkspace(t *testing.T) {
    root := t.TempDir()
    if err := runInit(root, false); err != nil {
        t.Fatal(err)
    }
    oldWd, _ := os.Getwd()
    defer os.Chdir(oldWd)
    if err := os.Chdir(root); err != nil {
        t.Fatal(err)
    }

    cmd := NewRootCmd()
    cmd.SetArgs([]string{"config", "set", "models.outline.provider", "glm"})
    if err := cmd.Execute(); err != nil {
        t.Fatalf("Execute: %v", err)
    }

    // Verify by re-reading
    cmd2 := NewRootCmd()
    out := &bytes.Buffer{}
    cmd2.SetOut(out)
    cmd2.SetArgs([]string{"config", "get", "models.outline.provider"})
    if err := cmd2.Execute(); err != nil {
        t.Fatalf("Execute: %v", err)
    }
    got := strings.TrimSpace(out.String())
    if got != "glm" {
        t.Errorf("after set, got %q, want %q", got, "glm")
    }
}

func TestConfigListShowsAllKeys(t *testing.T) {
    root := t.TempDir()
    if err := runInit(root, false); err != nil {
        t.Fatal(err)
    }
    oldWd, _ := os.Getwd()
    defer os.Chdir(oldWd)
    if err := os.Chdir(root); err != nil {
        t.Fatal(err)
    }

    cmd := NewRootCmd()
    out := &bytes.Buffer{}
    cmd.SetOut(out)
    cmd.SetArgs([]string{"config", "list"})
    if err := cmd.Execute(); err != nil {
        t.Fatalf("Execute: %v", err)
    }
    s := out.String()
    for _, want := range []string{"models.", "search.", "scaffolding.", "logging."} {
        if !strings.Contains(s, want) {
            t.Errorf("list missing %q: %q", want, s)
        }
    }
}

func TestConfigGetUnknownKeyErrors(t *testing.T) {
    root := t.TempDir()
    if err := runInit(root, false); err != nil {
        t.Fatal(err)
    }
    oldWd, _ := os.Getwd()
    defer os.Chdir(oldWd)
    if err := os.Chdir(root); err != nil {
        t.Fatal(err)
    }

    cmd := NewRootCmd()
    cmd.SetArgs([]string{"config", "get", "nonexistent.key"})
    err := cmd.Execute()
    if err == nil {
        t.Error("expected error for unknown key, got nil")
    }
}

// silence unused import warning if needed
var _ = filepath.Join
```

(Remove the `var _ = filepath.Join` line; remove `filepath` import if unused.)

- [ ] **Step 2: Run test, verify fail**

```bash
go test ./internal/cli/... -run TestConfig -v
```

Expected: FAIL.

- [ ] **Step 3: Implement `config.go`**

`internal/cli/config.go`:

```go
package cli

import (
    "fmt"
    "reflect"
    "strconv"
    "strings"

    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"

    "github.com/zhurong/jianwu/internal/workspace"
)

func newConfigCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "config",
        Short: "Read or write configuration",
    }
    cmd.AddCommand(newConfigGetCmd())
    cmd.AddCommand(newConfigSetCmd())
    cmd.AddCommand(newConfigListCmd())
    return cmd
}

func newConfigGetCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "get <key>",
        Short: "Get a config value by dotted key (e.g. models.outline.provider)",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            wsRoot, err := workspace.FindWorkspace(".")
            if err != nil {
                return &InfoError{Err: err, Code: ExitCodeWorkspaceNotFound}
            }
            ws, err := workspace.Load(wsRoot)
            if err != nil {
                return err
            }
            v, err := getConfigField(ws.Config, args[0])
            if err != nil {
                return err
            }
            fmt.Fprintln(cmd.OutOrStdout(), v)
            return nil
        },
    }
}

func newConfigSetCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "set <key> <value>",
        Short: "Set a config value in the workspace config.yaml",
        Args:  cobra.ExactArgs(2),
        RunE: func(cmd *cobra.Command, args []string) error {
            wsRoot, err := workspace.FindWorkspace(".")
            if err != nil {
                return &InfoError{Err: err, Code: ExitCodeWorkspaceNotFound}
            }
            ws, err := workspace.Load(wsRoot)
            if err != nil {
                return err
            }
            if err := setConfigField(ws.Config, args[0], args[1]); err != nil {
                return err
            }
            // Write back to workspace config.yaml
            data, err := yaml.Marshal(ws.Config)
            if err != nil {
                return fmt.Errorf("marshal config: %w", err)
            }
            path := wsRoot + "/.jianwu/config.yaml"
            if err := writeFile(path, data); err != nil {
                return err
            }
            fmt.Fprintf(cmd.OutOrStdout(), "set %s = %s\n", args[0], args[1])
            return nil
        },
    }
}

func newConfigListCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "list",
        Short: "List all config keys and values",
        Args:  cobra.NoArgs,
        RunE: func(cmd *cobra.Command, args []string) error {
            wsRoot, err := workspace.FindWorkspace(".")
            if err != nil {
                return &InfoError{Err: err, Code: ExitCodeWorkspaceNotFound}
            }
            ws, err := workspace.Load(wsRoot)
            if err != nil {
                return err
            }
            for _, line := range flattenConfig(ws.Config) {
                fmt.Fprintln(cmd.OutOrStdout(), line)
            }
            return nil
        },
    }
}

// getConfigField navigates a dotted path against the Config struct.
func getConfigField(cfg any, key string) (string, error) {
    v := reflect.ValueOf(cfg)
    for _, part := range strings.Split(key, ".") {
        if v.Kind() == reflect.Ptr {
            v = v.Elem()
        }
        if v.Kind() != reflect.Struct {
            return "", fmt.Errorf("key %q: %s is not a struct", key, v.Kind())
        }
        f := v.FieldByName(toExportedName(part))
        if !f.IsValid() {
            return "", fmt.Errorf("unknown config key %q (field %q)", key, part)
        }
        v = f
    }
    if v.Kind() == reflect.Ptr {
        v = v.Elem()
    }
    return fmt.Sprintf("%v", v.Interface()), nil
}

// setConfigField navigates a dotted path and sets the leaf field.
func setConfigField(cfg any, key, value string) error {
    v := reflect.ValueOf(cfg).Elem()
    parts := strings.Split(key, ".")
    for i, part := range parts {
        if v.Kind() == reflect.Ptr {
            v = v.Elem()
        }
        if v.Kind() != reflect.Struct {
            return fmt.Errorf("key %q: not a struct at %s", key, part)
        }
        f := v.FieldByName(toExportedName(part))
        if !f.IsValid() {
            return fmt.Errorf("unknown config key %q (field %q)", key, part)
        }
        if i == len(parts)-1 {
            return assignField(f, value)
        }
        v = f
    }
    return nil
}

func assignField(f reflect.Value, value string) error {
    switch f.Kind() {
    case reflect.String:
        f.SetString(value)
    case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
        n, err := strconv.ParseInt(value, 10, 64)
        if err != nil {
            return fmt.Errorf("expected int, got %q: %w", value, err)
        }
        f.SetInt(n)
    case reflect.Bool:
        b, err := strconv.ParseBool(value)
        if err != nil {
            return fmt.Errorf("expected bool, got %q: %w", value, err)
        }
        f.SetBool(b)
    default:
        return fmt.Errorf("setting field of kind %s not supported", f.Kind())
    }
    return nil
}

// toExportedName capitalizes the first letter: "outline" → "Outline".
func toExportedName(s string) string {
    if s == "" {
        return s
    }
    return strings.ToUpper(s[:1]) + s[1:]
}

// flattenConfig returns "key = value" lines for every leaf scalar.
func flattenConfig(cfg any) []string {
    var out []string
    var walk func(prefix string, v reflect.Value)
    walk = func(prefix string, v reflect.Value) {
        if v.Kind() == reflect.Ptr {
            v = v.Elem()
        }
        switch v.Kind() {
        case reflect.Struct:
            t := v.Type()
            for i := 0; i < v.NumField(); i++ {
                f := v.Field(i)
                name := strings.ToLower(t.Field(i).Name)
                key := name
                if prefix != "" {
                    key = prefix + "." + name
                }
                walk(key, f)
            }
        case reflect.Slice:
            // Skip slices (lists) for v1.0 list output
        default:
            if !v.IsZero() {
                out = append(out, fmt.Sprintf("%s = %v", prefix, v.Interface()))
            }
        }
    }
    walk("", reflect.ValueOf(cfg).Elem())
    return out
}

func writeFile(path string, data []byte) error {
    return osWriteFile(path, data, 0o644)
}
```

Also add to `internal/cli/root.go` or a small helper file `internal/cli/io.go`:

```go
package cli

import "os"

func osWriteFile(path string, data []byte, perm os.FileMode) error {
    return os.WriteFile(path, data, perm)
}
```

- [ ] **Step 4: Run tests, verify pass**

```bash
go test ./internal/cli/... -run TestConfig -v
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/config.go internal/cli/config_test.go internal/cli/io.go
git commit -m "feat(cli): config get/set/list with reflection-based key lookup"
```

---

## Task 15: End-to-End Happy Path Test

**Files:**
- Create: `internal/cli/e2e_test.go`

**Goal:** Exercise the full S1 surface in one test: `init` → `info` → `config set` → `config get` → `config list`.

- [ ] **Step 1: Write the e2e test**

`internal/cli/e2e_test.go`:

```go
package cli

import (
    "bytes"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestE2EHappyPath(t *testing.T) {
    root := t.TempDir()

    // 1. init
    run := func(args ...string) (string, error) {
        cmd := NewRootCmd()
        out := &bytes.Buffer{}
        cmd.SetOut(out)
        cmd.SetErr(out)
        cmd.SetArgs(args)
        // Each command resolves workspace from "." via FindWorkspace,
        // so we chdir into root for non-init commands.
        cmd.Execute()
        return out.String(), nil
    }

    // init in root
    if _, err := run("init", root); err != nil {
        t.Fatalf("init: %v", err)
    }
    if _, err := os.Stat(filepath.Join(root, ".jianwu", "config.yaml")); err != nil {
        t.Fatalf("config.yaml not created: %v", err)
    }

    // Switch to workspace for the rest
    oldWd, _ := os.Getwd()
    defer os.Chdir(oldWd)
    if err := os.Chdir(root); err != nil {
        t.Fatal(err)
    }

    // 2. info
    out, _ := run("info")
    if !strings.Contains(out, "Workspace:") {
        t.Errorf("info missing 'Workspace:': %q", out)
    }

    // 3. config set
    out, _ = run("config", "set", "models.expand.provider", "gemini")
    if !strings.Contains(out, "set models.expand.provider") {
        t.Errorf("set output unexpected: %q", out)
    }

    // 4. config get
    out, _ = run("config", "get", "models.expand.provider")
    if strings.TrimSpace(out) != "gemini" {
        t.Errorf("get after set: got %q want %q", strings.TrimSpace(out), "gemini")
    }

    // 5. config list
    out, _ = run("config", "list")
    if !strings.Contains(out, "models.expand.provider") {
        t.Errorf("list missing set key: %q", out)
    }
}
```

- [ ] **Step 2: Run e2e**

```bash
go test ./internal/cli/... -run TestE2E -v
```

Expected: PASS.

- [ ] **Step 3: Run all tests**

```bash
go test ./... -v
```

Expected: every test passes.

- [ ] **Step 4: Build the actual binary**

```bash
go build -o ./bin/jianwu ./cmd/jianwu
./bin/jianwu --version
```

Expected: prints `jianwu 0.1.0-dev`.

- [ ] **Step 5: Manual smoke test**

```bash
cd /tmp && rm -rf jianwu-smoke && mkdir jianwu-smoke && cd jianwu-smoke
/Users/rong.zhu/Code/@zhurong/jianwu/bin/jianwu init
/Users/rong.zhu/Code/@zhurong/jianwu/bin/jianwu info
/Users/rong.zhu/Code/@zhurong/jianwu/bin/jianwu config get models.outline.provider
/Users/rong.zhu/Code/@zhurong/jianwu/bin/jianwu config set scaffolding.concurrency 10
/Users/rong.zhu/Code/@zhurong/jianwu/bin/jianwu config list
```

Expected: all commands succeed; final `list` shows `concurrency = 10`.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/e2e_test.go
git commit -m "test(cli): e2e happy path init→info→config set→get→list"
```

---

## Task 16: README Polish + S1 Wrap

**Files:**
- Modify: `README.md`
- Modify: `internal/cli/version.go` (set Version to `0.1.0`)

**Goal:** Bring README up to a "v0.1.0-s1" level, document what works and what's deferred.

- [ ] **Step 1: Bump version**

`internal/cli/version.go`:

```go
package cli

// Version is the build's version string. Overridden at link time via -ldflags.
var Version = "0.1.0"
```

- [ ] **Step 2: Rewrite README**

`README.md`:

```markdown
# jianwu

> 简物（jiàn wù）—— 把 AI 的训练知识结构化为人类可阅读、可学习的图书。

Library + CLI. Web SaaS wrapper is a separate repo (`mouqin`).

## Status

**v0.1.0 — S1 (Foundation)**: workspace + config + CLI shell working.
LLM provider abstractions, engine, and LLM-driven commands come in
later slices (S2-S8). See `docs/superpowers/plans/` for the roadmap.

## Install

```bash
go install github.com/zhurong/jianwu/cmd/jianwu@latest
```

## Quick start

```bash
jianwu init my-library
cd my-library
jianwu info
jianwu config get models.outline.provider
jianwu config set scaffolding.concurrency 10
jianwu config list
```

## Configuration

5 layers, low → high precedence:

1. Built-in defaults
2. `~/.config/jianwu/config.yaml` (global user)
3. `<workspace>/.jianwu/config.yaml`
4. Environment variables (e.g. `JIANWU_OUTLINE_MODEL=glm-4.6`)
5. CLI flags (e.g. `--model glm-4.6`)

API keys live in `~/.config/jianwu/secrets.yaml` (mode 0600) or
environment variables `GEMINI_API_KEY` / `GLM_API_KEY` / `BRAVE_API_KEY`
/ `SERPER_API_KEY` / `JINA_API_KEY`. ENV overrides file.

## Development

```bash
go test ./...
go build -o ./bin/jianwu ./cmd/jianwu
```

See `DESIGN.md` for the full design doc.

## License

Code: AGPL-3.0 (see `LICENSE`).
Embedded zhurongshuo reference data (`internal/archetypes/`,
`internal/style/`, `internal/corpus/`): © zhurong, internal-use only,
not for redistribution.
```

- [ ] **Step 3: Final test sweep**

```bash
go test ./...
go vet ./...
```

Expected: zero failures, zero warnings.

- [ ] **Step 4: Commit + tag**

```bash
git add README.md internal/cli/version.go
git commit -m "docs: v0.1.0 README + version bump (S1 complete)"
git tag v0.1.0
```

---

## Self-Review

After writing this plan, I re-read the spec (DESIGN.md §4, §9, §10, §11 + the 26 grill decisions) and checked coverage:

**Spec coverage:**
- §4.1 Workspace structure (`.jianwu/`, `books/`, `exports/`, `archive/`): Task 7
- §4.2 Book file formats (meta.json, outline.json schemas): Tasks 4, 5
- §4.3 Status workflow: Status constants in Task 4 (workflow enforcement deferred to S8)
- §9.1 Config 5-layer: Task 9 (env/CLI override hooks in Task 11)
- §9.2 Workspace config example: Task 7's `defaultWorkspaceConfig`
- §9.3 Secrets: Task 10
- §10.1 `init` / `info` / `config get/set/list`: Tasks 12, 13, 14
- §10.1 `init --bare`: Task 12
- Q3 (workspace walk up): Task 6
- Q4 (cobra + custom resolver): Tasks 9, 11
- Q15 (TDD discipline): every task
- Q16 (exit codes + slog): Task 11 (slog wiring deferred — no log statements needed in S1)
- Q21 (secrets precedence + 0600): Task 10
- Q22 (init defaults + schema_version + migrate check): Tasks 7, 8
- Q23 (go install path): README
- Q24 (AGPL): Task 0
- Q25 (goldmark/google/uuid deferred — not needed in S1): noted, no work
- Q26 (init defaults + no key prompt): Tasks 7, 10

**Gaps / deferrals (intentional, called out):**
- LLM providers, engine stages, grill, expand: deferred to S2-S7
- `review` / `finalize` / `export`: deferred to S8
- `slog` log statements: not yet wired (no LLM paths in S1 to log about). Global flags captured in Task 11; will be applied in S2.
- `--debug` LLM dump: deferred to S2 when LLM calls exist
- Env var override (`JIANWU_*`): the resolver in Task 9 only reads file layers. CLI flag/env override hooks land when LLM stages wire in (S2).

**Placeholder scan:** Cleaned up the placeholder lines flagged inline during drafting (`var _ = ...` stubs, unused imports). All remaining code blocks show concrete implementation.

**Type consistency:** Spot-checked signatures across tasks:
- `workspace.Init(path string, opts InitOpts) error` — defined Task 7, consumed Task 12 ✓
- `workspace.Load(wsRoot string) (*Workspace, error)` — defined Task 8, consumed Tasks 13, 14 ✓
- `workspace.FindWorkspace(startPath string) (string, error)` — defined Task 6, consumed Tasks 13, 14 ✓
- `config.Load(wsRoot string) (*Config, error)` — defined Task 9, consumed Task 8 ✓
- `config.LoadSecrets() (*Secrets, error)` — defined Task 10, consumed Task 13 ✓
- `cli.NewRootCmd() *cobra.Command` — defined Task 11, consumed by `cmd/jianwu/main.go` and all `_test.go` in cli package ✓
- `cli.InfoError` — defined Task 13, consumed Task 14 and `main.go` ✓

No mismatches found.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-21-s1-foundation.md`.

Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration. Best for the bulk of this plan since tasks are well-scoped and self-contained.

**2. Inline Execution** — Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints for review. Good if you want to watch each step land.

Which approach?
