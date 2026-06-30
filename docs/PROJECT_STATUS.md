# jianwu 项目状态

> 本文档对 LLM 友好——任何接手后续迭代的 agent 读这一份就能理解项目当前形态、什么能用、什么没做、怎么扩展。
> 最后更新：2026-06-28（v0.3.5 — SaaS-ready 内核全线交付）

---

## TL;DR

jianwu 是一个把 LLM 训练知识结构化为人类可读图书的 Go 库 + CLI。

- **当前版本**：**v0.3.5**（v0.1.x 全线贯通 + factcheck/revise + Ollama + Storage + hugo/pdf + 章节迭代 + corpus sync + 6 原型 + 10 本语料 + v0.3 SaaS-ready **单租户**内核全线交付）
- **可用 CLI 命令**：`init` / `info` / `config get·set·list` / `new` / `expand`（含 `--all`）/ `review` / `finalize` / `export` / `status` / `factcheck` / `revise` / `rewrite` / `add-chapter` / `move-chapter` / `delete-chapter` / `corpus list·show·stats·sync·reindex`
- **库 API**：4 阶段引擎 `grill → outline → scaffolding → expand` + factcheck + revise
- **LLM providers**：Gemini / GLM / Ollama（本地模型）/ Mock（单元测试）
- **质量基线**：30+ 包测试全绿，`go vet` / `gofmt` 全清；`go test -race ./...` 全绿（v0.3.4 验证）
- **下一里程碑**：**v0.3.6** — 发布流程（`--version` + `release.sh`）+ Token 累计扩展 + 测试补全
- **v0.4（多租户接线）**：**触发式启动**，不在关键路径——mouqin 上线后由真实多租户需求触发。详见 `docs/decisions/27-v0.3-audit-decisions.md`

---

## 1. 项目目标

> **肩吾** —— 把 AI 的训练知识结构化为人类可阅读、可学习的图书。

用户给出主题，AI 走完 grill 问诊 → outline 生成 → scaffolding 脚手架 → expand 成稿（带 web search 引用）的全流程，
产出"配得上 zhurongshuo 同书架"的中文非虚构图书。完整状态机：`expanded → reviewed → final → export`。

## 2. 两个仓库

| 仓库 | 角色 | 状态 |
|---|---|---|
| `jianwu` | 核心引擎（库 + CLI） | **v0.3.5 shipped**（v0.1.x–v0.3.x 全线 + 6 原型 + 10 本语料 + SaaS-ready 单租户内核） |
| `jianwu`（同仓库） | mouqin.com 官网（Hugo 站 + waitlist API） | **已上线**（commit 07cd538）— co-hosted in jianwu repo |
| `mouqin` | Web SaaS app（包装 jianwu） | v1.0 未启动；前置 v0.3 SaaS-ready 内核已交付，mouqin MVP 单租户运行（v0.4 多租户推迟） |

库优先：所有核心逻辑在 `jianwu` 库里，CLI 和未来的 Web 都包同一个库。

## 3. 与 zhurongshuo 的关系

**完全独立。** zhurongshuo 是方法论参考 + 参考语料来源，**不是部署目标**。

- 一次性萃取：archetype YAML + few-shot 样例 + corpus outline 摘要 → 进 jianwu 二进制（`//go:embed`）
- 运行时零外部依赖：zhurongshuo 仓库消失，jianwu 仍能正常运行
- v0.2+ 的 `corpus sync` 才会重新读 zhurongshuo（尚未实现）

## 4. 当前架构

