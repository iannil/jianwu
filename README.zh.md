# jianwu

[English](README.md) | 中文

> 肩吾 —— 把 AI 的训练知识结构化为人类可阅读、可学习的图书。

生成中文长篇非虚构的 Go 库 + CLI。Web SaaS 包装层在独立仓库（`mouqin`）。

## 当前状态

**已发布：v0.1.3。** 完整写作闭环端到端跑通——`new → expand → review → finalize → export`。

| 层面 | 内容 | 状态 |
|---|---|---|
| CLI | `init` / `info` / `config get·set·list` / `new` | ✅ 已交付 |
| CLI | `expand <slug> <NN-MM>` | ✅ 已交付 |
| CLI | `review` / `finalize` / `export` / `status`（状态机） | ✅ 已交付 |
| 引擎 | Grill · Outline · Scaffolding · Expand（4 阶段） | ✅ 已交付 |
| 质量 | Expand prompt 注入（archetype + 风格规约 + 样例 + 相邻章节） | ✅ 已交付 |
| CLI | 配置驱动的 fallback 模型装配 | ⏳ 计划中（v0.1.4） |
| CLI | 各阶段 LLM 超时 | ⏳ 计划中（v0.1.5） |
| CLI | Draft 流式输出 | ⏳ 可选（v0.1.6） |

路线图见 [`docs/ROADMAP.md`](docs/ROADMAP.md)，项目全景快照见 [`docs/PROJECT_STATUS.md`](docs/PROJECT_STATUS.md)。

## 安装

```bash
go install github.com/iannil/jianwu/cmd/jianwu@latest
```

或从源码构建：

```bash
git clone https://github.com/iannil/jianwu
cd jianwu
go build -o ./bin/jianwu ./cmd/jianwu
```

## 快速开始

从空 workspace 到导出单一 markdown 图书的完整闭环：

```bash
jianwu init my-library
cd my-library

# API keys（或写入 ~/.config/jianwu/secrets.yaml，权限 0600）
export GEMINI_API_KEY=...
export GLM_API_KEY=...

jianwu new                     # 交互式 grill → outline → scaffolding
                               #   → books/<slug>/{meta.json, outline.json}
jianwu expand <slug> 01-01     # research → draft → validate；写出
                               #   chapters/01-01.md + 引用，更新 outline.json
jianwu review  <slug> 01-01    # 把一章 expanded 标记为 reviewed（人工签发）
jianwu status  <slug>          # 逐章进度 + 下一步动作提示
jianwu finalize <slug>         # 全部 reviewed → final（--dry-run 可预览）
jianwu export   <slug>         # 合并章节 → books/<slug>/export/<slug>.md
                               #   （全局脚注重编号；支持 --dry-run）
```

`<slug>` 是 `jianwu new` 打印的 kebab-case 书籍 id（也是 `books/` 下的目录名）。`<NN-MM>` 是章节地址：部 `NN`、章 `MM`。

状态机是严格的：一章必须先 `expanded` 才能 `review`，全书每章都 `reviewed` 才能 `finalize`。`outline.json` 是状态的单一真相源；章节 `.md` frontmatter 由它镜像同步。

## 配置

5 层，低 → 高优先级：

1. 编译时 defaults（`internal/config/defaults.go`）
2. `~/.config/jianwu/config.yaml`（全局用户）
3. `<workspace>/.jianwu/config.yaml`
4. 环境变量（如 `JIANWU_OUTLINE_MODEL=glm-4.6`）
5. CLI flag（如 `--model glm-4.6`）

```bash
jianwu config get models.outline.provider
jianwu config set scaffolding.concurrency 10
jianwu config list
```

**Secrets** 放在 `~/.config/jianwu/secrets.yaml`（强制 `0600` 权限）或环境变量：`GEMINI_API_KEY` / `GLM_API_KEY` / `BRAVE_API_KEY` / `SERPER_API_KEY` / `JINA_API_KEY`。ENV 按字段覆盖 file。

