# jianwu 项目状态

> 本文档对 LLM 友好——任何接手后续迭代的 agent 读这一份就能理解项目当前形态、什么能用、什么没做、怎么扩展。
> 最后更新：2026-06-26（v0.1.3 已 ship，完整闭环可用）

---

## TL;DR

jianwu 是一个把 LLM 训练知识结构化为人类可读图书的 Go 库 + CLI。

- **当前版本**：**v0.1.3**（2026-06-23 ship） — 完整 CLI 闭环：`new → expand → review → finalize → export`
- **可用 CLI 命令**：`init` / `info` / `config get·set·list` / `new` / `expand <slug> <NN-MM>` / `review` / `finalize` / `export` / `status`
- **库 API**：4 阶段引擎 `grill → outline → scaffolding → expand` 独立可调
- **质量基线**：24 个包测试全绿，`go vet` / `gofmt` 全清
- **下一里程碑**：**v0.1.4 Fallback Model Wiring** — 主模型失败时自动切备用模型

---

## 1. 项目目标

> **肩吾** —— 把 AI 的训练知识结构化为人类可阅读、可学习的图书。

用户给出主题，AI 走完 grill 问诊 → outline 生成 → scaffolding 脚手架 → expand 成稿（带 web search 引用）的全流程，
产出"配得上 zhurongshuo 同书架"的中文非虚构图书。完整状态机：`expanded → reviewed → final → export`。

## 2. 两个仓库

| 仓库 | 角色 | 状态 |
|---|---|---|
| `jianwu` | 核心引擎（库 + CLI） | **v0.1.3 shipped**（完整闭环） |
| `mouqin` | Web SaaS（包装 jianwu） | v1.0 未启动，前置 v0.3 SaaS-ready 内核 |

库优先：所有核心逻辑在 `jianwu` 库里，CLI 和未来的 Web 都包同一个库。

## 3. 与 zhurongshuo 的关系

**完全独立。** zhurongshuo 是方法论参考 + 参考语料来源，**不是部署目标**。

- 一次性萃取：archetype YAML + few-shot 样例 + corpus outline 摘要 → 进 jianwu 二进制（`//go:embed`）
- 运行时零外部依赖：zhurongshuo 仓库消失，jianwu 仍能正常运行
- v0.2+ 的 `corpus sync` 才会重新读 zhurongshuo

## 4. 当前架构

```
cmd/jianwu/main.go                    # CLI 入口（exit code mapping）
internal/
  cli/                                # cobra 命令层
    root / init / info / config / new # v0.1 命令
    expand / review / finalize        # v0.1.1-3 新增命令
    export / status                   # v0.1.3 新增命令
    prompt                            # TerminalPrompt（grill.UserInput 实现）
    providers / new_flow              # 编排 + provider 装配
    book_resolve                      # loadBook + mirrorChapterStatus 共享助手
    footnotes                         # 全局脚注重编号（export 用）
  workspace/                          # .jianwu/ 加载、walk-up detect、Init/Load
  config/                             # 5 层 config resolver + secrets (ENV > file, 0600)
  book/                               # Meta/Outline/Chapter/Citation 类型 + JSON I/O + Slugify
  archetypes/ style/ corpus/          # 内置数据（embed FS）
  provider/
    llm/                              # Chatter + Embedder 接口 + Retry/Fallback wrappers
      gemini/ glm/ mock/              # 三家实现
    search/                           # Searcher + Brave/Serper
    reader/                           # Reader + Jina
    llmfactory/ searchfactory/ readerfactory/  # 工厂（独立包避免 import cycle）
  engine/
    outline/                          # 单次 LLM 调用产出 book.Outline
    scaffolding/                      # N 章并行 + RetryFailed
    grill/                            # 12 维度 design tree + stateful session
    expand/                           # 3-iteration agent + 工具调用 + citation 解析
```