```
cmd/jianwu/main.go                    # CLI 入口（exit code mapping）
internal/
  cli/                                # cobra 命令层
    root / init / info / config / new # v0.1 命令
    expand / review / finalize        # v0.1.1-3 新增
    export / status                   # v0.1.3 新增
    factcheck / revise                # v0.2.0 新增
    prompt                            # TerminalPrompt（grill.UserInput 实现）
    providers / new_flow              # 编排 + provider 装配
    book_resolve                      # loadBook + mirrorChapterStatus 共享助手
    footnotes                         # 全局脚注重编号（export 用）
  workspace/                          # .jianwu/ 加载、walk-up detect、Init/Load（使用 storage.Storage）
  config/                             # 5 层 config resolver + secrets (ENV > file, 0600)
  book/                               # Meta/Outline/Chapter/Citation/ClaimVerdict 类型 + JSON I/O
  storage/                            # Storage 接口（v0.3.0）— OS / MemStorage，替换 book/workspace/grill 的 os.*
  archetypes/ style/ corpus/          # 内置数据（embed FS）
  provider/
    llm/                              # Chatter + Embedder + Streamer 接口 + Retry/Fallback wrappers
      gemini/ glm/ mock/ ollama/      # 四家实现（+ Ollama 本地模型 v0.2.1）
    search/                           # Searcher + Brave/Serper
    reader/                           # Reader + Jina
    llmfactory/ searchfactory/ readerfactory/  # 工厂（独立包避免 import cycle）
  engine/
    outline/                          # 单次 LLM 调用产出 book.Outline
    scaffolding/                      # N 章并行 + RetryFailed
    grill/                            # 12 维度 design tree + stateful session
    expand/                           # 3-iteration agent + 工具调用 + citation 解析
    factcheck/                        # v0.2.0 — claims 自动事实复核（逐 claim + Reader + LLM 验证）
    revise/                           # v0.2.0 — 基于 factcheck 结果的章节修订
```

**包依赖图（无环）：** `cli → engine → provider → book/config/workspace/storage`；
`{llm,search,reader}factory` 横跨 provider 子树以打破循环。

## 5. 核心引擎

### 4 阶段创作管线

| 阶段 | 包 | 模式 | 默认模型 | 状态 |
|---|---|---|---|---|
| Grill | `engine/grill` | stateful interactive | GLM-4.6 | ✅ 完成 |
| Outline | `engine/outline` | 单次 batch | Gemini 2.5 Pro | ✅ 完成 |
| Scaffolding | `engine/scaffolding` | N 章并行 | Gemini 2.5 Flash | ✅ 完成 |
| Expand | `engine/expand` | 3-iteration agent | GLM-4.6 | ✅ 完成（含 prompt 注入 + streaming） |

**关键流程（`jianwu new`）：**
```
grill.Run × 12 维度（用户接受/修改 LLM 推荐）
  ↓
outline.Generate（单次 LLM + JSON Schema 强制输出）
  ↓
scaffolding.ScaffoldAll（N 章 errgroup 并发，continue-on-error）
  ↓
save books/<slug>/{meta.json, outline.json}
  ↓
session archive to books/<slug>/.session.json
```

**Expand（3 迭代 agent）：**
```
expand.Generate(ctx, chatter, tools, input)
  ↓ iter 1: RunResearch
web_search × N + read_url × M  →  LLM 提取 research notes + citation candidates
  ↓ iter 2: RunDraft（注入 archetype + style guide + samples + 相邻章节）
LLM 写 markdown + [^N] footnotes（支持 streaming，`-L` 逐 token）
  ↓ iter 3: RunValidate
LLM 自检 + 修订（注入 style guide），输出 claims[].has_citation
  ↓
ParseFootnotes + mergeCitations(registry metadata)
  → ExpandOutput{Markdown, Citations[], UnverifiedClaims[], WordCount}
```

### 事实复核管线（v0.2.0）

```
jianwu factcheck <slug> <NN-MM>
  ↓
遍历 Claims[] — 对每个 HasCitation=true 的 claim：
  匹配 citations[i] → Reader.Read(url) → LLM 验证 claim 是否被 source 支持
  ↓
factcheck.Run() → Output{Verdicts[], SourceErrors[]}
  ↓
更新 outline.json（OutlineChapter.Verdicts）+ 跨章 ClaimWhitelist（meta.json）
  ↓
jianwu revise <slug> <NN-MM>（可选 — 基于 Verdicts.SuggestedRewrite 让 LLM 修订章节）
```

