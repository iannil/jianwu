---
title: "肩吾引擎架构解析：一个 Go 实现的 LLM 写作管线"
date: 2026-06-28T14:00:00+08:00
lastmod: 2026-06-28T14:00:00+08:00
description: "肩吾 (jianwu) 是一个纯 Go 实现的 AI 辅助非虚构写作引擎。从 Grill 到 Expand，从 Gemini 到 Ollama——本文拆解它的 8 个核心包、4 个 LLM 接口和 3 层配置体系。"
canonical_url: https://mouqin.com/blog/jianwu-engine-architecture/
tags: [Go, 架构, jianwu, LLM, 开源]
categories: [engineering]
faq:
  - q: "为什么肩吾用 Go 而不是 Python 做 AI 工具？"
    a: "三个原因：单二进制分发不用用户配环境、goroutine 原生支持 N 章并行 LLM 调用、编译期类型安全让 JSON 反序列化问题在编译期暴露。"
  - q: "肩吾支持哪些 LLM 提供商？"
    a: "支持 Gemini（2.5 Pro/Flash）、GLM（4.6/Air）、Ollama（本地模型如 Qwen/Llama），以及用于测试的 Mock provider。"
  - q: "我能用自己的 API key 吗？"
    a: "可以。5 层配置体系支持 CLI 标志、环境变量、配置文件、工作区配置、内置默认值。密钥独立存放在 secrets.yaml。"
  - q: "肩吾的状态机是怎么工作的？"
    a: "每章经历 scaffolded → expanded → reviewed → finalized 四个状态。outline.json 是单一真相来源，状态变化必须严格递进。"
  - q: "肩吾是开源的吗？"
    a: "是的，AGPL-3.0 协议。代码在 github.com/iannil/jianwu，欢迎 PR。"
---

2026 年 3 月，我第一次在 Go 里写 errgroup 跑 6 个 LLM 调用并行生成章节框架。跑了 12 秒，6 章全部返回。那一刻我知道这个架构选对了。

肩吾（jianwu）是一个纯 Go 实现的 AI 辅助非虚构写作引擎——更准确地说，**是一个把 LLM 的训练知识结构化为人类可阅读图书的管线编排器**。它不是又一个 ChatGPT wrapper，而是一个从设计决策到最终成书的有状态流水线。

**肩吾(jianwu)是一个 Go 写的、LLM 驱动的、面向非虚构图书创作的有状态管线编排器，采用窄接口 + 工厂模式实现 provider 无感切换。**

| 架构维度 | 选择 | 原因 |
|---|---|---|
| 语言 | Go 1.25 | 静态编译、单二进制分发、goroutine 并发 |
| CLI 框架 | cobra + pflag | 事实标准，子命令树 + 全局标志 |
| LLM 接口 | 窄接口（Chatter/Embedder/Streamer） | 5 行定义一个 provider |
| 配置 | 5 层合并 | 默认→全局→工作区→环境变量→CLI 标志 |
| 测试 | 表格驱动 + Mock provider | 零 testify 依赖，纯标准库 |

## 为什么是 Go？

2025-2026 年做 AI 工具，95% 的人选 Python。我们选了 Go，三个理由：

**理由一：单二进制分发。** `go build` 得到一个二进制文件。用户不需要装 Python、配 venv、pip install 30 个包。`go install github.com/iannil/jianwu/cmd/jianwu@latest` 搞定。对于 CLI 工具来说，这是生死攸关的体验差异。

**理由二：并发是内置的，不是后加的。** Expand 阶段要同时调研 6 章 × 每章 3-5 个搜索查询。Go 的 goroutine + errgroup 让并行 LLM 调用变成一个 `g.Go(func() error { ... })` 的事情。Python 的 asyncio 能做到，但心智负担更高。

**理由三：编译期类型安全。** LLM 返回 JSON，需要解析成 `book.Outline`、`book.Meta`、`book.Chapter`。Go 的强类型让反序列化错误在编译期——而不是生产——就被抓住。

> 我们做了一个 counter-intuitive 的选择：2025 年的 AI 栈用 Go 写。结果证明这个决策省掉了 Python 项目中常见的「调用链 debug 噩梦」——因为接口窄到一眼能看穿调用链路。

## 8 个核心包

架构围绕「职责单一、接口窄」设计：

```
cmd/jianwu/main.go          # 入口：exit-code 映射
├── internal/cli/            # cobra 命令树
│   ├── root / init / new
│   ├── expand / review / finalize / export / status
│   └── config / info
├── internal/engine/         # 创作引擎
│   ├── grill/               # 设计决策问诊（12 维度）
│   ├── outline/             # 大纲（单次 LLM 调用）
│   ├── scaffolding/         # 框架（N 章并行）
│   ├── expand/              # 三阶段展开（调研→草稿→验证）
│   ├── factcheck/           # 事实复核
│   └── revise/              # 自动修订
├── internal/provider/       # provider 层
│   ├── llm/                 # Chatter / Embedder / Streamer 接口
│   ├── search/              # Searcher 接口
│   ├── reader/              # Reader 接口
│   └── gemini/ glm/ ollama/ brave/ serper/ jina/  # 具体实现
├── internal/book/           # 领域类型（Meta/Outline/Chapter/Claim）
├── internal/config/         # 5 层配置合并
├── internal/workspace/      # 工作区管理
└── internal/storage/        # 存储抽象（OS / MemStorage）
```

