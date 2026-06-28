# jianwu

English | [中文](README.zh.md)

> 肩吾 — turn an LLM's training knowledge into human-readable, learnable books.

Library + CLI for generating Chinese long-form non-fiction. The web SaaS wrapper lives in a separate repo (`mouqin`).

## Current status

**Released: v0.1.3.** The full authoring loop works end to end — `new → expand → review → finalize → export`.

| Surface | Item | Status |
|---|---|---|
| CLI | `init` / `info` / `config get·set·list` / `new` | ✅ delivered |
| CLI | `expand <slug> <NN-MM>` | ✅ delivered |
| CLI | `review` / `finalize` / `export` / `status` (state machine) | ✅ delivered |
| Engine | Grill · Outline · Scaffolding · Expand (4 stages) | ✅ delivered |
| Quality | Expand prompt injection (archetype + style guide + samples + adjacent chapters) | ✅ delivered |
| CLI | Config-driven fallback model wiring | ⏳ planned (v0.1.4) |
| CLI | Per-stage LLM timeouts | ⏳ planned (v0.1.5) |
| CLI | Streaming draft output | ⏳ optional (v0.1.6) |

See [`docs/ROADMAP.md`](docs/ROADMAP.md) for the plan and [`docs/PROJECT_STATUS.md`](docs/PROJECT_STATUS.md) for a full state-of-the-project snapshot.

## Install

```bash
go install github.com/iannil/jianwu/cmd/jianwu@latest
```

Or build from source:

```bash
git clone https://github.com/iannil/jianwu
cd jianwu
go build -o ./bin/jianwu ./cmd/jianwu
```

## Quick start

The full loop, from empty workspace to a single exported markdown book:

```bash
jianwu init my-library
cd my-library

# API keys (or put them in ~/.config/jianwu/secrets.yaml, mode 0600)
export GEMINI_API_KEY=...
export GLM_API_KEY=...

jianwu new                     # interactive grill → outline → scaffolding
                               #   → books/<slug>/{meta.json, outline.json}
jianwu expand <slug> 01-01     # research → draft → validate; writes
                               #   chapters/01-01.md + citations, updates outline.json
jianwu review  <slug> 01-01    # mark one expanded chapter as reviewed (human-approved)
jianwu status  <slug>          # chapter-by-chapter progress + next-action hint
jianwu finalize <slug>         # all reviewed → final  (use --dry-run to preview)
jianwu export   <slug>         # merge chapters → books/<slug>/export/<slug>.md
                               #   (global footnote renumbering; --dry-run supported)
```

`<slug>` is the kebab-case book id printed by `jianwu new` (also the directory name under `books/`). `<NN-MM>` is the chapter address: part `NN`, chapter `MM`.

The state machine is strict: a chapter must be `expanded` before `review`, and every chapter must be `reviewed` before `finalize`. `outline.json` is the single source of truth for status; chapter `.md` frontmatter is mirrored from it.

## Configuration

Five layers, low → high precedence:

1. Built-in defaults (`internal/config/defaults.go`)
2. `~/.config/jianwu/config.yaml` (global user)
3. `<workspace>/.jianwu/config.yaml`
4. Environment variables (e.g. `JIANWU_OUTLINE_MODEL=glm-4.6`)
5. CLI flags (e.g. `--model glm-4.6`)

```bash
jianwu config get models.outline.provider
jianwu config set scaffolding.concurrency 10
jianwu config list
```

**Secrets** live in `~/.config/jianwu/secrets.yaml` (enforced mode `0600`) or environment variables: `GEMINI_API_KEY` / `GLM_API_KEY` / `BRAVE_API_KEY` / `SERPER_API_KEY` / `JINA_API_KEY`. ENV overrides file, field by field.

**Default model per stage:**

| Stage | Default model |
|---|---|
| Grill | GLM-4.6 |
| Outline | Gemini 2.5 Pro |
| Scaffolding | Gemini 2.5 Flash |
| Expand | GLM-4.6 |

## Providers

Everything is behind small Go interfaces (`Chatter`, `Embedder`, `Searcher`, `Reader`); engine layers compose them.

**LLM**
- **Gemini** via the official `google.golang.org/genai` SDK (`gemini-2.5-pro`, `gemini-2.5-flash`, `text-embedding-004`).
- **GLM** via direct REST, OpenAI-compatible client (`glm-4.6`, `glm-4-air`, `embedding-3`). The same client is reusable for Qwen / Moonshot / DeepSeek.
- **Mock** for unit tests.

**Search:** Brave Search API (primary) → Serper.dev (fallback).
**URL reader:** Jina Reader (`r.jina.ai`).

**Retry:** 3 attempts, exponential backoff (1s → 2s → 4s) + ±20% jitter on network / 429 / 5xx, context-aware (Ctrl+C cancels immediately). A `FallbackWrapper` exists at the library level; config-driven fallback-model selection from the CLI is wired in v0.1.4.

## Engine

jianwu ships the full 4-stage engine. Each stage is an independent, callable library package.

```
jianwu new
  ↓
grill.Run        # 12-dimension design tree; user accepts / edits the LLM's recommendation
  ↓
outline.Generate # single LLM call, JSON-Schema-forced output → book.Outline
  ↓
scaffolding      # N chapters in parallel (errgroup, continue-on-error)
  ↓
books/<slug>/{meta.json, outline.json}  +  archived session at .session.json
```