## 6. Provider 抽象

```go
// internal/provider/llm/interface.go
type Chatter interface { Chat(ctx, ChatRequest) (*ChatResponse, error) }
type Embedder interface { Embed(ctx, EmbedRequest) (*EmbedResponse, error) }
type Streamer interface { Stream(ctx, ChatRequest) (<-chan StreamChunk, error) }
```

| Provider | 实现 | 用途 |
|---|---|---|
| Gemini | `provider/llm/gemini`（官方 `google.golang.org/genai` SDK） | outline + scaffolding |
| GLM | `provider/llm/glm`（直 REST，OpenAI-compatible 复用） | intake + expand |
| Ollama | `provider/llm/ollama`（HTTP REST，`http://localhost:11434`） | 本地模型（Qwen3, etc.） |
| Mock | `provider/llm/mock` | 单元测试 |
| Brave | `provider/search/brave` | 主 search |
| Serper | `provider/search/serper` | 备 search |
| Jina | `provider/reader/jina` | URL → markdown |

**错误分类（影响 retry/fallback）：**
- `ErrNetwork` / `ErrRateLimit` / `ErrServer` → 触发 retry + fallback
- `ErrLLMProvider` (4xx) → 不 retry、不 fallback（无意义）
- 退出码：`0` 成功 / `1` 通用错 / `2` 用法错 / `3` workspace 未找到 / `4` LLM 错 / `5` 网络错

**Retry/Fallback：**
- 同 provider retry 3 次，指数退避 1s→2s→4s + ±20% jitter，ctx-aware（Ctrl+C 立即响应）
- Retry 耗尽 → fallback provider（`ModelRef.Fallback`）；fallback 也失败 → 返回最后一次错误
- primary==fallback 时跳过（打 warn 日志）

## 7. 数据模型

### Workspace（`.jianwu/`）
```
<jianwu-workspace>/
  .jianwu/
    config.yaml              # workspace 配置（覆盖全局）
    sessions/<id>.json       # 运行中的 grill 会话
  books/<slug>/
    meta.json                # Meta（含 ClaimWhitelist，跨章复用已验证声明）
    outline.json             # Outline（含 Verdicts[] 事实复核结果）
    .session.json            # 已完成的 grill 会话（audit log）
    chapters/NN-MM.md        # expand 产出（YAML frontmatter + markdown + [^N] footnotes）
    export/<slug>.md         # export --target md 产出
    export/<slug>/           # export --target hugo 产出（chapter-per-file）
```

完整 schema 见 `internal/book/types.go`。

### Key types in `internal/book/types.go`
- `Meta` — `ClaimWhitelist map[string]bool`（已跨章验证的声明，factcheck 复用）
- `OutlineChapter` — `Verdicts []ClaimVerdict`（factcheck 结果）
- `ClaimVerdict` — `{ClaimText, Verified, Reasoning, SuggestedRewrite, CitationID}`
- `Claim` — `{Text, HasCitation}`

## 8. 配置系统

**5 层（高 → 低优先级）：**
1. CLI flag（`--model glm-4.6`）
2. 环境变量（`JIANWU_OUTLINE_MODEL=...`）
3. Workspace `.jianwu/config.yaml`
4. 全局 `~/.config/jianwu/config.yaml`
5. 编译时 defaults（`internal/config/defaults.go`）

**Secrets：** `~/.config/jianwu/secrets.yaml`（0600 权限强制）或 ENV（`GEMINI_API_KEY` / `GLM_API_KEY` / `BRAVE_API_KEY` / `SERPER_API_KEY` / `JINA_API_KEY`）。ENV > file 字段级覆盖。

## 9. 已交付

### ✅ CLI 命令（完整列表）

