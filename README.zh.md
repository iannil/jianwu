# jianwu (肩吾)

[English](README.md) | 中文

> 把 LLM 的训练知识结构化为可阅读、可学习的图书 —— 一条命令完成。

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev)
[![Go Report Card](https://goreportcard.com/badge/github.com/iannil/jianwu)](https://goreportcard.com/report/github.com/iannil/jianwu)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen)](CONTRIBUTING.md)
[![GitHub Stars](https://img.shields.io/github/stars/iannil/jianwu?style=social&label=Stars)](https://github.com/iannil/jianwu)

**jianwu** 是一个 Go CLI + 库，编排 LLM 写出长篇非虚构图书 —— 带目录、一致引用、章节级状态管理和多格式导出。

Web SaaS 版本：[mouqin.com](https://mouqin.com) — Early Access

---

## 适用人群

- **写作者 & 研究者** — 把研究笔记变成有结构的、带脚注的图书
- **LLM 高级用户** — 受够了 token 限制和缺少书级结构
- **中文非虚构作者** — 为长篇中文写作设计的 AI 辅助管线
- **Go 开发者** — 对 LLM 编排、provider 抽象、prompt 工程感兴趣

## 解决的问题

LLM 有海量训练知识，但只能输出 token。它们无法给你一本带目录、一致引用和章节状态管理的书。大部分 AI 写作工具产的是**文字**——段落、邮件、博客。

**jianwu 产的是书。**

## 功能

| 领域 | 能力 |
|---|---|
| **创作管线** | Grill（设计）→ Outline → Scaffold → Expand（调研 → 草稿 → 验证） |
| **质量保障** | 逐 claim URL 验证 + 自动修订 |
| **导出** | Markdown、Hugo 站点、PDF（单书或全书站） |
| **LLM 提供商** | Gemini 2.5 Pro/Flash、GLM-4、Ollama（本地）、Mock |
| **搜索 & 阅读** | Brave Search、Serper（备选）、Jina Reader |
| **语料** | 内置参考书 + 嵌入索引（RAG） |
| **配置** | 5 层合并（默认 → 全局 → 工作区 → 环境变量 → 标志） |
| **开发者体验** | 窄 Go 接口、状态机、表格驱动测试 |

**当前版本：v0.3.5。** [完整状态 →](docs/PROJECT_STATUS.md) | [路线图 →](docs/ROADMAP.md)

## 快速开始

```bash
# 安装
go install github.com/iannil/jianwu/cmd/jianwu@latest

# 或从源码构建
git clone https://github.com/iannil/jianwu
cd jianwu && go build -o ./bin/jianwu ./cmd/jianwu

# 设置 API key
export GEMINI_API_KEY=...

# 从零到导出，一次会话完成
jianwu init my-book && cd my-book
jianwu new                     # grill → outline → scaffolding
jianwu expand my-book 01-01    # research → draft → validate
jianwu status my-book          # 进度 + 下一步
jianwu finalize my-book        # 锁稿
jianwu export my-book          # 全书 markdown + 脚注
```

📖 [完整入门指南 →](docs/getting-started.md)

## 架构

```
cmd/jianwu/main.go
├── internal/cli/            # cobra 命令树
├── internal/engine/
│   ├── grill/               # 12 维度设计树
│   ├── outline/             # 单次 LLM 调用 → JSON Schema 输出
│   ├── scaffolding/         # N 章并行（errgroup）
│   ├── expand/              # 3 轮迭代：调研 → 草稿 → 验证
│   ├── factcheck/           # 逐 claim URL 验证
│   └── revise/              # 自动修订 + 新引用
├── internal/provider/
│   ├── llm/                 # Chatter / Embedder / Streamer 接口
│   ├── search/              # Searcher 接口
│   ├── reader/              # Reader 接口
│   └── gemini/ glm/ ollama/ brave/ serper/ jina/  # 具体实现
├── internal/book/           # Meta / Outline / Chapter / Claim 类型
├── internal/config/         # 5 层配置合并
├── internal/workspace/      # 工作区管理
├── internal/storage/        # Storage 接口（OS + MemStorage）
└── internal/corpus/         # 参考语料 + 嵌入索引
```

**关键设计决策：**
- 窄接口（5 行定义一个 provider）
- 无全局状态，无 `init()`（`//go:embed` 除外）
- 状态机：`scaffolded → expanded → reviewed → finalized`
- 表格驱动测试 + Mock provider；零 testify 依赖
- AGPL-3.0（代码）+ 内部使用的 zhurongshuo 数据

## Providers

一切都藏在小 Go 接口后。接入新 provider = 实现接口 + 注册工厂。

- **LLM：** Gemini 2.5 Pro/Flash、GLM-4.6/Air、Ollama（本地：Qwen、Llama 等）、Mock
- **搜索：** Brave Search（主）→ Serper（备）
- **阅读：** Jina Reader

**重试：** 3 次，指数退避（1s→2s→4s）+ ±20% jitter；context-aware（Ctrl+C 取消）。FallbackWrapper 链式切换 provider。

## 资源

| 资源 | 链接 |
|---|---|
| 博客 | [mouqin.com/blog/](https://mouqin.com/blog/) |
| 引擎详解 | [mouqin.com/engine/](https://mouqin.com/engine/) |
| CLI 参考 | [mouqin.com/docs/commands/](https://mouqin.com/docs/commands/) |
| 配置指南 | [mouqin.com/docs/configuration/](https://mouqin.com/docs/configuration/) |
| 架构文档 | [docs/architecture/overview.md](docs/architecture/overview.md) |
| 路线图 | [docs/ROADMAP.md](docs/ROADMAP.md) |
| 决策记录 | [docs/decisions/26-grill-decisions.md](docs/decisions/26-grill-decisions.md) |

## 贡献

欢迎 PR！见 [CONTRIBUTING.md](CONTRIBUTING.md)。

**开发：**

```bash
go test ./...                       # 全部测试（Mock provider）
go build -o ./bin/jianwu ./cmd/jianwu
go vet ./...  &&  gofmt -l .        # 必须全清

# Live LLM 测试（需要 API key）
GEMINI_API_KEY=xxx go test ./internal/engine/expand/... -run TestGenerateLive
```

**SDD 工作流：** subagent-driven development — `/grill-me` 定设计决策，writing-plans 出 task 计划，每 task 独立 subagent。

---

⭐ **如果你觉得这个项目有用，给个 star —— 帮助更多人发现它。**

## 许可

代码：**AGPL-3.0**（见 [LICENSE](LICENSE)）。

内置 zhurongshuo 参考数据（`internal/archetypes/`、`internal/style/`、`internal/corpus/`）：© zhurong，仅内部使用，不可再分发。
