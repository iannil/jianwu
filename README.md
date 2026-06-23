# jianwu

> 简物（jiàn wù）—— 把 AI 的训练知识结构化为人类可阅读、可学习的图书。

Library + CLI. Web SaaS wrapper is a separate repo (`mouqin`).

## Status

**v0.1.x (in progress)**: v0.1.0 shipped the library API + `jianwu new`; v0.1.x is bringing the CLI surface to match the v0.1 promise (expand / review / finalize / export / status commands + zhurongshuo-style prompt injection). See `docs/ROADMAP.md` for the v0.1.x plan and `docs/decisions/26-grill-decisions.md` § v0.1.x 完成度审计决策 for why.

## Install

```bash
go install github.com/iannil/jianwu/cmd/jianwu@latest
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
4. environment variables (e.g. `JIANWU_OUTLINE_MODEL=glm-4.6`)
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

## Engine (v0.1.0)

jianwu v0.1.0 ships the full 4-stage engine + the `new` command:

- **Outline** (v0.3.0): single LLM call produces full book outline
- **Scaffolding** (v0.4.0): N chapters in parallel, continue-on-error
- **Grill** (v0.5.0): stateful interactive Q&A with 12-dimension design tree
- **Expand** (v0.1.0): per-chapter 3-iteration agent (research → draft → validate), web search grounding, [^N] citation tracking
- **`jianwu new`** (v0.6.0): command chaining grill → outline → scaffolding

### v0.1 status

`jianwu new` produces a scaffolded book; `jianwu expand <slug> <NN-MM>` (shipped v0.1.1, 2026-06-23) produces one chapter's markdown + citations + outline.json status update.

Remaining work for v0.1.x (per `docs/ROADMAP.md`):
- Expand prompt injection (archetype YAML + style samples + adjacent chapters) — v0.1.2, currently produces generic LLM markdown, not zhurongshuo style
- review / finalize / export / status CLI commands — v0.1.3
- Fallback model wiring — v0.1.4
- Real timeouts on LLM calls — v0.1.5
- Streaming output for long-running stages — v0.1.6 (optional)

## License

Code: AGPL-3.0 (see `LICENSE`).
Embedded zhurongshuo reference data (`internal/archetypes/`,
`internal/style/`, `internal/corpus/`): © zhurong, internal-use only,
not for redistribution.