| 命令 | 版本 | 说明 |
|---|---|---|
| `init [--bare] [path]` | v0.1.0 | 初始化 workspace |
| `info` | v0.1.0 | 工作区诊断信息 |
| `config get/set/list` | v0.1.0 | 配置查询与修改 |
| `new [--force]` | v0.1.0 | 完整 new 流程（grill→outline→scaffolding） |
| `expand <slug> <NN-MM> [--force]` | **v0.1.1** | 展开单个章节（research→draft→validate，支持 streaming） |
| `review <slug> <NN-MM>` | **v0.1.3** | 标记章节为已审阅 |
| `finalize <slug> [--dry-run]` | **v0.1.3** | 全书审阅完成后定稿 |
| `export <slug> [--dry-run] [--target md|hugo|pdf]` | **v0.1.3** | 合并章节导出（支持 md/hugo/pdf） |
| `status <slug>` | **v0.1.3** | 章节进度概览 + 下一步提示 |
| `factcheck <slug> <NN-MM>` | **v0.2.0** | 自动事实复核 — 逐 claim 验证引用来源 |
| `revise <slug> <NN-MM>` | **v0.2.0** | 基于 factcheck 结果，LLM 修订未通过章节 |
| `rewrite <slug> <NN-MM>` | **v0.2.2** | 重写章节（等价于 `expand --force --force`） |
| `add-chapter <slug> --after <NN-MM> --topic "..." [--as <NN-MM>]` | **v0.2.2** | 插入新章节（保留 gap 不重编号） |
| `move-chapter <slug> <NN-MM> <target-part> [--after <NN-MM>]` | **v0.2.2** | 移动章节到其他 part |
| `delete-chapter <slug> <NN-MM>` | **v0.2.2** | 删除章节（从 outline + 文件系统） |
| `expand --all <slug>` | **v0.2.2** | errgroup 并行展开全书 scaffolded 章节 |
| `corpus list/show/stats` | **v0.2.3** | 查看参考语料列表/详情/统计 |
| `corpus sync --from <path>` | **v0.2.3** | 从 zhurongshuo 目录同步扩展语料到 workspace |
| `corpus reindex` | **v0.2.3** | 调用 embedder 重建 embedding 索引并缓存 |

**全局标志：** `--verbose` / `-L`（INFO 日志），`--debug`（DEBUG + LLM 请求/响应 dump），`--dir` / `-d`（指定 workspace 根目录，默认 CWD）

### ✅ 引擎库 API
- `outline.Generate(ctx, chatter, Input) (*book.Outline, error)`
- `scaffolding.GenerateChapter(ctx, chatter, ChapterInput) (*ChapterOutput, error)`
- `scaffolding.ScaffoldAll(...) map[string]Result`
- `scaffolding.RetryFailed(...)`
- `grill.NewSession()` / `grill.Run(...)` / `grill.Repository{Save,Load,ListIncomplete,Archive}`
- `expand.Generate(ctx, chatter, tools, ExpandInput) (*ExpandOutput, error)`
- `factcheck.Run(ctx, chatter, reader, Input) (*Output, error)` — 逐 claim 真实性验证
- `revise.Run(ctx, chatter, Input) (*Output, error)` — 基于 verdicts 修订章节

### ✅ Provider 库
- LLM: Gemini（含 context cache helper）+ GLM（OpenAI-compatible HTTP client）+ Ollama（本地模型）+ Mock
- Search: Brave + Serper
- Reader: Jina
- Streamer: Gemini（genai SDK stream）+ GLM（SSE 解析）+ Ollama（stream API）+ Mock
- Retry + Fallback wrappers（ctx-aware）
- 5 个工厂函数（`llmfactory` / `searchfactory` / `readerfactory`）

