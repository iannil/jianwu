# jianwu (肩吾)

把 LLM 的训练知识结构化为人类可阅读、可学习的图书。  
Go CLI + 库。入口：`cmd/jianwu/main.go`。

## 项目

- **当前版本：** v0.1.6（v0.1.x 全部交付）
- **技术栈：** Go 1.25 + cobra (CLI) + spf13/pflag + YAML 配置 + gemini/glm LLM 提供商
- **入口点：** `cmd/jianwu/main.go` → `cli.NewRootCmd()` → cobra 子命令
- **工作区模型：** 每个项目 = 一个 git 仓库，包含 `.jianwu/` 配置 + `books/<slug>/` 输出
- **下一迭代：** v0.2.2 自动事实复核

## 命令

| 操作 | 命令 |
|--------|---------|
| 构建 | `go build -o ./bin/jianwu ./cmd/jianwu` |
| 全部测试 | `go test ./internal/...` |
| 单包测试 | `go test ./internal/cli/...` |
| 静态检查 | `go vet ./...` |
| 运行 | `go run ./cmd/jianwu <command>` |

## 架构

7 个关键内部包：

- **`internal/cli/`** — cobra 命令树；薄封装层，调用 engine + book + workspace。  
  每个子命令有 `newXxxCmd()` + `runXxx()` 可测试核心。所有 CLI 命令列表见 `docs/PROJECT_STATUS.md §9`。
- **`internal/engine/`** — 4 个子包：`grill/`（访谈）、`outline/`（结构）、`scaffolding/`（框架）、`expand/`（3 轮迭代：调研 → 草稿 → 验证）。核心创作管线。
- **`internal/book/`** — 领域类型：`Meta`（meta.json）、`Outline`（outline.json）、`Chapter`、slug。纯数据 + IO。
- **`internal/provider/`** — 抽象层：`llm/`（Chatter/Embedder 接口）、`search/`、`reader/`，以及工厂包。内置实现：gemini、glm、mock（llm）；brave、serper（搜索）；jina（阅读器）。
- **`internal/config/`** — 5 层合并配置（默认 → 全局 → 工作区 → 环境变量 → 命令行标志）。密钥在 `~/.config/jianwu/secrets.yaml`。
- **`internal/workspace/`** — 工作区的初始化、检测、加载、状态管理。
- **`internal/corpus/`** + **`internal/archetypes/`** + **`internal/style/`** — 嵌入的 YAML 资源（参考语料、图书原型、风格指南）。

## 约定

- **测试：** 表格驱动测试（结构体切片，使用 `in`/`want` 字段，命名用例）。测试函数命名 `TestXxx`（不用 `suite`）。使用 `t.Fatal` / `t.Errorf` / `t.Fatalf`。不用 testify/assert。测试包与源码同级。
- **错误处理：** 用 `fmt.Errorf("context: %w", err)` 包装错误 —— 始终用 `%w` 保持可解包。区分 `*InfoError`（面向用户，带退出码）和内部错误。
- **命名：** Go 标准（camelCase，短变量名）。所有导出的结构体加 JSON 标签（`json:"snake_case"`）。frontmatter 结构体加 YAML 标签。
- **导入：** 标准库第一组，第三方第二组，内部包第三组，组间空行分隔。
- **注释：** 导出的类型/函数写 doc comment（`// Foo does X.`）。设计决策可加交叉引用，如 `// Per decision Q2=B`。
- **库包中不使用 init()**（`embed.go` 中的 `//go:embed` 除外）。无全局状态。
- **小接口：** `Chatter`、`Embedder`、`Streamer` —— Go 风格的窄接口。
- **provider 装配：** 通过 `ProviderDeps` 结构体 + 工厂函数构建；`chatterProviderHook` / `providerDepsHook` 是 test-only 全局可变 var（v0.3.4 重构）。

## 文档索引

| 文档 | 用途 |
|---|---|
| `docs/PROJECT_STATUS.md` | LLM 友好的当前状态全景 |
| `docs/ROADMAP.md` | v0.1.x → v1.0 路线图 |
| `docs/architecture/overview.md` | 架构图 + 数据流 + 关键接口 |
| `docs/decisions/26-grill-decisions.md` | 26 项核心决策 + v0.1.x 审计决策 |
| `docs/EXTRACTION_NOTES.md` | zhurongshuo 资产萃取记录 |
| `docs/archive/plans/` | 已完成切片的 SDD plan（模板参考） |
| `docs/archive/DESIGN.md` | 原始设计文档（v0.1 锁定版，部分已过期） |

## 备注

<!-- 用于在会话中记录 agent 发现的快速备注空间。 -->
