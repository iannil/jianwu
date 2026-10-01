# Independent Product Implementation Plan

> **For agentic workers:** Use subagent-driven-development for independent implementation tasks and review; the primary agent owns integration and product documentation.

**Goal:** Deliver jianwu as an independent CLI product with reliable generation, fact-checking, usage accounting and local release tooling.
**Architecture:** Explicit citation IDs; bounded generation workers and a single persistence coordinator; book-owned usage data; internal engines remain private.
**Tech Stack:** Go 1.25+, cobra, existing providers and storage.
**Spec:** docs/superpowers/specs/2026-09-27-independent-product-design.md

## Global Constraints

No new runtime dependencies. No remote publication or website deployment. Preserve old book JSON readability. No inferred citation associations. Tests must exercise observable data flow and failure handling. No fabricated live evaluation results.

## Tasks and ownership

- [x] Quality pipeline: book claim citation IDs, expand output/schema/prompts, factcheck matching and errors, revise revalidation. Tests reproduce missing claims, shuffled citations, missing/multiple sources and stale review state. Owner: quality worker. Files: internal/book/types.go (claims only), internal/engine/{expand,factcheck}, internal/cli/{factcheck,revise}.go and their tests.
- [x] Usage: book usage data, tracking wrappers, new/status wiring and helpers. Tests for accumulation, concurrent tracking, failure and stream accounting, old metadata. Owner: usage worker. Files: internal/book/usage.go, Meta usage field only, internal/engine/usage*, internal/cli/{usage,new,new_flow,status}*.
- [x] Release: version handlers, scripts/release.sh, tests for metadata and dry-run artifact. Owner: release worker. Files: internal/cli/{root,version}*, scripts/release.sh. No tag/push during validation.
- [x] Batch and persistence: extract generation result from runExpand, bounded workers, serial merge/save and failure exit. Test multi-chapter state retention and partial failure; add atomic write failure tests. Owner: primary. Files: internal/cli/expand*, internal/book/{io,chapter}*, internal/storage atomic helper.
- [x] Independent product docs and evaluation: replace SaaS roadmap with reliability + sample evaluation; correct quick start; add measured evaluation protocol and compatibility notes; mark obsolete plan superseded. Owner: primary.
- [x] Integration review: go test -race ./..., go vet ./..., gofmt; local release build and version checks; review complete diff, fix concrete regressions, document validation and remaining limits.

## Decisions

User approval supersedes old mouqin plan and repeated design approval gates. Existing workspace starts clean; all edits remain reviewable here. Workers own disjoint files except book/types.go: claim additions and Meta usage additions must be coordinated, never overwrite whole files. Public SDK, multi-tenant work and live deployment removed from scope. Real reader study requires actual participants and is documented separately from code acceptance.