### ✅ 数据资产（embed）
- **6 个原型 YAML**（本体-认识-实践 / 诊断-解码-破局 / 基础-应用-实战 / 心法-方法-实践 / 理论-动力-历史-当下 / 宏-中-微）
- 1 个 style-guide.md + **6 个 few-shot samples**（各原型对应）
- **10 本 builtin corpus JSON**（含 4 本新增：barbaric-order / data-as-the-boundary / open-map / revisiting-history）

### ✅ Expand Prompt 注入（v0.1.2）
- archetype YAML 整份注入 draft prompt
- 完整 style-guide.md 注入 draft + validate 双 prompt
- 3 个 archetype 对应 samples 注入 draft prompt
- 相邻章节（prev/next）Title+Abstract+KeyConcepts 注入 user_draft

### ✅ 全局 fallback（v0.1.4）
- `ModelRef` 内嵌 `Fallback *ModelRef`；`buildChatter` 静态装配 `FallbackWrapper`
- primary==fallback 时跳过（打 warn 日志）

### ✅ LLM 超时 + Ctrl+C（v0.1.5）
- `LLMConfig.TimeoutSeconds`（全局默认 90s）+ `ModelRef.TimeoutSeconds` 阶段覆盖（expand 600s）
- `stageCtx()` 每个阶段自动带超时 + `signal.NotifyContext` Ctrl+C 立即取消

### ✅ Streaming 输出（v0.1.6）
- `llm.Streamer` 接口；Gemini genai SDK + GLM SSE + Ollama stream + Mock 实现
- 仅 draft 阶段流式（`-L` 逐 token stdout）

### ✅ 自动事实复核（v0.2.0）
- `jianwu factcheck <slug> <NN-MM>` — 逐 claim 读取 source URL，LLM 验证
- `jianwu revise <slug> <NN-MM>` — 基于 unverified claims 让 LLM 修订章节
- ClaimWhitelist 跨章复用已验证声明（metal.json `claim_whitelist`）

### ✅ 多导出目标（v0.2.0 后）
- `export --target md`（单文件 markdown，默认）
- `export --target hugo`（chapter-per-file Hugo content structure）
- `export --target pdf`（pandoc auto-generation，需系统安装 pandoc）

### ✅ Storage 接口（v0.3.0 地基）
- `internal/storage/storage.go` — `Storage` 接口（ReadFile/WriteFile/MkdirAll/RemoveAll/Rename/Stat/ReadDir）
- `storage.OS` — 默认文件系统实现
- `storage.MemStorage` — 内存实现用于测试
- book/workspace/config/cli/grill 已迁移到 `storage.OS` 调用

### ✅ Embedding 索引缓存（v0.2.3）

- `corpus.BuildIndex(ctx, embedder, model, books)` — 为所有语料书生成 embedding 向量
- `corpus.SaveIndex(path, idx)` / `corpus.LoadIndex(path)` — 索引文件 I/O
- `corpus.CorpusIndex.FindSimilar(slug, topN)` — 余弦相似度搜索
- `corpus corpus reindex` 命令：加载 embedder，调用 BuildIndex，保存到 `.jianwu/corpus_index.json`
- expand `ToolRegistry.LookupSimilarBook(slug, topN)` — 懒加载缓存索引，避免实时调用 embedder
- `ToolRegistry.SetCorpusIndexPath(path)` / `CorpusIndexPathForWorkspace` — CLI 层在 buildToolRegistry 时自动配置

### ✅ 后 3 个原型 + 新语料（v0.2.3 后）

- **3 个新原型 YAML** — `micro-meso-macro`（宏-中-微）、`theory-dynamics-history-present`（理论-动力-历史-当下）、`mindset-method-practice`（心法-方法-实践）
- **3 个新 few-shot samples** — 各原型对应
- **4 本新增语料 JSON** — `barbaric-order`、`data-as-the-boundary`、`open-map`、`revisiting-history`
- 结合原有 6 本语料，已覆盖 zhurongshuo 主要训练逻辑主题

### ✅ Corpus Sync（v0.2.3）

