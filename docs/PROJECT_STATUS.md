# jianwu 项目状态

> 本文档对 LLM 友好——任何接手后续迭代的 agent 读这一份就能理解项目当前形态、什么能用、什么没做、怎么扩展。
> 最后更新：2026-06-22（v1.0.0 已 ship）

---

## TL;DR

jianwu 是一个把 LLM 训练知识结构化为人类可读图书的 Go 库 + CLI。v1.0.0 已交付完整的 4 阶段引擎 + `jianwu new` 命令闭环。

- **当前版本**：v1.0.0（tag 在 master）
- **可用入口**：`jianwu init` / `info` / `config get/set/list` / `new`（完整 grill → outline → scaffolding 闭环）
- **库 API**：`internal/engine/{outline,scaffolding,grill,expand}` 4 个引擎阶段独立可调
- **CLI 缺口**：`expand` / `review` / `finalize` / `export` 命令（v1.0.x 补）
- **质量基线**：21 个包测试全绿，`go vet` / `gofmt -l` 全清

---

## 1. 项目目标

> **简物（jiàn wù）**—— 把 AI 的训练知识结构化为人类可阅读、可学习的图书。

用户给出主题，AI 走完 grill 问诊 → outline 生成 → scaffolding 脚手架 → expand 成稿（带 web search 引用）的全流程，产出"配得上 zhurongshuo 同书架"的中文非虚构图书。

## 2. 两个仓库

| 仓库 | 角色 | 状态 |
|---|---|---|
| `jianwu` | 核心引擎（库 + CLI） | v1.0.0 shipped |
| `mouqin` | Web SaaS（包装 jianwu） | v2 未启动 |

库优先：所有核心逻辑在 `jianwu` 库里，CLI 和未来的 Web 都包同一个库。

## 3. 与 zhurongshuo 的关系

**完全独立。** zhurongshuo 是方法论参考 + 参考语料来源，**不是部署目标**。

- 一次性萃取：archetype YAML + few-shot 样例 + corpus outline 摘要 → 进 jianwu 二进制（`//go:embed`）
- 运行时零外部依赖：zhurongshuo 仓库消失，jianwu 仍能正常运行
- v1.1+ 的 `corpus sync` 才会重新读 zhurongshuo

## 4. 当前架构

```
cmd/jianwu/main.go                    # CLI 入口（exit code mapping）
internal/
  cli/                                # cobra 命令层
    root / init / info / config / new # v1.0 命令
    prompt                            # TerminalPrompt（grill.UserInput 实现）
    providers / new_flow              # 编排 + provider 装配
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

**包依赖图（无环）：** `cli → engine → provider → book/config/workspace`；`{llm,search,reader}factory` 横跨 provider 子树以打破循环。

## 5. v1.0 4 阶段引擎

| 阶段 | 包 | 模式 | 默认模型 | 状态 |
|---|---|---|---|---|
| Grill | `engine/grill` | stateful interactive | GLM-4.6 | ✅ 完成 |
| Outline | `engine/outline` | 单次 batch | Gemini 2.5 Pro | ✅ 完成 |
| Scaffolding | `engine/scaffolding` | N 章并行 | Gemini 2.5 Flash | ✅ 完成 |
| Expand | `engine/expand` | 3-iteration agent | GLM-4.6 | ✅ 引擎完成，CLI 待补 |

**关键流程（v1.0）：**
```
jianwu new
  ↓
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

**Expand（库 API，v1.0.x 补 CLI）：**
```
expand.Generate(ctx, chatter, tools, input)
  ↓ iter 1: RunResearch
web_search × N + read_url × M  →  LLM 提取 research notes + citation candidates
  ↓ iter 2: RunDraft
LLM 写 markdown + [^N] footnotes
  ↓ iter 3: RunValidate
LLM 自检 + 修订，输出 claims[].has_citation
  ↓
ParseFootnotes + mergeCitations(registry metadata)
  → ExpandOutput{Markdown, Citations[], UnverifiedClaims[], WordCount}
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

**Retry/Fallback（Q7）：**
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
    outline.json             # Outline（parts[].chapters[]）
    .session.json            # 已完成的 grill 会话（audit log）
    chapters/NN-MM.md        # expand 后产出（v1.0.x）
  exports/                   # 导出产物（v1.1+）
  archive/                   # 归档旧版 book（v1.1+）
```