```
jianwu expand <slug> <NN-MM>
  ↓ iter 1  research   web_search × N + read_url × M → research notes + citation candidates
  ↓ iter 2  draft      LLM writes markdown + [^N] footnotes
  ↓ iter 3  validate   self-check + revise → claims[].has_citation
  ↓
ParseFootnotes + merge citation metadata
  → chapters/NN-MM.md (frontmatter + prose) + outline.json status/citations/word_count update
```

Expand prompts inject the real archetype YAML, the full style guide, few-shot style samples, and adjacent-chapter excerpts — so the prose targets the zhurongshuo register rather than generic LLM output.

## Architecture

```
cmd/jianwu/main.go                    # CLI entry (exit-code mapping)
internal/
  cli/                                # cobra command layer
    root / init / info / config / new
    expand / review / finalize / export / status
    prompt                            # TerminalPrompt (grill.UserInput impl)
    providers / new_flow              # orchestration + provider assembly
  workspace/                          # .jianwu/ load, walk-up detect, Init/Load
  config/                             # 5-layer resolver + secrets (ENV > file, 0600)
  book/                               # Meta/Outline/Chapter/Citation types + JSON I/O + Slugify
  archetypes/ style/ corpus/          # embedded data (//go:embed FS)
  provider/
    llm/        gemini/ glm/ mock/    # Chatter + Embedder + Retry/Fallback wrappers
    search/     brave/ serper/        # Searcher
    reader/     jina/                 # Reader
    llmfactory/ searchfactory/ readerfactory/   # factories (separate pkgs to break import cycles)
  engine/
    outline/                          # single LLM call → book.Outline
    scaffolding/                      # N-chapter parallel + RetryFailed
    grill/                            # 12-dimension design tree + stateful session
    expand/                           # 3-iteration agent + tool calls + citation parsing
```

**Package dependency graph (acyclic):** `cli → engine → provider → book / config / workspace`. The three `*factory` packages span the provider subtree to break import cycles.

**Workspace layout:**

```
<workspace>/
  .jianwu/
    config.yaml            # workspace config (overrides global)
    schema_version         # "1"（已废弃，不再写入或校验）
    sessions/<id>.json     # in-flight grill sessions
  books/<slug>/
    meta.json              # id / slug / title / archetype / parameters / engine versions
    outline.json           # parts[] × chapters[]; per-chapter status + citations (source of truth)
    .session.json          # completed grill session (audit log)
    chapters/NN-MM.md      # expand output: frontmatter + markdown + [^N] footnotes
    export/<slug>.md       # `jianwu export` output
  exports/  archive/       # reserved (v0.2+)
```

Chapter `status` flows `scaffolded → expanded → reviewed → final` (`failed` on error).

## Library API

All engine logic is in the library; the CLI and the future web app wrap the same packages.

```go
outline.Generate(ctx, chatter, Input) (*book.Outline, error)

scaffolding.GenerateChapter(ctx, chatter, ChapterInput) (*ChapterOutput, error)
scaffolding.ScaffoldAll(...) map[string]Result
scaffolding.RetryFailed(...)

grill.NewSession()
grill.Run(...)
grill.Repository{ Save, Load, ListIncomplete, Archive }

expand.Generate(ctx, chatter, tools, ExpandInput) (*ExpandOutput, error)
```

Provider interfaces:

```go
type Chatter  interface { Chat (ctx, ChatRequest)  (*ChatResponse,  error) }
type Embedder interface { Embed(ctx, EmbedRequest) (*EmbedResponse, error) }
type Searcher interface { Search(ctx, Query)       (*Results,       error) }
type Reader   interface { Read (ctx, url string)   (string,         error) }
```

**Error classes** (drive retry / fallback / exit codes): `ErrNetwork` / `ErrRateLimit` / `ErrServer` retry then fall back; `ErrLLMProvider` (4xx) does neither. Exit codes: `0` ok · `1` generic · `2` usage · `3` workspace not found · `4` LLM error · `5` network error.

## Development

```bash
go test ./...                                 # all packages
go build -o ./bin/jianwu ./cmd/jianwu
go vet ./...  &&  gofmt -l .                   # must be clean

go test -run TestE2E ./internal/cli/...        # E2E happy path (mock provider)
GEMINI_API_KEY=xxx go test ./internal/engine/expand/... -run TestGenerateLive   # real LLM
```

**Testing strategy:** library/state-machine/pure-logic code is test-first (TDD); LLM-driven code is test-after with a Mock provider + `httptest`. Live integration tests skip when no API key is present.

**Development workflow (SDD — subagent-driven development):**

1. `/grill-me` — align on decisions across the design tree.
2. `writing-plans` — produce a task-by-task plan (each task TDD: RED → GREEN → commit).
3. `subagent-driven-development` — fresh implementer subagent per task + a task reviewer.
4. Slice complete → whole-branch review → one fix commit resolving all findings.
5. Tag `vX.Y.Z` + push.

Embedded data assets (via `//go:embed`): 3 archetype YAML files, 1 style guide + 3 few-shot samples, 6 built-in corpus JSON books. Runtime has zero external dependency on the zhurongshuo repo.

See [`docs/archive/DESIGN.md`](docs/archive/DESIGN.md) for the original design doc (v0.1 locked) and [`docs/decisions/26-grill-decisions.md`](docs/decisions/26-grill-decisions.md) for the decision record.

## License

Code: **AGPL-3.0** (see [`LICENSE`](LICENSE)).

Embedded zhurongshuo reference data (`internal/archetypes/`, `internal/style/`, `internal/corpus/`): © zhurong, internal-use only, not for redistribution.
