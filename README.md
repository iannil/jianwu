# jianwu (肩吾)

English | [中文](README.zh.md)

> From LLM knowledge to published books — one CLI command.

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev)
[![Go Report Card](https://goreportcard.com/badge/github.com/iannil/jianwu)](https://goreportcard.com/report/github.com/iannil/jianwu)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen)](CONTRIBUTING.md)
[![GitHub Stars](https://img.shields.io/github/stars/iannil/jianwu?style=social&label=Stars)](https://github.com/iannil/jianwu)

**jianwu** is a Go CLI + library that orchestrates LLMs to write long-form non-fiction books — with table of contents, consistent citations, chapter-level state management, and multi-format export.

The web SaaS version: [mouqin.com](https://mouqin.com) — Early Access

---

## Who is this for?

- **Writers & researchers** — turn research notes into structured, footnoted books
- **LLM power users** — frustrated by token limits and lack of book-level structure
- **Chinese non-fiction authors** — AI-assisted writing pipeline built for long-form Chinese
- **Go developers** — interested in LLM orchestration, provider abstraction, and prompt engineering

## The problem

LLMs have vast training knowledge but can only output tokens. They can't give you a book with a table of contents, consistent citations, and chapter-level state management. Most AI writing tools produce **text** — paragraphs, emails, blog posts.

**jianwu produces books.**

## Features

| Area | Capabilities |
|---|---|
| **Authoring pipeline** | Grill (design) → Outline → Scaffold → Expand (research → draft → validate) |
| **Quality assurance** | Per-claim factcheck with URL verification + auto-revision |
| **Export** | Markdown, Hugo site, PDF (single-book or full-site) |
| **LLM providers** | Gemini 2.5 Pro/Flash, GLM-4, Ollama (local: Qwen, Llama, etc.), Mock |
| **Search & reading** | Brave Search, Serper (fallback), Jina Reader |
| **Corpus** | Built-in reference books + embedding index for RAG |
| **Config** | 5-layer merge (default → global → workspace → env → flags) |
| **Developer experience** | Narrow Go interfaces, state machine, table-driven tests |

**Current release: v0.3.5.** [Full status →](docs/PROJECT_STATUS.md) | [Roadmap →](docs/ROADMAP.md)

## Quick start

```bash
# Install
go install github.com/iannil/jianwu/cmd/jianwu@latest

# Or build from source
git clone https://github.com/iannil/jianwu
cd jianwu && go build -o ./bin/jianwu ./cmd/jianwu

# Set API keys
export GEMINI_API_KEY=...

# Write a book — from zero to exported markdown in one session
jianwu init my-book && cd my-book
jianwu new                     # grill → outline → scaffolding
jianwu expand my-book 01-01    # research → draft → validate
jianwu status my-book          # progress + next action
jianwu finalize my-book        # lock the book
jianwu export my-book          # one-book markdown with footnotes
```

📖 [Full getting-started guide →](docs/getting-started.md)

## Architecture

```
cmd/jianwu/main.go
├── internal/cli/            # cobra command tree
├── internal/engine/
│   ├── grill/               # 12-dimension design tree
│   ├── outline/             # single LLM call → JSON Schema output
│   ├── scaffolding/         # N chapters in parallel (errgroup)
│   ├── expand/              # 3 iterations: research → draft → validate
│   ├── factcheck/           # per-claim URL verification
│   └── revise/              # auto-revision with new citations
├── internal/provider/
│   ├── llm/                 # Chatter / Embedder / Streamer interfaces
│   ├── search/              # Searcher interface
│   ├── reader/              # Reader interface
│   └── gemini/ glm/ ollama/ brave/ serper/ jina/  # implementations
├── internal/book/           # Meta / Outline / Chapter / Claim types
├── internal/config/         # 5-layer config merge
├── internal/workspace/      # workspace management
├── internal/storage/        # Storage interface (OS + MemStorage)
└── internal/corpus/         # reference corpus + embedding index
```

**Key design decisions:**
- Narrow interfaces (5 lines to define a provider)
- No global state, no `init()` (except `//go:embed`)
- State machine: `scaffolded → expanded → reviewed → finalized`
- Table-driven tests with Mock provider; zero testify dependency
- AGPL-3.0 (code) + internal-use zhurongshuo data

## Providers

Everything is behind small Go interfaces. Add a new provider by implementing the interface and registering the factory.

- **LLM:** Gemini 2.5 Pro/Flash, GLM-4.6/Air, Ollama (local: Qwen, Llama, etc.), Mock
- **Search:** Brave Search (primary) → Serper (fallback)
- **Read:** Jina Reader

**Retry:** 3 attempts, exponential backoff (1s→2s→4s) + ±20% jitter; context-aware (Ctrl+C cancels). FallbackWrapper chains providers.

## Resources

| Resource | Link |
|---|---|
| Blog | [mouqin.com/blog/](https://mouqin.com/blog/) |
| Engine deep-dive | [mouqin.com/engine/](https://mouqin.com/engine/) |
| CLI reference | [mouqin.com/docs/commands/](https://mouqin.com/docs/commands/) |
| Configuration guide | [mouqin.com/docs/configuration/](https://mouqin.com/docs/configuration/) |
| Architecture docs | [docs/architecture/overview.md](docs/architecture/overview.md) |
| Roadmap | [docs/ROADMAP.md](docs/ROADMAP.md) |
| Design decisions | [docs/decisions/26-grill-decisions.md](docs/decisions/26-grill-decisions.md) |

## Contributing

PRs are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

**Development:**

```bash
go test ./...                       # all tests (uses Mock provider)
go build -o ./bin/jianwu ./cmd/jianwu
go vet ./...  &&  gofmt -l .        # must be clean

# Live LLM test (requires API key)
GEMINI_API_KEY=xxx go test ./internal/engine/expand/... -run TestGenerateLive
```

**SDD workflow:** We use subagent-driven development — `/grill-me` for design decisions, writing-plans for task-level plans, fresh subagent per task.

---

⭐ **If you find this project useful, give it a star — it helps others discover it.**

## License

Code: **AGPL-3.0** (see [LICENSE](LICENSE)).

Embedded zhurongshuo reference data (`internal/archetypes/`, `internal/style/`, `internal/corpus/`): © zhurong, internal-use only, not for redistribution.