- `jianwu corpus list` — 列出所有语料书（含 workspace 覆盖标记）
- `jianwu corpus show <slug>` — 显示语料书详细信息（title、parts、chapters）
- `jianwu corpus stats` — 统计总书数/部数/章节数/archetype 分布
- `jianwu corpus sync --from <path>` — 从 zhurongshuo checkout 目录同步 JSON 到 `.jianwu/corpus/`，含 slug/title 校验
- `jianwu corpus reindex` — 调用 embedder 生成 embedding 索引并缓存到 `.jianwu/corpus_index.json`
- `corpus.LoadWithWorkspace(wsRoot)` — 分层加载：workspace 覆盖层 + builtin 回退

### ✅ Ollama 本地模型支持（v0.2.1）
- `provider/llm/ollama/` — Chatter + Embedder + Streamer
- 默认 `http://localhost:11434`，配置 `provider: ollama` 即可使用
- 支持 Qwen3、DeepSeek 等本地模型

## 10. 待做（v0.2 → v0.3 → v1.0）

### v0.2（功能扩展 — ✅ 已全部交付）

- [x] 章节迭代命令（`rewrite` / `add-chapter` / `move-chapter` / `delete-chapter` / `expand --all`）
- [x] `corpus sync` 扩展语料（重新从 zhurongshuo 拉取）
- [x] Embedding 索引文件缓存（v0.1 是实时计算）
- [x] 后 3 个原型（micro-meso-macro / theory-dynamics-history-present / mindset-method-practice）+ 4 本新语料

### v0.3（SaaS-ready 内核改造 — ✅ 已全部交付）

> jianwu 原假设"单用户 + 本地"：全局单文件 secrets、部分 `os.*` 调用、无进度回调、无 token 计量。
> v0.3.0–0.3.5 补全了这层内核能力，使 jianwu 可被多租户 Web 安全嵌入，**不含任何 web UI**。

> **已交付：** `Storage` 接口（v0.3.0 地基）已在 `internal/storage/` 实现，book/workspace/config/cli/grill 已迁移。

| 切片 | 内容 | 前置 | 状态 |
|---|---|---|---|
| v0.3.0 | 存储抽象 `Storage` 接口 | 地基 | ✅ 已交付 |
| v0.3.1 | 长任务 / 进度模型（expand 回调 + 可取消） | v0.1.5 | ✅ 已交付 |
| v0.3.2 | Token / 成本计量 | — | ✅ 已交付 |
| v0.3.3 | per-tenant Secrets | — | ✅ 已交付 |
| v0.3.4 | 并发安全 provider 装配 | — | ✅ 已交付（显式参数注入，`go test -race` 全绿） |
| v0.3.5 | SaaS 安全加固（SSRF allowlist / LimitReader / 错误截断） | — | ✅ 已交付 |

**v0.3 SaaS-ready 内核已全部交付。**

### v1.0（mouqin SaaS）

- [ ] mouqin web app（前后端）
- [ ] 多用户 / 鉴权 / 账单
- [ ] 公开 book 分享链接
- [ ] 在线 grill-me（web 版交互）
- [ ] 协作（评论、共享 workspace）

## 11. 已知技术债

### 架构层
- ~~`cli.chatterProviderHook` + `cli.providerDepsHook` 全局可变 var~~ — v0.3.4 已重构：`runExpand`/`runNewFlow` 用显式参数注入，测试直接构造 mock
- 三个 factory 包（`llmfactory` / `searchfactory` / `readerfactory`）独立存在只为打破 import cycle——是 Go 标准做法但显得啰嗦

### 代码层（已清理）
- ~~`book.Citation.UsedInParagraph` 字段从未填充~~ — 已删除
- ~~`expand.ExpandOutput.Draft` 字段保留 pre-validation draft~~ — 已删除
- ~~`cli.new.go` 的 `_ = session`~~ — 已重构
- ~~`expand.similar_book_tool.go` `LookupSimilarBook` 从未被主流程调用~~ — 已删除
- ~~`provider/llm/interface.go` `chatterEmbedder` 废弃类型~~ — 已删除

