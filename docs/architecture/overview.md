# jianwu 架构总览

> 本文档给 LLM 一份"如果只读一个文件就要能改 jianwu 代码"的架构地图。
> 详细 API 见 `PROJECT_STATUS.md` + 各包的 godoc。

---

## 包依赖图（无环）

```
cmd/jianwu/main.go
    │
    ▼
internal/cli ──────────┬─→ internal/engine/{outline, scaffolding, grill, expand}
    │                  │       │
    │                  │       ├─→ internal/book（types + JSON I/O）
    │                  │       ├─→ internal/archetypes, internal/style, internal/corpus（embed FS）
    │                  │       └─→ internal/provider/llm（Chatter 接口）
    │                  │
    │                  ├─→ internal/provider/{llmfactory, searchfactory, readerfactory}
    │                  │       │
    │                  │       └─→ internal/provider/{llm, search, reader}/*（具体实现）
    │                  │
    │                  ├─→ internal/workspace（.jianwu/ 加载、Init、Load）
    │                  └─→ internal/config（5 层 resolver + secrets）
    │
    └─→ internal/cli 自身（cobra 命令、TerminalPrompt、providers 装配）
```

**为什么有 `llmfactory` / `searchfactory` / `readerfactory` 独立包？**

避免 import cycle：
- `internal/provider/llm/factory.go` import `llm/gemini` + `llm/glm`
- 但 `llm/gemini` + `llm/glm` 都 import `llm`（为了 `Chatter` 接口）
- → 循环

把 factory 放到平级的 `llmfactory` 包，就能 import `llm` + `llm/gemini` + `llm/glm` 三者。

## 数据流（v1.0 `jianwu new` 全流程）

```
[jianwu new]
    │
    ├─ workspace.FindWorkspace(".")  ← walk up 找 .jianwu/
    ├─ workspace.Load(wsRoot)        ← 验证 schema_version + 加载 config
    ├─ config.LoadSecrets()          ← ENV > file（0600 强制）
    │
    ├─ offerResume(repo)             ← 列出 .jianwu/sessions/*.json 中 status=in_progress
    │   └─ 用户选择恢复 → 加载该 session
    │
    ├─ grill.Run × 12 维度
    │   ├─ LLM 生成推荐（per dimension）
    │   ├─ TerminalPrompt.Ask（用户接受/修改/跳过）
    │   └─ repo.Save(session)  ← 每步落盘，Ctrl+C 可恢复
    │
    ├─ deriveSlugFromTopic(session.Answers["topic"])
    ├─ checkSlugConflict(wsRoot, slug, --force)
    │
    ├─ outline.Generate(chatter, Input)
    │   ├─ buildPromptData: load archetype YAML + corpus outlines + style samples
    │   ├─ render system + user templates
    │   ├─ chatter.Chat（RetryWrapper 装配）
    │   └─ JSON parse → book.Outline
    │
    ├─ writeBookMeta + SaveOutline
    │
    ├─ scaffolding.ScaffoldAll(chatter, outline, archetypeID, params, opts)
    │   ├─ errgroup.SetLimit(5)
    │   ├─ 每章: GenerateChapter → LLM chat → parse JSON → 更新 outline
    │   └─ continue-on-error: 失败章 status=failed，其他继续
    │
    ├─ SaveOutline（含 scaffolded 章节字段）
    │
    └─ repo.Archive(session, slug)  ← 移到 books/<slug>/.session.json（audit log）
```

## 数据流（v1.0.x `jianwu expand` — 待实现）

```
[jianwu expand <slug> <NN-MM>]
    │
    ├─ workspace + config + secrets
    ├─ LoadMeta + LoadOutline
    ├─ 找到指定 chapter（part NN, chapter MM）
    │
    ├─ ToolRegistry 装配:
    │   ├─ Searcher: Brave（primary）+ Serper（fallback）
    │   ├─ Reader: Jina
    │   ├─ Embedder: 与 chatter 同 provider
    │   └─ Outline callback: 读相邻章节
    │
    ├─ expand.Generate(ctx, chatter, tools, ExpandInput)
    │   ├─ iter 1 RunResearch:
    │   │   ├─ buildResearchQueries（topic + chapter + key concepts）
    │   │   ├─ tools.SearchAndRegister × N（cap 5）→ 注册 citation metadata
    │   │   ├─ tools.ReadURL × M（cap 10）→ 补全 reader_provider
    │   │   └─ LLM call → ResearchNotes JSON
    │   │
    │   ├─ iter 2 RunDraft:
    │   │   └─ LLM call → markdown + [^N] footnotes
    │   │
    │   ├─ iter 3 RunValidate:
    │   │   └─ LLM call → ValidationResult JSON（revised_markdown + claims[].has_citation）
    │   │
    │   ├─ ParseFootnotes(finalMD) → map[ID]FootnoteDef
    │   ├─ mergeCitations(defs, tools.Citations())  ← URL 匹配补全 metadata
    │   └─ 统计 unverified_claims（has_citation=false）
    │
    └─ SaveChapter + 更新 outline.json（status=expanded + citations + word_count + unverified_claims）
```

