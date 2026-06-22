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

## Engine (v0.3.0)

The 4-stage engine is being built slice by slice. v0.3.0 ships the **Outline** stage:

- Input: archetype ID + grill parameters (topic, audience, depth, goal, length, language)
- Output: complete `outline.json` with parts and chapters
- Stateless single LLM call with structured output (JSON Schema enforced)
- Prompt injects: archetype YAML, few-shot style samples, matching reference book outlines
- Wrapped in RetryWrapper + FallbackWrapper for resilience

Remaining stages (deferred):
- Scaffolding (parallel per-chapter, S4)
- Grill (interactive stateful, S5)
- Expand (agent loop + web search, S7)

## License

Code: AGPL-3.0 (see `LICENSE`).
Embedded zhurongshuo reference data (`internal/archetypes/`,
`internal/style/`, `internal/corpus/`): © zhurong, internal-use only,
not for redistribution.