**各阶段默认模型：**

| 阶段 | 默认模型 |
|---|---|
| Grill | GLM-4.6 |
| Outline | Gemini 2.5 Pro |
| Scaffolding | Gemini 2.5 Flash |
| Expand | GLM-4.6 |

## Providers

一切都藏在小 Go 接口后（`Chatter`、`Embedder`、`Searcher`、`Reader`）；引擎层组合它们。

**LLM**
- **Gemini** 走官方 `google.golang.org/genai` SDK（`gemini-2.5-pro`、`gemini-2.5-flash`、`text-embedding-004`）。
- **GLM** 走直接 REST、OpenAI-compatible 客户端（`glm-4.6`、`glm-4-air`、`embedding-3`）。同一客户端可复用于 Qwen / Moonshot / DeepSeek。
- **Mock** 用于单元测试。

**Search：** Brave Search API（主）→ Serper.dev（备）。
**URL Reader：** Jina Reader（`r.jina.ai`）。

**Retry：** 3 次重试，指数退避（1s → 2s → 4s）+ ±20% jitter，针对 network / 429 / 5xx，context-aware（Ctrl+C 立即取消）。库层已有 `FallbackWrapper`；从 CLI 配置驱动的 fallback 模型选择在 v0.1.4 装配。

## 引擎

jianwu 提供完整 4 阶段引擎。每个阶段都是独立、可调用的库包。

```
jianwu new
  ↓
grill.Run        # 12 维度设计树；用户接受 / 修改 LLM 的推荐
  ↓
outline.Generate # 单次 LLM 调用，JSON Schema 强制输出 → book.Outline
  ↓
scaffolding      # N 章并行（errgroup，continue-on-error）
  ↓
books/<slug>/{meta.json, outline.json}  +  归档会话 .session.json
```

```
jianwu expand <slug> <NN-MM>
  ↓ iter 1  research   web_search × N + read_url × M → research notes + 引用候选
  ↓ iter 2  draft      LLM 写 markdown + [^N] 脚注
  ↓ iter 3  validate   自检 + 修订 → claims[].has_citation
  ↓
ParseFootnotes + 合并引用元数据
  → chapters/NN-MM.md（frontmatter + 正文）+ 更新 outline.json 状态/引用/字数
```

Expand prompt 真正注入 archetype YAML、完整风格规约、few-shot 风格样例和相邻章节选段——让正文对齐 zhurongshuo 文体，而非 generic LLM 输出。

## 架构

```
cmd/jianwu/main.go                    # CLI 入口（exit code 映射）
internal/
  cli/                                # cobra 命令层
    root / init / info / config / new
    expand / review / finalize / export / status
    prompt                            # TerminalPrompt（grill.UserInput 实现）
    providers / new_flow              # 编排 + provider 装配
  workspace/                          # .jianwu/ 加载、walk-up detect、Init/Load
  config/                             # 5 层 resolver + secrets（ENV > file, 0600）
  book/                               # Meta/Outline/Chapter/Citation 类型 + JSON I/O + Slugify
  archetypes/ style/ corpus/          # 内置数据（//go:embed FS）
  provider/
    llm/        gemini/ glm/ mock/    # Chatter + Embedder + Retry/Fallback wrappers
    search/     brave/ serper/        # Searcher
    reader/     jina/                 # Reader
    llmfactory/ searchfactory/ readerfactory/   # 工厂（独立包以打破 import cycle）
  engine/
    outline/                          # 单次 LLM 调用 → book.Outline
    scaffolding/                      # N 章并行 + RetryFailed
    grill/                            # 12 维度设计树 + stateful session
    expand/                           # 3-iteration agent + 工具调用 + citation 解析
```

**包依赖图（无环）：** `cli → engine → provider → book / config / workspace`。三个 `*factory` 包横跨 provider 子树以打破 import cycle。

