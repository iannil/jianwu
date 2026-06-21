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

## License

Code: AGPL-3.0 (see `LICENSE`).
Embedded zhurongshuo reference data (`internal/archetypes/`,
`internal/style/`, `internal/corpus/`): © zhurong, internal-use only,
not for redistribution.