**包依赖图（无环）：** `cli → engine → provider → book/config/workspace`；
`{llm,search,reader}factory` 横跨 provider 子树以打破循环。

## 5. v0.1 4 阶段引擎

| 阶段 | 包 | 模式 | 默认模型 | 状态 |
|---|---|---|---|---|
| Grill | `engine/grill` | stateful interactive | GLM-4.6 | ✅ 完成 |
| Outline | `engine/outline` | 单次 batch | Gemini 2.5 Pro | ✅ 完成 |
| Scaffolding | `engine/scaffolding` | N 章并行 | Gemini 2.5 Flash | ✅ 完成 |
| Expand | `engine/expand` | 3-iteration agent | GLM-4.6 | ✅ 完成（含 prompt 注入） |

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
LLM 写 markdown + [^N] footnotes
  ↓ iter 3: RunValidate
LLM 自检 + 修订（注入 style guide），输出 claims[].has_citation
  ↓
ParseFootnotes + mergeCitations(registry metadata)
  → ExpandOutput{Markdown, Citations[], UnverifiedClaims[], WordCount}
```

**状态机命令（v0.1.3，纯状态/文件操作，不调 LLM）：**
```
review <slug> <NN-MM>   : expanded → reviewed
finalize <slug>         : 全书 reviewed → final（Meta.Status="final"）
export <slug>           : 合并 chapters → 单 md，全局脚注重编号
status <slug>           : 显示各章状态 + 下一步提示
```

## 6. Provider 抽象

```go
// internal/provider/llm/interface.go
type Chatter interface { Chat(ctx, ChatRequest) (*ChatResponse, error) }
type Embedder interface { Embed(ctx, EmbedRequest) (*EmbedResponse, error) }
// Streamer / Tooler 接口预留（S5/S7 followup）
```

| Provider | 实现 | 用途 |
|---|---|---|
| Gemini | `provider/llm/gemini`（官方 `google.golang.org/genai` SDK） | outline + scaffolding |
| GLM | `provider/llm/glm`（直 REST，OpenAI-compatible 复用） | intake + expand |
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
- Retry 耗尽 → fallback provider；fallback 也失败 → 返回最后一次错误

## 7. 数据模型

### Workspace（`.jianwu/`）
```
<jianwu-workspace>/
  .jianwu/
    config.yaml              # workspace 配置（覆盖全局）
    schema_version           # 内容 = "1"
    sessions/<id>.json       # 运行中的 grill 会话
  books/<slug>/
    meta.json                # Meta（id/slug/title/archetype/parameters/...）
    outline.json             # Outline（parts[].chapters[]）+ citations[] + status
    .session.json            # 已完成的 grill 会话（audit log）
    chapters/NN-MM.md        # expand 产出（YAML frontmatter + markdown + [^N] footnotes）
    export/<slug>.md         # export 命令产出（全书合并，脚注重编号）
  exports/                   # 导出产物（v0.2+）
  archive/                   # 归档旧版 book（v0.2+）