**Workspace 布局：**

```
<workspace>/
  .jianwu/
    config.yaml            # workspace 配置（覆盖全局）
    schema_version         # "1"
    sessions/<id>.json     # 运行中的 grill 会话
  books/<slug>/
    meta.json              # id / slug / title / archetype / parameters / engine versions
    outline.json           # parts[] × chapters[]；逐章 status + citations（真相源）
    .session.json          # 已完成的 grill 会话（audit log）
    chapters/NN-MM.md      # expand 产出：frontmatter + markdown + [^N] 脚注
    export/<slug>.md       # `jianwu export` 产出
  exports/  archive/       # 预留（v0.2+）
```

章节 `status` 流转 `scaffolded → expanded → reviewed → final`（出错时 `failed`）。

## 库 API

所有引擎逻辑都在库里；CLI 和未来的 web app 包同一组包。

```go
outline.Generate(ctx, chatter, Input) (*book.Outline, error)

scaffolding.GenerateChapter(ctx, chatter, ChapterInput) (*ChapterOutput, error)
scaffolding.ScaffoldAll(...) map[string]Result
scaffolding.RetryFailed(...)

grill.NewSession()
grill.Run(...)
grill.Repository{ Save, Load, ListIncomplete, Archive }

expand.Generate(ctx, chatter, tools, ExpandInput) (*ExpandOutput, error)
```

Provider 接口：

```go
type Chatter  interface { Chat (ctx, ChatRequest)  (*ChatResponse,  error) }
type Embedder interface { Embed(ctx, EmbedRequest) (*EmbedResponse, error) }
type Searcher interface { Search(ctx, Query)       (*Results,       error) }
type Reader   interface { Read (ctx, url string)   (string,         error) }
```

**错误分类**（驱动 retry / fallback / exit code）：`ErrNetwork` / `ErrRateLimit` / `ErrServer` 触发重试再 fallback；`ErrLLMProvider`（4xx）两者都不做。退出码：`0` 成功 · `1` 通用 · `2` 用法 · `3` workspace 未找到 · `4` LLM 错 · `5` 网络错。

## 开发

```bash
go test ./...                                 # 全部包
go build -o ./bin/jianwu ./cmd/jianwu
go vet ./...  &&  gofmt -l .                   # 必须全清

go test -run TestE2E ./internal/cli/...        # E2E happy path（mock provider）
GEMINI_API_KEY=xxx go test ./internal/engine/expand/... -run TestGenerateLive   # 真 LLM
```

**测试策略：** 库 / 状态机 / 纯逻辑代码 test-first（TDD）；LLM-driven 代码 test-after，用 Mock provider + `httptest`。Live integration 测试在无 API key 时 SKIP。

**开发工作流（SDD —— subagent-driven development）：**

1. `/grill-me` —— 在设计树上对齐决策。
2. `writing-plans` —— 出 task-by-task 计划（每 task TDD：RED → GREEN → commit）。
3. `subagent-driven-development` —— 每 task 派 fresh implementer subagent + task reviewer。
4. 切片完成 → whole-branch review → 一个修复 commit 解决所有 findings。
5. tag `vX.Y.Z` + push。

内置数据资产（`//go:embed`）：3 个 archetype YAML、1 个风格规约 + 3 个 few-shot 样例、6 本内置 corpus JSON。运行时对 zhurongshuo 仓库零外部依赖。

完整设计见 [`docs/archive/DESIGN.md`](docs/archive/DESIGN.md)（v0.1 锁定版），决策记录见 [`docs/decisions/26-grill-decisions.md`](docs/decisions/26-grill-decisions.md)。

## 许可

代码：**AGPL-3.0**（见 [`LICENSE`](LICENSE)）。

内置 zhurongshuo 参考数据（`internal/archetypes/`、`internal/style/`、`internal/corpus/`）：© zhurong，仅内部使用，不可再分发。
