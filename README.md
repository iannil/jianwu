# jianwu

> 简物（jiàn wù）—— 把 AI 的训练知识结构化为人类可阅读、可学习的图书。

Library + CLI. Web SaaS wrapper is a separate repo (`mouqin`).

## Status

**v1.0.0 — S7 (Expand complete)**: Full 5-stage engine working (Outline, Scaffolding, Grill, `new` command, Expand). CLI expand command pending v1.0.x patch. See `docs/superpowers/plans/` for roadmap.

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

## Engine (v1.0.0)

jianwu v1.0.0 ships the full 4-stage engine + the `new` command:

- **Outline** (v0.3.0): single LLM call produces full book outline
- **Scaffolding** (v0.4.0): N chapters in parallel, continue-on-error
- **Grill** (v0.5.0): stateful interactive Q&A with 12-dimension design tree
- **Expand** (v1.0.0): per-chapter 3-iteration agent (research → draft → validate), web search grounding, [^N] citation tracking
- **`jianwu new`** (v0.6.0): command chaining grill → outline → scaffolding

### v1.0 status

`jianwu new` produces a scaffolded book end-to-end. The Expand engine is library-only in v1.0.0 — a CLI command (`jianwu expand <slug> <NN-MM>`) is pending v1.0.x.

Remaining work for v1.0.x:
- CLI command for expand (`jianwu expand <slug> <NN-MM>`)
- Fallback model wiring (Config carries primary only today)
- Streaming output for long-running stages
- Real timeouts on LLM calls

## License

Code: AGPL-3.0 (see `LICENSE`).
Embedded zhurongshuo reference data (`internal/archetypes/`,
`internal/style/`, `internal/corpus/`): © zhurong, internal-use only,
not for redistribution.