### 安全（v0.3.5 已加固）
- ~~Search/Reader 的 BaseURL 配置无 allowlist~~ — `reader.ValidateURL()` 已添加（仅允许 http/https，禁止 localhost/私有 IP）
- ~~Jina 的 `io.ReadAll` 无大小限制~~ — 已用 `LimitReader`（10MB body + 4KB error body）
- ~~Search/Reader 错误消息含完整 response body~~ — Brave + Serper 已截断（4KB cap）
- ~~Citation 中的 URL 无 SSRF 校验~~ — `reader.ValidateURL()` 集成到 Jina reader（v0.3.5）

## 12. 关键设计决策（grill-me 26 项）

完整决策记录见 `docs/decisions/26-grill-decisions.md`。摘要：

- **库优先**：所有引擎逻辑在 Go 库，CLI/Web 是薄包装
- **多厂商 day-1 做实**：Gemini + GLM + Ollama，不留"预留接口"
- **批次分层锚定**：scaffolding 用参数化知识（快），expand 默认开 web search（可信）
- **强制引用 + 人工签发**：事实性陈述必须有 source；status workflow 兜底质量
- **独立于 zhurongshuo**：内置最小语料 + sync 扩展分层
- **显式 workspace**：一个 workspace = 一个 git 仓库 = 一个逻辑集合

## 13. 开发工作流

参考已归档的切片计划（`docs/archive/plans/`）了解 SDD（subagent-driven development）模式：

1. **/grill-me** 决策对齐（先决策后代码）→ 产出 `docs/decisions/`
2. **writing-plans** 出 task-by-task 计划 → `docs/plans/`
3. **subagent-driven-development** 每个 task 派 fresh implementer subagent + task reviewer
4. 切片完成 → opus 最终 whole-branch review → 一个修复 commit 解决所有 findings
5. 计划文件归档到 `docs/archive/plans/`（ship 后）

新切片应该按这个模式继续。

## 14. 测试策略

- **库代码**（数据解析、状态机、纯逻辑）：TDD（test-first）
- **LLM-driven 代码**：test-after，Mock Provider + httptest
- **E2E**：每命令 1 个 happy path（用 `chatterProviderHook` / `providerDepsHook` 注入 mock）
- **Live integration**：每阶段都有，无 API key 时 SKIP

跑全部测试：
```bash
go test ./...                    # 全部
go test -run TestE2E ./internal/cli/...  # 仅 E2E
GEMINI_API_KEY=xxx go test ./internal/engine/expand/... -run TestGenerateLive  # 真 LLM
```

## 15. 快速上手

```bash
# 安装
go install github.com/iannil/jianwu/cmd/jianwu@latest

# 初始化 workspace
jianwu init my-library
cd my-library

# 配置 API keys（至少配一个）
export GEMINI_API_KEY=...
export GLM_API_KEY=...
# 或使用本地模型（无需 API key）
# jianwu config set intake.provider ollama

# 完整跑一遍
jianwu new                    # grill → outline → scaffolding
jianwu expand <slug> 01-01    # 展开第 1 章
jianwu factcheck <slug> 01-01 # 事实复核（可选）
jianwu revise <slug> 01-01    # 修订（可选）
jianwu review  <slug> 01-01   # 标记审阅
jianwu finalize <slug>        # 定稿
jianwu export <slug>          # 导出
```

## 16. 文档索引