## 关键接口

### Provider 抽象

```go
// internal/provider/llm/interface.go
type Chatter interface {
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}
type Embedder interface {
    Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)
}
// Streamer / Tooler 接口预留（v1.0.4 流式 / v1.0.x 工具调用）

// internal/provider/search/interface.go
type Searcher interface {
    Search(ctx context.Context, query string, opts SearchOpts) ([]SearchResult, error)
}

// internal/provider/reader/interface.go
type Reader interface {
    Read(ctx context.Context, url string) (Content, error)
}
```

### 引擎入口

```go
// outline
outline.Generate(ctx, chatter llm.Chatter, in outline.Input) (*book.Outline, error)

// scaffolding
scaffolding.GenerateChapter(ctx, chatter, in ChapterInput) (*ChapterOutput, error)
scaffolding.ScaffoldAll(ctx, chatter, outline, archetypeID, params, opts) map[string]Result
scaffolding.RetryFailed(ctx, chatter, outline, archetypeID, params, opts) map[string]Result

// grill
grill.Run(ctx, chatter, tree, session, ui UserInput) (*Dimension, error)  // caller 循环
grill.NewSession() *Session
grill.NewRepository(workspaceRoot) *Repository

// expand
expand.Generate(ctx, chatter, tools *ToolRegistry, in ExpandInput) (*ExpandOutput, error)
expand.NewToolRegistry(searcher, reader, embedder, outlineFn) *ToolRegistry
```

## 错误处理

所有 LLM 错误都通过 `llm.ClassifyError(err, statusCode)` 包装为 `*HTTPError`，可 `errors.Is(err, llm.ErrNetwork)` 分类。

```go
// internal/provider/llm/errors.go
var (
    ErrNetwork     = errors.New("network error")     // 触发 retry + fallback
    ErrRateLimit   = errors.New("rate limited")       // 触发 retry + fallback
    ErrServer      = errors.New("server error")       // 触发 retry + fallback
    ErrLLMProvider = errors.New("llm provider error") // 4xx，不 retry
)
```

CLI 层通过 `*cli.InfoError{Err, Code}` 把错误映射到 exit code：

```go
// cmd/jianwu/main.go
if errors.As(err, &ie) {
    os.Exit(ie.Code)  // 3/4/5
}
os.Exit(1)
```

## 测试策略

- 库代码：TDD（test-first）
- LLM-driven：test-after，Mock Provider + httptest
- 跨切：E2E 用 `chatterProviderHook`（test-only 全局）注入 mock
- Live：`GEMINI_API_KEY` / `GLM_API_KEY` 设置时跑真 API，否则 SKIP

## 配置加载顺序

```
高优先级 → 低优先级
1. CLI flag          (--model glm-4.6)
2. ENV var           (JIANWU_OUTLINE_MODEL)
3. Workspace config  (<ws>/.jianwu/config.yaml)
4. Global config     (~/.config/jianwu/config.yaml)
5. Builtin defaults  (internal/config/defaults.go)
```

Secrets 单独走：
```
高 → 低
1. ENV               (GEMINI_API_KEY)
2. Secrets file      (~/.config/jianwu/secrets.yaml, 0600 强制)
```

## 添加新功能时的检查清单

新 CLI 命令：
- [ ] 在 `internal/cli/<name>.go` 加 `new<Name>Cmd()`
- [ ] 注册到 `internal/cli/root.go` 的 `NewRootCmd()`
- [ ] 加 `InfoError` 包装 + 对应 exit code
- [ ] 加单元测试 + 1 个 E2E 测试

新 provider：
- [ ] 在 `internal/provider/<type>/<name>/` 实现 interface
- [ ] 加 `httptest` 单元测试
- [ ] 在对应 `*factory` 包加 case
- [ ] 加 factory 测试

新引擎阶段：
- [ ] 在 `internal/engine/<name>/` 建包
- [ ] `types.go` + `embed.go`（prompt 模板）+ `schema.go`（JSON Schema）+ 主入口 `<name>.go`
- [ ] TDD 单元测试 + live integration 测试
- [ ] 在 `cli/new_flow.go` 或新命令里编排