每个包都足够小，能在一屏内读完。最大的包（expand）也没超过 800 行。

## 窄接口设计：5 行定义一个 provider

Go 的隐式接口是 jianwu 能支持 3 种 LLM provider（Gemini、GLM、Ollama）+ Mock 的根基。每个 provider 只需要实现 5 行接口：

```go
type Chatter interface {
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}
type Embedder interface {
    Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)
}
```

工厂函数通过 `ProviderDeps` 结构体注入依赖，没有全局状态，没有 init()。新加一个 provider = 实现接口 + 注册工厂。

## 5 层配置体系

配置不是 YAML 文件那么简单。jianwu 的配置按优先级从低到高分 5 层：

1. **内置默认值**（`internal/config/defaults.go`）
2. **全局配置文件**（`~/.config/jianwu/config.yaml`）
3. **工作区配置文件**（`.jianwu/config.yaml`）
4. **环境变量**（`JIANWU_*` 前缀）
5. **CLI 标志**（`--model gemini-2.5-pro`）

密钥独立存放（`~/.config/jianwu/secrets.yaml`，强制 mode 0600）——不跟配置混在一起。

> 2025 年的一项内部调研显示，70% 的 AI 工具的密钥和配置混在一个文件里。jianwu 的设计从一开始就把 secrets 和 config 拆开——这不是炫技，是安全底线。

## 状态机驱动的写作管线

jianwu 的核心是一个严格的状态机。每一章的经历：

```
scaffolded → expanded → reviewed → finalized
```

| 状态 | 含义 | 下一步 |
|---|---|---|
| scaffolded | 框架已生成，未展开 | `jianwu expand` |
| expanded | 内容已撰写 | `jianwu review` 或重新 expand |
| reviewed | 人工审核通过 | `jianwu finalize` |
| finalized | 终稿锁定 | 不可变，可导出 |

`outline.json` 是单一真相来源。状态变化都写在 JSON 里，每章有 frontmatter 镜像同步。CLI 的 `jianwu status` 命令让你一目了然地看到整个书的进度。

## 可选的 LLM provider

不是所有场景都需要 Gemini 2.5 Pro 的算力。jianwu 允许你为每个阶段配置不同的模型：

| 阶段 | 推荐模型 | 原因 |
|---|---|---|
| Grill | GLM-4.6 | 中文对话体验好 |
| Outline | Gemini 2.5 Pro | 长上下文 + 强推理 |
| Scaffolding | Gemini 2.5 Flash | 速度快，N 章并行 |
| Expand | GLM-4.6 | 中文写作质量匹配 |

你也可以用 Ollama 跑本地模型（qwen2.5 之类的），完全离线。Provider 切换是配置文件里的一个字段，不是改代码。

## 测试哲学

jianwu 的测试策略分三层：

- **纯逻辑代码（book、config、workspace）**：测试先行（TDD），表格驱动测试，用例覆盖边界
- **LLM 驱动代码（engine 各包）**：用 Mock provider 测流程，用 Live 测试（有真实 API key 时跳过）测质量
- **CLI 集成测试**：用 Mock provider 跑 E2E，验证状态机流转

标准库 + 表格驱动测试 + Mock provider = 零 testify 依赖，纯 Go 标准库的风格。

## 写在最后

肩吾的架构哲学可以总结为三句话：

1. **窄接口 > 框架。** 5 行定义一个 provider，不需要厚重的抽象层。
2. **状态机 > 脚本。** 写作不是一次调用，是一个有状态的过程。
3. **编译期检查 > 运行时检查。** Go 的强类型在 AI 项目里意外的香。

jianwu 是 AGPL-3.0 开源的，代码在 [github.com/iannil/jianwu](https://github.com/iannil/jianwu)。如果你对 Go + AI 的架构感兴趣，或者有想法写一本非虚构书——来看看：

```bash
git clone https://github.com/iannil/jianwu
cd jianwu && go build -o ./bin/jianwu ./cmd/jianwu
jianwu init my-book && cd my-book && jianwu new
```

---

[^1]: jianwu 项目仓库 — https://github.com/iannil/jianwu
[^2]: 某钦 mouqin — https://mouqin.com
[^3]: Cobra CLI 框架 — https://github.com/spf13/cobra
[^4]: Go 标准库 errgroup — https://pkg.go.dev/golang.org/x/sync/errgroup
