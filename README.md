# jianwu (肩吾)

English | [中文](README.zh.md)

Turn knowledge into structured, reviewable non-fiction books with traceable sources.

jianwu is an independent, local-first Go CLI for writers, researchers and people organizing knowledge. It guides book design, outlines and chapter drafting, records source checks, and keeps prose, citations and progress in your workspace.

**Development version: 0.3.6-dev, not yet released.** [Project status](docs/PROJECT_STATUS.md) · [Capabilities](docs/CAPABILITIES.md) · [Roadmap](docs/ROADMAP.md)

## Workflow

```text
Design interview → Outline → Chapter scaffolds → Research / Draft / Validate
                                                        ↓
                                                 Fact-check ↔ Revise
                                                        ↓
                                             Human review → Finalize → Export
```

- Six structural archetypes, reference material and style guidance.
- Gemini, GLM and Ollama providers, with search and source reading.
- Chapter status, explicit claim-to-citation IDs and verification records.
- Batch expansion with up to five concurrent generators and serialized saves; failures produce a nonzero exit code.
- Cumulative provider-reported LLM tokens, including observable stream/retry/fallback usage; missing reports are marked incomplete.
- Markdown, Hugo and PDF export (PDF requires pandoc/xelatex).

Source verification assists human review. Book quality and learning outcomes still require [real evaluation](docs/EVALUATION.md).

## Getting started

Requires Go 1.25+. These changes are not tagged yet; build this checkout:

```sh
git clone https://github.com/iannil/jianwu
cd jianwu
go build -o ./bin/jianwu ./cmd/jianwu
./bin/jianwu --version
```

Add the directory containing `bin/jianwu` to PATH. The default configuration uses GLM, Gemini, Brave and Jina; configure the corresponding API keys. See the [full setup guide](docs/getting-started.md).

```sh
jianwu init my-workspace
cd my-workspace
jianwu new --tokens
# Find the actual topic-derived slug under books/
book_slug='replace-with-actual-book-slug'
jianwu expand "$book_slug" --all --tokens
jianwu factcheck "$book_slug" 01-01
# Revise if needed, fact-check again, then read the prose and original sources
jianwu review "$book_slug" 01-01
# Repeat verification/review for every chapter before finalizing
jianwu finalize "$book_slug"
jianwu export "$book_slug" --target md
```

Back up your workspace before writing. Do not modify one book from multiple CLI processes simultaneously. Legacy claims without citation IDs remain unverified; back up and re-expand to establish explicit associations.

## Web UI

Prefer a visual workflow? `jianwu serve` starts a local web UI (default http://127.0.0.1:8787, localhost only):

```sh
cd my-workspace
jianwu serve
```

The browser covers the whole pipeline: workspace init, the 12-dimension design interview, outline/scaffolding generation, per-chapter expand/fact-check/revise/review, batch expand, finalize, and export download — plus corpus management and config inspection. Long operations run as background jobs with live progress and logs; all writes are serialized inside the server under the same one-writer-per-book constraint as the CLI.

The same workspace is also exposed as a JSON API at `http://127.0.0.1:8787/api/v1/` for local integrations; CLI commands and the engine layer are unchanged. See [CAPABILITIES](docs/CAPABILITIES.md).

## Development and release

```sh
go test -race ./...
go vet ./...
scripts/release_test.sh
scripts/release.sh 0.3.6 --dry-run
```

The [release workflow](docs/RELEASING.md) produces a local binary, build metadata and checksums. It never tags or pushes automatically. Engine packages live under `internal/`; no public Go SDK is currently offered.

jianwu no longer serves as mouqin's engine. The [independent product decision](docs/decisions/28-independent-product.md) supersedes the former SaaS roadmap. Historical website assets remain in this repository; this change does not deploy or migrate them.

## License

Code: AGPL-3.0; see [LICENSE](LICENSE). Embedded zhurongshuo reference data (`internal/archetypes/`, `internal/style/`, `internal/corpus/`): © zhurong, internal use only, not for redistribution.