| 文档 | 用途 |
|---|---|
| `README.md` | 用户视角介绍 + 安装 + 快速上手 |
| `docs/PROJECT_STATUS.md`（本文档） | LLM 友好的当前状态全景 |
| `docs/architecture/overview.md` | 架构图 + 数据流 + 关键接口 |
| `docs/decisions/26-grill-decisions.md` | 26 项核心决策 + v0.1.x 审计决策 |
| `docs/ROADMAP.md` | v0.1.x → v1.0 路线图 |
| `docs/EXTRACTION_NOTES.md` | zhurongshuo 资产萃取记录 |
| `docs/plans/` | 当前切片的 SDD plan（为空表示无进行中切片） |
| `docs/archive/plans/` | 已完成切片的 SDD plan（S1-S7 + v0.1.1–v0.1.6 + v0.2.0） |
| `docs/archive/DESIGN.md` | 原始设计文档（v0.1 锁定版，部分内容已过期） |
| `AGENTS.md` | 给 AI agent 的项目记忆（构建/测试/架构/约定速查） |
| `LICENSE` | AGPL-3.0 |

## 17. 里程碑回顾

### v0.1.1 Expand CLI（2026-06-23）

expand 引擎从库 API 接到 CLI，加 `jianwu expand <slug> <NN-MM>` 命令。8 个 task 走完 SDD。

### v0.1.2 Expand Prompt 注入（2026-06-23）

把 archetype + style guide + samples + 相邻章节真正注入 draft/validate prompt，使产出贴合 zhurongshuo 风格。

### v0.1.3 状态机命令（2026-06-23）

review / finalize / export / status 四命令，纯状态/文件操作（不调 LLM）。
outline.json 为状态真相源；export 初始仅 `--target md`。

### v0.1.4 Fallback Model Wiring（2026-06-26）

`ModelRef` 内嵌 `Fallback *ModelRef`；`buildChatter` 静态装配 `FallbackWrapper`。
配置模板含 fallback 示例。单元测试 + E2E mock 测试。

### v0.1.5 LLM Timeout + Grill Hardening（2026-06-26）

全局默认 90s 超时 + expand 600s 覆盖。Ctrl+C 通过 `signal.NotifyContext` 立即取消。
Grill 模块 4 个低风险修复（重复 walk 逻辑、Context 检查等）。

### v0.1.6 Streaming Output（2026-06-26）

`llm.Streamer` 接口（`Stream(ctx, req) <-chan StreamChunk`），Gemini+GLM+Mock 实现。
`-L` 模式逐 token stdout。仅 draft 阶段流式。

### v0.2.0 事实复核 + 修订（2026-06-26 后）

- `factcheck` 引擎：逐 claim + Reader.Read(url) + LLM 验证，输出 ClaimVerdict[]
- `revise` 引擎：基于 unverified claims + SuggestedRewrite，让 LLM 修订章节
- ClaimWhitelist 跨章复用已验证声明
- 伴随 v0.2.1：Ollama 本地模型支持 + hugo/pdf export targets + Storage 接口地基

### v0.2.2 章节迭代命令（2026-06-28）

- `rewrite` / `add-chapter` / `move-chapter` / `delete-chapter` / `expand --all` 五个命令
- 全书的章节生命周期管理（插入、删除、移动、重写、批量展开）

### v0.2.3 Corpus Sync + Embedding 索引（2026-06-28）

- `corpus list/show/stats/sync/reindex` 五个子命令
- `corpus.BuildIndex` + `CorpusIndex.FindSimilar` 实现 embedding 缓存
- Workspace 覆盖层 + builtin 回退分层加载

### 后 3 个原型交付（v0.2.3 后）

- 3 个新原型 YAML + 3 个新 few-shot samples + 4 本新语料 JSON
- 至此 jianwu 共 6 个原型、10 本语料、6 个样例

### v0.3.0–0.3.5 SaaS-ready 内核全线交付（2026-06-28）

- 存储抽象（v0.3.0）、长任务进度模型（v0.3.1）、Token/成本计量（v0.3.2）
- per-tenant Secrets（v0.3.3）、并发安全 provider 装配（v0.3.4）
- SaaS 安全加固：SSRF allowlist + LimitReader + 错误截断（v0.3.5）
