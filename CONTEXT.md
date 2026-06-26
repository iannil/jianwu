# jianwu (肩吾) — Domain Context

> 将 LLM 的训练知识结构化为人类可阅读、可学习的图书。
> Go CLI + 库。
>
> 本文档记录领域概念、术语表、关键决策及其背后的理由。
> 修改代码前先读此文件，确保变更与领域模型一致。

## 核心概念

### 图书（Book）

由 `grill → outline → scaffolding → expand` 管线生成的结构化知识产物。

```
workspace/
  .jianwu/config.yaml        # 工作区配置
  .jianwu/sessions/          # 活跃的 grill 会话（resumable）
  books/<slug>/
    meta.json                # 图书元数据（来自 grill session）
    outline.json             # 大纲 + 章节状态
    chapters/NN-MM.md        # 各章 markdown（含 YAML frontmatter）
    .session.json            # 归档的 grill 会话（audit log）
```

### Slug

图书的唯一标识符，由 topic 推导而来（`deriveSlugFromTopic`）。用作 `books/` 下的目录名。不可变，一旦创建不改。

### 12 维设计树（Grill Design Tree）

访谈阶段覆盖的 12 个维度，6 个核心 + 6 个条件触发：

**核心维度（始终询问）：**
1. `topic` — 核心问题（自由文本，无 Options）
2. `audience` — 受众（scholar / advanced-practitioner / educated-general / beginner）
3. `goal` — 目标（understanding / operational / decision）
4. `archetype` — 结构原型（ontology-epistemology-practice / diagnosis-decoding-breakthrough / foundations-application-practice）
5. `depth` — 深度（intro / intermediate / advanced）
6. `length` — 篇幅（short / medium / long）

**条件/可选维度：**
7. `language` — 语言（zh / en / bilingual）
8. `scope` — 范围（single / volume / chapter）
9. `example_type` — 例子类型（case / thought_experiment / data / mixed）
10. `citation_style` — 引用风格，仅当 audience=scholar 时（academic / popular / none）
11. `visualization` — 可视化（charts / tables / none）
12. `timeliness` — 时效（timeless / current / forward）

### 结构原型（Archetype）

三种图书骨架，定义章节结构和展开方向：

- **ontology-epistemology-practice**（本体论—认识论—实践）：从「是什么」到「怎么用」
- **diagnosis-decoding-breakthrough**（诊断—解码—突破）：从「问题在哪」到「怎么做」
- **foundations-application-practice**（基础—应用—实践）：从「需要知道什么」到「能做什么」

每个原型对应一个嵌入的 YAML 文件（`internal/archetypes/`）。

### 引擎管线（Engine Pipeline）

4 个阶段按顺序执行，每阶段有独立的 Context（带超时和信号取消）：

1. **Grill** — 访谈，收集 12 维设计决策，LLM 推荐 + 用户确认，可恢复
2. **Outline** — 单次 LLM 调用生成 JSON Schema 强制的 Outline
3. **Scaffolding** — 并行生成各章框架（errgroup, concurrency=5），continue-on-error
4. **Expand** — 3 轮迭代：Research（搜索+读URL）→ Draft（LLM写稿）→ Validate（自检+修订）

### 配置优先级（5 层）

CLI flag > ENV var > workspace config > global config > builtin defaults

Secrets（API keys）单独走：ENV > `~/.config/jianwu/secrets.yaml`（强制 0600）

### Provider 抽象

```
Chatter  → [RetryWrapper → FallbackWrapper] → 具体 provider (gemini/glm)
Searcher → 具体 provider (brave/serper)
Reader   → 具体 provider (jina)
```

三工厂包（`llmfactory`/`searchfactory`/`readerfactory`）按名称构造 provider，避免循环依赖。

## 术语表

| 术语 | 说明 |
|---|---|
| **workspace** | 一个 git 仓库 + `.jianwu/` 配置目录 |
| **slug** | 图书标识符，由 topic 推导，用作目录名 |
| **archetype** | 图书结构原型（3 种），定义章节骨架 |
| **grill** | 设计访谈阶段，12 维决策树 |
| **scaffolding** | 并行生成各章框架（errgroup） |
| **expand** | 逐章展开（research→draft→validate） |
| **outline** | 生成图书大纲（单次 LLM 调用，JSON Schema 约束） |
| **Chatter** | LLM 聊天接口（Chat+Stream 变体） |
| **Embedder** | LLM 嵌入接口 |
| **Searcher** | 搜索 API 接口（Brave/Serper） |
| **Reader** | URL 阅读器接口（Jina） |
| **frontmatter** | 章节 markdown 文件的 YAML 头信息 |
| **citation** | 引用元数据（URL + 标题 + 来源），expand 阶段收集 |
| **InfoError** | 带退出码的用户友好错误，映射到 CLI exit code |

## 决策记录索引

关键决策（Q1-Q26）记录在 `docs/decisions/26-grill-decisions.md`。每次遇到需决策的问题，以 `Q<N>=<option>` 形式记录，后续代码注释中引用（如 `// Per Q11.A2`）。

## 测试策略

- 库代码：TDD（test-first），表格驱动测试
- LLM-driven：test-after，Mock Provider + httptest
- 跨切：E2E 用 `chatterProviderHook`（test-only 全局）注入 mock
- Live：API key 存在时跑真实 LLM，否则 SKIP
- 测试文件与生产代码同级（非 `_test/` 包）
- 不使用 testify/suite

## 约束

- Go 1.25+
- Module path：`github.com/iannil/jianwu`
- 无全局可变状态（除 `chatterProviderHook` / `providerDepsHook` — 标记废弃，计划 v0.2.6 重构）
- 错误始终用 `fmt.Errorf("context: %w", err)` 包装（`%w`）
- 导出的结构体加 JSON 标签（`json:"snake_case"`）