### Book 文件
- `meta.json`：书籍元数据（slug, archetype, parameters, engine versions）
- `outline.json`：parts[] × chapters[]，每章带 status (scaffolded/expanded/reviewed/final/failed) + abstract/key_concepts/learning_objectives/suggested_examples + citations[]
- `chapters/NN-MM.md`（v1.0.x expand 后）：markdown + frontmatter + `[^N]` footnotes

完整 schema 见 `internal/book/types.go`。

## 8. 配置系统

**5 层（高 → 低优先级）：**
1. CLI flag（`--model glm-4.6`）
2. 环境变量（`JIANWU_OUTLINE_MODEL=...`）
3. Workspace `.jianwu/config.yaml`
4. 全局 `~/.config/jianwu/config.yaml`
5. 编译时 defaults（`internal/config/defaults.go`）

**Secrets：** `~/.config/jianwu/secrets.yaml`（0600 权限强制）或 ENV（`GEMINI_API_KEY` / `GLM_API_KEY` / `BRAVE_API_KEY` / `SERPER_API_KEY` / `JINA_API_KEY`）。ENV > file 字段级覆盖。

## 9. 已交付（v1.0.0）

### ✅ CLI 命令
- `jianwu init [--bare] [path]`
- `jianwu info`
- `jianwu config get/set/list`
- `jianwu new [--force]`

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
- 6 本 builtin corpus JSON（reality-construction / advancement-of-reality / silent-games / forced-convergence / ai-engineer-in-action / intelligent-computing-center-construction-guide）

## 10. 待做（v1.0.x → v1.1）

### v1.0.x（让 v1.0 真正好用）
- [ ] `jianwu expand <slug> <NN-MM>` CLI 命令（把 expand 引擎接到 CLI）
- [ ] `jianwu review <slug> <NN-MM>` / `jianwu finalize <slug>` / `jianwu export <slug> --target md`
- [ ] Fallback model wiring（Config.Models[stage].Fallback 字段 + FallbackWrapper 装配）
- [ ] Streaming 输出（grill + expand 长时间运行时 token 流式）
- [ ] LLM 调用超时（避免 hang）
- [ ] Expand 注入 archetype YAML + style samples 到 prompt（当前用占位符）

### v1.1（功能扩展）
- [ ] 章节迭代命令（`rewrite` / `add-chapter` / `move-chapter`）
- [ ] `corpus sync` 扩展语料（重新从 zhurongshuo 拉取）
- [ ] Embedding 索引文件（v1.0 是实时计算，v1.1 加 cache）
- [ ] 自动事实复核（claims 抽取 + 验证 agent）
- [ ] Workspace migration（schema v1 → v2）
- [ ] 多 export target（zhurongshuo / hugo / pdf）
- [ ] 后 3 个原型（micro-meso-macro / theory-dynamics-history-present / mindset-method-practice）

### v2（mouqin SaaS）
- [ ] mouqin web app（前后端）
- [ ] 多用户 / 鉴权 / 账单
- [ ] 公开 book 分享链接
- [ ] 在线 grill-me（web 版交互）
- [ ] 协作（评论、共享 workspace）

## 11. 已知技术债

### 架构层
- `cli.chatterProviderHook` 是 test-only 全局可变 var（注释已警告），未来重构为 struct field 更干净
- 三个 factory 包（`llmfactory` / `searchfactory` / `readerfactory`）独立存在只为打破 import cycle——是 Go 标准做法但显得啰嗦
- `expand.types.ExpandOutput.Draft` 字段保留 pre-validation draft 用于 debug，正常路径不用