```

完整 schema 见 `internal/book/types.go`。

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
| `expand <slug> <NN-MM> [--force]` | **v0.1.1** | 展开单个章节（research→draft→validate） |
| `review <slug> <NN-MM>` | **v0.1.3** | 标记章节为已审阅 |
| `finalize <slug> [--dry-run]` | **v0.1.3** | 全书审阅完成后定稿 |
| `export <slug> [--dry-run]` | **v0.1.3** | 合并章节导出为单文件 markdown |
| `status <slug>` | **v0.1.3** | 章节进度概览 + 下一步提示 |

### ✅ 引擎库 API
- `outline.Generate(ctx, chatter, Input) (*book.Outline, error)`
- `scaffolding.GenerateChapter(ctx, chatter, ChapterInput) (*ChapterOutput, error)`
- `scaffolding.ScaffoldAll(...) map[string]Result`
- `scaffolding.RetryFailed(...)`
- `grill.NewSession()` / `grill.Run(...)` / `grill.Repository{Save,Load,ListIncomplete,Archive}`
- `expand.Generate(ctx, chatter, tools, ExpandInput) (*ExpandOutput, error)`

### ✅ Provider 库
- LLM: Gemini（含 context cache helper）+ GLM（OpenAI-compatible HTTP client）+ Mock
- Search: Brave + Serper
- Reader: Jina
- Retry + Fallback wrappers（ctx-aware）
- 5 个工厂函数（`llmfactory` / `searchfactory` / `readerfactory`）

### ✅ 数据资产（embed）
- 3 个 archetype YAML（本体-认识-实践 / 诊断-解码-破局 / 基础-应用-实战）
- 1 个 style-guide.md + 3 个 few-shot samples
- 6 本 builtin corpus JSON

### ✅ Expand Prompt 注入（v0.1.2）
- archetype YAML 整份注入 draft prompt
- 完整 style-guide.md 注入 draft + validate 双 prompt
- 3 个 archetype 对应 samples 注入 draft prompt
- 相邻章节（prev/next）Title+Abstract+KeyConcepts 注入 user_draft

## 10. 待做（v0.1.x → v0.2 → v0.3 → v1.0）

### v0.1.x（让 v0.1 真正名副其实）

> v0.1.0 tag 时实际范围是库 API + new CLI，未含 expand CLI 与风格注入。
> v0.1.x 系列补齐承诺。**v0.1.5 ship 后视为 v0.1 真正交付。**

- [x] `jianwu expand <slug> <NN-MM>` CLI 命令（**v0.1.1**，已交付 2026-06-23）
- [x] Expand prompt 注入 archetype + samples + guide + adjacent（**v0.1.2**，已交付）
- [x] `jianwu review / finalize / export / status` 状态机命令（**v0.1.3**，已交付）
- [ ] Fallback model wiring（**v0.1.4**，当前迭代）
- [ ] LLM 调用超时（**v0.1.5**）
- [ ] Streaming 输出（**v0.1.6**，可选 polish）

### v0.2（功能扩展）

- [ ] 章节迭代命令（rewrite / add-chapter / move-chapter / delete-chapter / expand --all）
- [ ] `corpus sync` 扩展语料（重新从 zhurongshuo 拉取）
- [ ] Embedding 索引文件（v0.1 是实时计算，v0.2 加 cache）
- [ ] 自动事实复核（claims 抽取 + 验证 agent）
- [ ] Workspace migration（schema v1 → v2）
- [ ] 多 export target（zhurongshuo / hugo / pdf）
- [ ] 后 3 个原型（micro-meso-macro / theory-dynamics-history-present / mindset-method-practice）

### v0.3（SaaS-ready 内核改造，mouqin 前置）

> jianwu 当前全程假设"单用户 + 本地"——12 处直接 `os.*` 文件调用、secrets 全局单文件、
> provider 装配靠全局可变 var。v0.3 补这层内核能力，**不含任何 web UI**。

| 切片 | 内容 | 前置 |
|---|---|---|
| v0.3.0 | 存储抽象 `Storage` 接口 | 地基 |
| v0.3.1 | 长任务 / 进度模型（expand 回调 + 可取消） | v0.1.5 |
| v0.3.2 | Token / 成本计量 | — |
| v0.3.3 | per-tenant Secrets | — |
| v0.3.4 | 并发安全 provider 装配（吸收 v0.2.6） | — |
| v0.3.5 | SaaS 安全加固（SSRF allowlist / LimitReader / 错误截断） | — |

### v1.0（mouqin SaaS）

- [ ] mouqin web app（前后端）
- [ ] 多用户 / 鉴权 / 账单
- [ ] 公开 book 分享链接
- [ ] 在线 grill-me（web 版交互）
- [ ] 协作（评论、共享 workspace）

## 11. 已知技术债

### 架构层
- `cli.chatterProviderHook` + `cli.providerDepsHook` 是 test-only 全局可变 var（注释已警告），v0.3.4 重构为显式注入
- 三个 factory 包（`llmfactory` / `searchfactory` / `readerfactory`）独立存在只为打破 import cycle——是 Go 标准做法但显得啰嗦

### 代码层（已清理）
- ~~`book.Citation.UsedInParagraph` 字段从未填充~~ — 已删除
- ~~`expand.ExpandOutput.Draft` 字段保留 pre-validation draft~~ — 已删除
- ~~`cli.new.go` 的 `_ = session`~~ — 已重构
- ~~`expand.similar_book_tool.go` `LookupSimilarBook` 从未被主流程调用~~ — 已删除（v0.1.2 切出独立切片，代码债清理）
- ~~`provider/llm/interface.go` `chatterEmbedder` 废弃类型~~ — 已删除（用 `ChatterEmbedder` 替代）

### 安全（v0.1 可接受，v1.0 SaaS 必修）
- Search/Reader 的 BaseURL 配置无 allowlist（v1.0 SaaS 需要）
- Jina 的 `io.ReadAll` 无大小限制（v1.0 加 LimitReader）
- Search/Reader 错误消息含完整 response body（v1.0 截断）
- Citation 中的 URL 无 SSRF 校验（Jina 服务端 fetch，客户端不直连）

## 12. 关键设计决策（grill-me 26 项）

完整决策记录见 `docs/decisions/26-grill-decisions.md`。摘要：

- **库优先**：所有引擎逻辑在 Go 库，CLI/Web 是薄包装
- **多厂商 day-1 做实**：Gemini + GLM，不留"预留接口"
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

# 配置 API keys
export GEMINI_API_KEY=...
export GLM_API_KEY=...

# 完整跑一遍（grill → outline → scaffolding → expand → review → finalize → export）
jianwu new
jianwu expand <slug> 01-01
jianwu review  <slug> 01-01
jianwu finalize <slug>
jianwu export   <slug> --dry-run  # 预览
jianwu export   <slug>            # 产出 books/<slug>/export/<slug>.md
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
| `docs/archive/plans/` | 已完成切片的 SDD plan（S1-S7 + v0.1.1–v0.1.3） |
| `docs/archive/DESIGN.md` | 原始设计文档（v0.1 锁定版，部分内容已过期） |
| `AGENTS.md` | 给 AI agent 的项目记忆（构建/测试/架构/约定速查） |
| `LICENSE` | AGPL-3.0 |

## 17. 里程碑回顾

### v0.1.1 Expand CLI（2026-06-23）

expand 引擎从库 API 接到 CLI，加 `jianwu expand <slug> <NN-MM>` 命令。8 个 task 走完 SDD。

### v0.1.2 Expand Prompt 注入（2026-06-23）

把 archetype + style guide + samples + 相邻章节真正注入 draft/validate prompt，使产出贴合 zhurongshuo 风格。similar-book 切出独立切片（v0.2.1）。

### v0.1.3 状态机命令（2026-06-23）

review / finalize / export --target md / status 四命令，纯状态/文件操作（不调 LLM）。
outline.json 为状态真相源、镜像同步章节 .md frontmatter；严格状态机 expanded→reviewed→final。
export 全局重编号脚注、缺正文章节占位提示。

### 下一个切片：v0.1.4 Fallback Model Wiring

**目标：** 配置中指定备用模型，主模型失败时自动接管。全局单一 fallback（决策 Q10=A）。

**任务：**
- `config.ModelRef` 加 `Fallback *ModelRef` 字段
- `cli.buildChatter` 检测 Fallback，非空则 wrap with `FallbackWrapper`
- fallback provider == primary provider 时打 warning + 不装 wrapper
- 配置示例更新到 workspace 默认模板
- E2E test：primary 失败 → fallback 接管

**验收：** 配 primary=gemini-2.5-pro + fallback=glm-4.6，断网 Gemini 时自动切到 GLM。
