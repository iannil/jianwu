# jianwu (肩吾)

把 LLM 的训练知识结构化为人类可阅读、可学习的图书。  
Go CLI + 库。入口：`cmd/jianwu/main.go`。

## 项目

- **当前版本：** v0.3.5（SaaS-ready 单租户内核 + mouqin.com 官网 co-hosted + 全管线交付）
- **技术栈：** Go 1.25 + cobra (CLI) + spf13/pflag + YAML 配置 + gemini/glm/ollama LLM 提供商
- **入口点：** `cmd/jianwu/main.go` → `cli.NewRootCmd()` → cobra 子命令
- **工作区模型：** 每个项目 = 一个 git 仓库，包含 `.jianwu/` 配置 + `books/<slug>/` 输出
- **下一迭代：** v0.3.6 — 发布流程（`--version` + `release.sh`）+ Token 累计扩展 + 测试补全
- **审计决策：** v0.3.5 ship 后审计（2026-06-30）— 见 `docs/decisions/27-v0.3-audit-decisions.md`（v0.3 重定义为 single-tenant SaaS-ready；v0.4 多租户推迟到 mouqin 上线后触发）

## 命令

| 操作 | 命令 |
|--------|---------|
| 构建 | `go build -o ./bin/jianwu ./cmd/jianwu` |
| 全部测试 | `go test ./internal/...` |
| 单包测试 | `go test ./internal/cli/...` |
| 静态检查 | `go vet ./...` |
| 运行 | `go run ./cmd/jianwu <command>` |

## 架构

8 个关键内部包（含 2 个新包 v0.2.0 + storage v0.3.0）：

- **`internal/cli/`** — cobra 命令树；薄封装层，调用 engine + book + workspace。  
  每个子命令有 `newXxxCmd()` + `runXxx()` 可测试核心。所有 CLI 命令列表见 `docs/PROJECT_STATUS.md §9`。
  全局标志：`--verbose`/`-L`、`--debug`、`--dir`/`-d`（指定 workspace 根目录，详见 `root.go`）。
  共享辅助函数在 `book_resolve.go`（`loadBook`/`findChapter`/`findPart`/`parseChapterAddr`/`mirrorChapterStatus`）。
- **`internal/engine/`** — 6 个子包：`grill/`（访谈）、`outline/`（结构）、`scaffolding/`（框架）、`expand/`（3 轮迭代：调研 → 草稿 → 验证）、`factcheck/`（自动事实复核）、`revise/`（基于 verdicts 修订章节）。核心创作 + 质量管线。
- **`internal/book/`** — 领域类型：`Meta`（含 `ClaimWhitelist`）、`Outline`（含 `Verdicts[]`）、`Chapter`、`Claim`、`ClaimVerdict`、slug。纯数据 + IO。
- **`internal/provider/`** — 抽象层：`llm/`（Chatter/Embedder/Streamer 接口）、`search/`、`reader/`，以及工厂包。内置实现：gemini、glm、ollama、mock（llm）；brave、serper（搜索）；jina（阅读器）。
- **`internal/storage/`** — `Storage` 接口（v0.3.0 地基）：ReadFile/WriteFile/MkdirAll/RemoveAll/Rename/Stat/ReadDir。默认 `OS` 实现 + `MemStorage` 测试实现（含 16 个测试）。book/workspace/config/cli/grill 已迁移。
- **`internal/config/`** — 5 层合并配置（默认 → 全局 → 工作区 → 环境变量 → 命令行标志）。密钥在 `~/.config/jianwu/secrets.yaml`。
- **`internal/workspace/`** — 工作区的初始化、检测、加载、状态管理（使用 `storage.OS`）。
- **`internal/corpus/`** + **`internal/archetypes/`** + **`internal/style/`** — 嵌入的 YAML 资源（参考语料、图书原型、风格指南）。

## 约定

- **测试：** 表格驱动测试（结构体切片，使用 `in`/`want` 字段，命名用例）。测试函数命名 `TestXxx`（不用 `suite`）。使用 `t.Fatal` / `t.Errorf` / `t.Fatalf`。不用 testify/assert。测试包与源码同级。
- **错误处理：** 用 `fmt.Errorf("context: %w", err)` 包装错误 —— 始终用 `%w` 保持可解包。区分 `*InfoError`（面向用户，带退出码）和内部错误。
- **命名：** Go 标准（camelCase，短变量名）。所有导出的结构体加 JSON 标签（`json:"snake_case"`）。frontmatter 结构体加 YAML 标签。
- **导入：** 标准库第一组，第三方第二组，内部包第三组，组间空行分隔。
- **注释：** 导出的类型/函数写 doc comment（`// Foo does X.`）。设计决策可加交叉引用，如 `// Per decision Q2=B`。
- **库包中不使用 init()**（`embed.go` 中的 `//go:embed` 除外）。无全局状态。
- **小接口：** `Chatter`、`Embedder`、`Streamer` —— Go 风格的窄接口。
- **provider 装配：** 通过 `ProviderDeps` 结构体 + 工厂函数构建；~~`chatterProviderHook` / `providerDepsHook`~~ 已清除，v0.3.4 用显式参数注入。

## 文档索引

| 文档 | 用途 |
|---|---|
| `docs/CAPABILITIES.md` | 用户面向的功能概览（CLI 命令、引擎管线、Provider、配置、导出） |
| `docs/PROJECT_STATUS.md` | LLM 友好的当前状态全景（更新至 v0.2.0 + 2026-06-28 审计修复） |
| `docs/ROADMAP.md` | v0.1.x → v1.0 路线图（更新至 v0.2.x 实际状态） |
| `docs/architecture/overview.md` | 架构图 + 数据流 + 关键接口 |
| `docs/decisions/26-grill-decisions.md` | 26 项核心决策 + v0.1.x 审计决策 |
| `docs/EXTRACTION_NOTES.md` | zhurongshuo 资产萃取记录 |
| `docs/archive/plans/` | 已完成切片的 SDD plan（v0.1.0–v0.1.6 + v0.2.0）|
| `docs/archive/DESIGN.md` | 原始设计文档（v0.1 锁定版，部分已过期） |

## 备注

<!-- 用于在会话中记录 agent 发现的快速备注空间。 -->