### 代码层（minor，可清理）
- `book.Citation.UsedInParagraph` + `expand.Citation.UsedInParagraph` 字段从未填充（保留 schema 兼容）
- `expand.ResearchPlan` struct 从未使用（iter 1 直接走 ResearchNotes）
- `expand.citation.go` 的 `inChinese` 局部变量设置后只在空 `if` body 中读
- `expand.NewToolRegistryFromProviders` 是 `NewToolRegistry` 的简单 alias（无附加值）
- `cli.new.go` 的 `_ = session` 是预期行为（CLI 摘要不用 session），注释已说明

### 安全（v1.0 可接受，v2 SaaS 必修）
- Search/Reader 的 BaseURL 配置无 allowlist（v2 SaaS 需要）
- Jina 的 `io.ReadAll` 无大小限制（v2 加 LimitReader）
- Search/Reader 错误消息含完整 response body（v2 截断）
- Citation 中的 URL 无 SSRF 校验（Jina 服务端 fetch，我们的客户端不直连）

### 文档
- `DESIGN.md` §11 状态行还写"v1.0 设计已锁定，进入实施阶段"（应改为"v1.0.0 已交付"）
- `EXTRACTION_NOTES.md` 还标"待审阅"（资产实际已被 7 个切片的 review 验证）

## 12. 关键设计决策（grill-me 26 项）

完整决策记录见 `docs/decisions/26-grill-decisions.md`。摘要：

- **库优先**：所有引擎逻辑在 Go 库，CLI/Web 是薄包装
- **多厂商 day-1 做实**：Gemini + GLM，不留"预留接口"
- **批次分层锚定**：scaffolding 用参数化知识（快），expand 默认开 web search（可信）
- **强制引用 + 人工签发**：事实性陈述必须有 source；status workflow 兜底质量
- **独立于 zhurongshuo**：内置最小语料 + sync 扩展分层
- **显式 workspace**：一个 workspace = 一个 git 仓库 = 一个逻辑集合

## 13. 开发工作流（v1.0.x+）

参考已归档的 7 个切片计划（`docs/archive/plans/`）了解 SDD（subagent-driven development）模式：

1. **/grill-me** 26 个维度决策对齐
2. **writing-plans** 出 task-by-task 计划（每个 task TDD：RED → GREEN → commit）
3. **subagent-driven-development** 每个 task 派 fresh implementer subagent + task reviewer
4. 切片完成 → opus 最终 whole-branch review → 一个修复 commit 解决所有 findings
5. tag vX.Y.Z + push

新切片应该按这个模式继续。

## 14. 测试策略

- **库代码**（数据解析、状态机、纯逻辑）：TDD（test-first）
- **LLM-driven 代码**：test-after，Mock Provider + httptest
- **E2E**：1 个 happy path（`cli/e2e_new_test.go` 用 `chatterProviderHook` 注入 mock）
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

# 完整跑一遍
jianwu new
# ... 答 12 个 grill 问题（接受推荐用回车）...
# 自动产出 books/<slug>/{meta.json, outline.json}

# 扩展单章（v1.0.x 才有 CLI，目前只能写 Go 代码调 expand.Generate）
```

## 16. 文档索引

| 文档 | 用途 |
|---|---|
| `README.md` | 用户视角介绍 + 安装 + 快速上手 |
| `docs/PROJECT_STATUS.md`（本文档） | LLM 友好的当前状态全景 |
| `docs/architecture/overview.md` | 架构图 + 数据流（待写） |
| `docs/decisions/26-grill-decisions.md` | 26 项核心决策记录 |
| `docs/ROADMAP.md` | v1.0.x → v2 路线图 |
| `docs/archive/plans/*.md` | 7 个已完成切片的 SDD plan（归档参考） |
| `DESIGN.md` | 原始设计文档（v1.0 锁定版，部分状态需更新） |
| `EXTRACTION_NOTES.md` | zhurongshuo 资产萃取记录 |
| `LICENSE` | AGPL-3.0 |
