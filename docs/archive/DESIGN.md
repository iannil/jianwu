# jianwu 设计文档

> 肩吾 —— 把 AI 的训练知识结构化为人类可阅读、可学习的图书。
> 库 + CLI 部分。Web SaaS 包装层是独立仓库 `mouqin`。

最后更新：2026-06-22
状态：**v0.1.0 已交付**（tag 在 master）。当前文档反映 v0.1 锁定时的设计，实施进展以 `docs/PROJECT_STATUS.md` 为准。

---

## 1. 项目概述

### 1.1 使命

人只需要输出他的需求，AI 来负责将需求梳理成体系化、结构化、人类能理解的结构和大纲，并按需展开为带引用的可信成稿。

### 1.2 两个仓库

| 仓库 | 角色 | 阶段 |
|---|---|---|
| `jianwu` | 核心引擎（库 + CLI），本仓库 | v0.1 优先 |
| `mouqin` | Web SaaS，包装 jianwu 库对外服务 | v1.0 |

库优先：所有核心逻辑写在 `jianwu` 库里，CLI 和 Web 都包同一个库。

### 1.3 与 zhurongshuo 的关系

**完全独立。** zhurongshuo 是参考与方法论来源，不是部署目标。

- zhurongshuo 提供：方法论参考、参考语料、底层思想
- zhurongshuo 不提供：产品形态、文件格式、元数据 schema、部署目标
- 任何"输出到 zhurongshuo"是适配器层（`jianwu export --target zhurongshuo`），在 v0.2 实现

**独立性检验**：如果 zhurongshuo 仓库明天消失，jianwu 仍能正常运行、用所有原型、完成全流程；只是不能 sync 更新参考语料。

---

## 2. 核心原则

1. **库优先**：所有引擎逻辑写成 Go 库；CLI 和 Web 都是薄包装。
2. **配置驱动**：模型映射、原型偏好、provider 选择都在配置文件里，不写死代码。
3. **多厂商从 day 1 做实**：LLM provider、search provider 都从一开始就抽象，不留"预留接口"。
4. **批次分层锚定**：脚手架阶段用参数化知识（快、便宜），展开阶段默认开 web search 锚定（可信）。
5. **强制引用 + 人工签发**：事实性陈述必须有 source；status workflow 兜底质量。
6. **独立于 zhurongshuo**：内置最小语料 + sync 扩展语料分层；运行时零外部依赖。
7. **显式 workspace**：一个 workspace = 一个 git 仓库 = 一个逻辑集合。

---

## 3. 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│  mouqin (Web SaaS, v1.0)                                      │
│  包装 jianwu 库，对外服务                                    │
└─────────────────────────────────────────────────────────────┘
                          │ 包同一个库
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  jianwu (库 + CLI, v0.1)                                      │
│                                                              │
│  ┌─ CLI 命令（v0.1）                                        │
│  │  init / new / list / show / status                       │
│  │  expand / review / finalize / config / info              │
│  │                                                          │
│  ├─ Workspace：.jianwu/config.yaml + books/<slug>/          │
│  │  meta.json + outline.json + grill.log + chapters/NN-MM.md│
│  │                                                          │
│  ├─ 引擎（4 阶段 LLM 编排）                                  │
│  │  Grill(stateful) → Outline(batch) →                     │
│  │  Scaffolding(并行) → Expand(agent + tools)               │
│  │                                                          │
│  ├─ 原型库（6 个，先做 3）                                   │
│  │  内置 internal/archetypes/ + 用户 ~/.config/jianwu/      │
│  │                                                          │
│  ├─ LLM Providers：Gemini + GLM，分阶段配模型               │
│  │                                                          │
│  ├─ Search：Brave(主) + Serper(备) + Jina Reader(URL 内容)  │
│  │                                                          │
│  └─ 参考语料：内置最小 + ~/.local/share/jianwu/corpus/      │
│     embedding 索引支持相似检索                               │
└─────────────────────────────────────────────────────────────┘
                          │ 完全独立（仅通过一次性萃取）
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  zhurongshuo                                                │
│  - 提供：方法论参考、参考语料、底层思想                       │
│  - 不提供：产品形态、文件格式、部署目标                       │
│  - 仅通过一次性萃取进入 jianwu（archetypes + 样例）          │
└─────────────────────────────────────────────────────────────┘
```

---

## 4. 数据模型

### 4.1 Workspace 结构

```
my-library/                         # workspace root（用户起的名）
  .jianwu/
    config.yaml                     # per-workspace 配置
    schema_version                  # workspace schema 版本
  books/
    <book-slug>/
      meta.json
      outline.json
      grill.log
      .session.json                 # 问诊会话状态（运行中才有）
      chapters/
        01.md                       # 第一部 header + intro
        01-01.md                    # 第一部第一章
        01-02.md
        02.md
        02-01.md
        ...
  exports/                          # 导出产物（git-ignore）
    <book-slug>/
      zhurongshuo/, hugo/, pdf/, ...
  archive/                          # 归档的旧版本 book（可选）
```

### 4.2 Book 文件格式

#### meta.json

```json
{
  "id": "uuid-v7",
  "slug": "reality-of-time",
  "title": "时间的实在",
  "subtitle": "optional",
  "archetype": "ontology-epistemology-practice",
  "parameters": {
    "audience": "educated-general",
    "depth": "advanced",
    "goal": "understanding",
    "length": "long"
  },
  "language": "zh",
  "status": "draft",
  "created_at": "2026-06-21T14:30:00Z",
  "updated_at": "2026-06-21T15:45:00Z",
  "engine": {
    "jianwu_version": "0.1.0",
    "archetype_library_version": "v1",
    "grill_tree_version": "v1",
    "style_guide_version": "v1",
    "samples_version": "v1"
  }
}
```

#### outline.json

```json
{
  "parts": [
    {
      "index": 1,
      "title": "...",
      "role": "ontology",
      "intro": "...",
      "chapters": [
        {
          "index": 1,
          "title": "...",
          "abstract": "...",
          "key_concepts": ["..."],
          "learning_objectives": ["..."],
          "suggested_examples": ["..."],
          "claims": [],
          "status": "scaffolded",
          "word_count_target": 3000,
          "word_count": 0,
          "citations_count": 0,
          "unverified_claims": 0,
          "coherence_score": null,
          "expanded_with": null,
          "reviewed_at": null,
          "reviewed_by": null
        }
      ]
    }
  ]
}
```

#### chapters/NN-MM.md（脚手架阶段）

```markdown
---
part: 1
chapter: 1
title: ...
status: scaffolded
---

<!-- scaffolding placeholder -->
{{abstract}}

{{key_concepts}}
```

#### chapters/NN-MM.md（展开后）

```markdown
---
part: 1
chapter: 1
title: ...
status: expanded
word_count: 3124
---

# ...

正文段落...

正文段落...[^1]

## 引用

[^1]: [Title](https://...) accessed 2026-06-21
```

### 4.3 状态工作流

```
scaffolded → expanded → reviewed → final
                                      │
                                      ▼
                              （可回退到 expanded）
```

- `scaffolded`：仅 outline.json 有结构，章节正文为占位
- `expanded`：章节已展开成稿，含引用
- `reviewed`：人工核阅过
- `final`：定稿，可导出

`jianwu finalize <slug>` 要求全书章节都是 reviewed 才能转 final。

---

## 5. 引擎设计

### 5.1 4 阶段 LLM 编排

| 阶段 | 模式 | 状态 | 默认模型 | 备选 |
|---|---|---|---|---|
| **Grill（问诊）** | LLM 驱动的交互式对话，按设计树走 | 会话级 stateful | GLM-4.6 | Gemini 2.5 Pro |
| **Outline（大纲草稿）** | 单次 batch LLM 调用，纯函数风格 | 无状态 | Gemini 2.5 Pro | GLM-4.6 |
| **Scaffolding（每章脚手架）** | 并行 batch LLM 调用，N 章 N 并发 | 无状态 | Gemini 2.5 Flash | GLM air/mini |
| **Expand（展开成稿）** | 受控 agent loop + 工具 | 命令级 stateful | GLM-4.6（默认）/ Gemini Pro（高质） | — |

### 5.2 Grill 阶段：设计树

6 个核心维度（必问，依赖驱动顺序）：

| # | 维度 | 依赖 | AI 推荐策略 |
|---|---|---|---|
| 1 | 核心问题（这本书回答什么问题） | — | 从用户初始需求抽取并改写为一句话 |
| 2 | 受众 | #1 | 从主题推断 |
| 3 | 目标（理解/实操/决策） | #2 | 受众推断 |
| 4 | 结构原型 | #3 | 目标推断；同时查参考语料 |
| 5 | 深度（入门/进阶/专家） | #2 | 受众推断 |
| 6 | 篇幅（short/medium/long） | #4+#5 | 原型+深度推断 |

6 个条件维度（触发条件满足才问）：

| # | 维度 | 触发条件 |
|---|---|---|
| 7 | 范围（单本/卷/章） | 初始需求未明说 |
| 8 | 语言（中/英/双语） | 默认 zh；明确要求时确认 |
| 9 | 例子类型（案例/思想实验/数据） | 实证性主题 |
| 10 | 引用风格（学术/通俗/无） | 受众=专家 或 学术主题 |
| 11 | 可视化（图/表/无） | 系统/数据/流程主题 |
| 12 | 时效（永恒/当下/前瞻） | 技术/政策主题 |

设计树本身写在 `data/grill-tree.yaml`，方便后续增删。

### 5.3 LLM Provider 抽象

```go
type Provider interface {
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    Stream(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
    Tools(ctx context.Context, req ToolRequest) (*ToolResponse, error)
    Embed(ctx context.Context, inputs []string) ([][]float32, error)
}
```

v0.1 实现：`gemini.Provider` + `glm.Provider`。

模型映射在配置文件里，按阶段独立：

```yaml
# ~/.config/jianwu/config.yaml
models:
  intake:
    provider: glm
    model: glm-4.6
    fallback: { provider: gemini, model: gemini-2.5-pro }
  outline:
    provider: gemini
    model: gemini-2.5-pro
    fallback: { provider: glm, model: glm-4.6 }
  scaffolding:
    provider: gemini
    model: gemini-2.5-flash
    fallback: { provider: glm, model: glm-4-air }
  expand:
    provider: glm
    model: glm-4.6
    fallback: { provider: gemini, model: gemini-2.5-pro }
```

API keys 不入 workspace，只在 `~/.config/jianwu/secrets.yaml`（不入库）或环境变量 `GEMINI_API_KEY` / `GLM_API_KEY`。

Prompt caching：Gemini 用 context cache API；GLM 客户端层做静态 prefix hash 比对的简易缓存。

失败自动 fallback（可配置关闭）。

### 5.4 Expand 阶段：受控 Agent Loop

工具集：

| 工具 | 用途 |
|---|---|
| `web_search(query, opts)` | 调 search provider |
| `read_url(url)` | 调 URL Reader 拉 clean markdown |
| `lookup_similar_book(topic, archetype)` | 基于 embedding 检索参考语料 top-3 |
| `read_adjacent_chapters(part, chapter)` | 读相邻章节保证连贯 |
| `save_draft(content, citations)` | 持久化当前草稿 |
| `propose_revision()` | 让 LLM 自评并提议修订 |

Agent 单章展开的迭代上限：默认 3 次（研究→草稿→校验→修订）。

### 5.5 Web Search 集成

```go
type SearchProvider interface {
    Search(ctx context.Context, query string, opts SearchOpts) ([]SearchResult, error)
}

type URLReader interface {
    Read(ctx context.Context, url string) (Content, error)
}

type SearchOpts struct {
    MaxResults    int
    TimeRange     string  // past_year / past_month / etc.
    Language      string  // zh / en
    IncludeContent bool   // true 时同时调 Reader
}
```

v0.1 实现：
- Brave Search API（主，2000 queries/mo 免费）
- Serper.dev（备，2500 注册免费 credits）
- Jina Reader（URL 内容，1M tokens/mo 免费）

失败处理：
- search 超时/限流 → 切 fallback provider
- URL reader 失败 → citation 标 `unverified`，LLM 知道仅有 snippet
- 全失败 → 章节标 status=failed，提示重试

---

## 6. 原型库（Archetype Library）

### 6.1 Schema

```yaml
schema_version: 1
id: ontology-epistemology-practice
name:
  zh: 本体-认识-实践型
  en: Ontology-Epistemology-Practice
description: |
  适用于需要在基础层面建立深度理解的领域。
when_to_use:
  goals: [understanding]
  topic_types: [foundational, philosophical, scientific]
  audience_fit: [scholar, advanced-practitioner]
  not_recommended_for: [practical-playbook, quick-reference]
parts:
  - role: ontology
    title_template:
      zh: "第{n}部：{topic}的本体——{subtitle}"
      en: "Part {n}: The Ontology of {topic} — {subtitle}"
    guidance: |
      这一部要回答"这个领域里根本性的存在是什么"。
      避免：实操步骤、案例堆砌、技术细节。
    typical_chapters: [3, 5]
    chapter_role_hints:
      - 定义核心概念
      - 划定边界（什么不是 X）
      - 给出最小完备集
  - role: epistemology
    ...
  - role: order
    ...
  - role: practice
    ...
examples:
  - slug: reality-construction
    source: zhurongshuo
    fit_score: 0.95
  - slug: advancement-of-reality
    source: zhurongshuo
    fit_score: 0.92
metadata:
  extracted_from: zhurongshuo-corpus-v1
  extracted_at: 2026-06-21
  author: jianwu+human
```

### 6.2 v0.1 原型列表

先做 3 个（v0.1）：

| # | ID | 中文名 | 适用场景 |
|---|---|---|---|
| 1 | `ontology-epistemology-practice` | 本体-认识-实践型 | 基础理解/哲学 |
| 2 | `diagnosis-decoding-breakthrough` | 诊断-解码-破局型 | 问题解决/批判分析 |
| 3 | `foundations-application-practice` | 基础-应用-实战型 | 技术/教学 |

v0.1.x 增量补：

| # | ID | 中文名 | 适用场景 |
|---|---|---|---|
| 4 | `micro-meso-macro` | 微观-中观-宏观型 | 系统分析/层次解构 |
| 5 | `theory-dynamics-history-present` | 理论-动力-历史-当下型 | 历史/文明/演化 |
| 6 | `mindset-method-practice` | 心法-方法-实战型 | 技能/心法传授 |

v1.0 留：

| # | ID | 中文名 | 适用场景 |
|---|---|---|---|
| 7 | `theory-empire-digital-future` | 理论-经验-抵抗-未来型 | 权力/信任/社会结构（4-5 部大长篇） |
| 8 | `survival-winner-mindset` | 生存-赢家-心法型 | 战略/生存/竞争（与 #6 有重叠，可能合并） |

### 6.3 存放位置

- 内置 defaults：`internal/archetypes/`（Go embed）
- 用户覆盖：`~/.config/jianwu/archetypes/`
- 加载顺序：用户目录 > 内置（同名时用户覆盖）

---

## 7. 参考语料管理

### 7.1 分层结构

| 层 | 位置 | 内容 |
|---|---|---|
| 内置最小 | `internal/corpus/builtin/`（embed） | 每原型 1-2 本代表书的 outline 摘要 + few-shot 样例段落 |
| 扩展集 | `~/.local/share/jianwu/corpus/` | 通过 `jianwu corpus sync` 同步（v0.2） |
| Embedding 索引 | `~/.local/share/jianwu/corpus/index/` | 用于相似检索 |

加载顺序：扩展集 > 内置。

### 7.2 萃取物范围（不复制全文）

每本参考书存：
- outline 摘要（parts/chapters 标题 + 1-paragraph abstract）
- few-shot 样例段落（每原型 3-5 段，每段 200-500 字）
- 元数据（slug / archetype / audience / depth）
- source 归因（`source: zhurongshuo`, `source_url: ...`）

### 7.3 v0.1 范围

仅内置最小语料，不做 `corpus sync`。`lookup_similar_book` 工具基于 embedding 在内置语料上跑。

### 7.4 v0.2+

```
jianwu corpus list                  # 列出当前可用语料
jianwu corpus sync --from <path>    # 从外部源同步
jianwu corpus show <slug>           # 查看某本参考书的摘要
jianwu corpus stats                 # 语料统计
jianwu corpus reindex               # 重建 embedding 索引
```

---

## 8. 风格与质量

### 8.1 风格规约（style-guide.md）

位置：`internal/style/style-guide.md`（内置）+ `~/.config/jianwu/style-guide.md`（用户覆盖）

内容：
- 硬规则（违反就重写）：不使用"首先让我们"、不堆 emoji、中文不用"哦/呢/啦"等
- 软偏好（倾向但不强制）：长句优于短句堆叠、术语首次出现给定义
- 反例（zhurongshuo 不写的样子）

v0.1：AI 基于 zhurongshuo 现有文本反推萃取 → 用户审阅修改。

### 8.2 Few-shot 样例段落

位置：`internal/style/samples/<archetype-id>.md`

每原型 3-5 段 zhurongshuo 代表性段落（200-500 字/段）。在大纲和展开阶段注入 prompt。

### 8.3 事实可信度

- **强制引用**：展开阶段每个事实性陈述必须有 `[source: URL/书名]`，无引用标 `unverified_claims++`
- **Citation 用 `[^N]` 脚注语法**：每条含 id/url/title/accessed_at/snippet/used_in_paragraph/search_provider/reader_provider
- **人工签发**：status workflow `scaffolded → expanded → reviewed → final`
- **自动事实复核**（v0.2）：claims 抽取 + 验证 agent，接口在 `outline.json` 的 `claims[]` 字段已预留

### 8.4 章节级质量信号字段

```json
{
  "status": "expanded",
  "word_count": 3124,
  "word_count_target": 3000,
  "citations_count": 8,
  "unverified_claims": 2,
  "coherence_score": null,
  "expanded_with": {
    "provider": "glm",
    "model": "glm-4.6",
    "tools_used": ["web_search", "lookup_similar_book"],
    "iterations": 2,
    "tokens": { "in": 12450, "out": 3124 }
  },
  "reviewed_at": null,
  "reviewed_by": null
}
```

---

## 9. 配置系统

### 9.1 Config 分层（优先级从高到低）

1. 命令行参数 `--model glm-4.6` 等
2. 环境变量 `JIANWU_OUTLINE_MODEL=glm-4.6`
3. Workspace `.jianwu/config.yaml`
4. 用户全局 `~/.config/jianwu/config.yaml`
5. 内置 defaults（编译时）

### 9.2 Workspace 配置示例

```yaml
# <workspace>/.jianwu/config.yaml
schema_version: 1

models:
  intake:    { provider: glm,    model: glm-4.6 }
  outline:   { provider: gemini, model: gemini-2.5-pro }
  scaffolding: { provider: gemini, model: gemini-2.5-flash }
  expand:    { provider: glm,    model: glm-4.6 }

search:
  primary: brave
  fallback: serper
  reader: jina

archetypes:
  library: [builtin, user]   # 加载顺序

style:
  guide: [user, builtin]
  samples: [builtin]
```

### 9.3 全局 secrets

```yaml
# ~/.config/jianwu/secrets.yaml（不入库）
gemini_api_key: ...
glm_api_key: ...
brave_api_key: ...
serper_api_key: ...
jina_api_key: ...
```

---

## 10. CLI 命令

### 10.1 v0.1 命令集

```
jianwu init [path]                          # 初始化 workspace
jianwu init --bare                          # 已有目录改造，不创建 books/
jianwu info                                 # 显示 workspace 状态

jianwu config get <key>                     # 读配置
jianwu config set <key> <value>             # 写配置
jianwu config list                          # 列出所有配置（含来源）

jianwu new                                  # 启动交互式问诊，产出新 book
jianwu list                                 # 列出 workspace 内所有 book
jianwu show <slug>                          # 显示 book 的 outline + meta
jianwu status <slug>                        # 显示各章状态

jianwu expand <slug> <NN-MM>                # 展开单章成稿
jianwu review <slug> <NN-MM>                # 标记章节为 reviewed
jianwu finalize <slug>                      # 全书 reviewed 后转 final

jianwu export <slug> --target md --out <path>   # 仅 markdown 合并导出
```

### 10.2 v0.2 增补

```
jianwu rewrite <slug> <NN-MM>               # 重写已展开章节
jianwu add-chapter <slug> --after <NN-MM> --topic "..."
jianwu move-chapter <slug> <NN-MM> <target-part>

jianwu corpus list / sync / show / stats / reindex

jianwu export <slug> --target zhurongshuo --out <path>
jianwu export <slug> --target hugo --out <path>

jianwu migrate                              # workspace schema 升级
```

---

## 11. v0.1 MVP 范围

### 11.1 必做

- Workspace：`init` / `info` / `config get/set/list`
- Book 创建：`new`（完整 grill-me → 原型 → 大纲 → 脚手架）
- Book 查看：`list` / `show` / `status`
- 章节展开：`expand`（单章，web search + 引用）
- 状态工作流：`review` / `finalize`
- 原型库：3 个 v0.1 原型（先做最常用）
- Provider：Gemini + GLM 两个实现
- 参考语料：内置最小
- 风格：style-guide + few-shot 样例
- LLM 编排：4 阶段全跑通
- Web search：Brave + Serper + Jina
- 引用追踪：citations + unverified_claims
- 导出：仅 markdown 合并

### 11.2 砍到 v0.2

- 章节迭代命令（rewrite / add-chapter / move-chapter）
- corpus sync 扩展语料
- 自动事实复核
- Workspace migration 工具
- 多个 export target（zhurongshuo / hugo）
- 后 3 个原型（micro-meso-macro / theory-dynamics-history-present / mindset-method-practice）

### 11.3 成功标准

从 0 开始，用 `jianwu new` 走完一遍 grill-me 问诊，拿到一本完整的大纲+脚手架书；挑一章 `jianwu expand` 展开成带引用的成稿；`jianwu review` 标记；`jianwu finalize` 定稿。整个流程跑通，输出质量"配得上 zhurongshuo 同书架"。

---

## 12. 路线图

### v0.1 — 私人引擎（当前）

最小可用完整闭环。仅祝融自用。

### v0.2 — 让 v0.1 真正好用

- 章节迭代命令
- 语料扩展（corpus sync）
- Embedding 相似检索落地
- 自动事实复核
- Workspace migration
- 多 export target
- 后 3 个原型

### v1.0 — mouqin SaaS 化

- mouqin web app（前后端）
- 多用户 / 鉴权 / 账单
- 公开 book 分享链接
- 在线 grill-me（web 版交互）
- 多 book 卷管理
- 协作功能（评论、共享 workspace）

### v2+ — 长期

- 本地模型支持（Qwen3 / Ollama）
- 第三方原型库市场
- 多语言翻译流水线集成
- Plugin system

---

## 13. 前置工作（写代码前必须做）

### 13.1 原型库萃取（B + D：AI 辅助 + 先 3 个）

**流程：**

1. 写一次性脚本 `scripts/extract-archetypes.go`，读 zhurongshuo 的：
   - `data/books.yaml` + `data/practices.yaml`
   - 所有书的 `content/books/*/*/part-*/**` 和 `content/practices/*/**`

   > 注：此脚本从未实现；实际用 Claude Code 直接萃取（见 docs/EXTRACTION_NOTES.md §2.6）。`scripts/` 目录已于 v0.1.1-post 删除。
2. 把所有 part_titles + 章节标题 + 部分章节摘要喂给 LLM，要求：
   - 归纳出 3 个最常用结构原型
   - 每原型含：id/name.{zh,en}/description/when_to_use/parts[]（含 role/title_template/guidance/typical_chapters/chapter_role_hints）/examples[]
3. LLM 输出 YAML，用户审阅修订
4. 把定稿原型 + 对应代表书的 outline 摘要 + few-shot 段落，复制进 `internal/archetypes/` 和 `internal/corpus/builtin/`
5. 脚本完成后保留作 `jianwu corpus sync` 的实现基础

**先做的 3 个原型：**
- `ontology-epistemology-practice`（参考：reality-construction, advancement-of-reality）
- `diagnosis-decoding-breakthrough`（参考：silent-games, forced-convergence）
- `foundations-application-practice`（参考：ai-engineer-in-action, intelligent-computing-center-construction-guide）

### 13.2 风格规约萃取

**流程：**

1. 同样基于 zhurongshuo 现有正文，让 LLM 反推：
   - 硬规则（句式禁忌、词汇禁忌、标点偏好）
   - 软偏好（句长倾向、术语处理、段落结构）
   - 5-10 条反例（zhurongshuo 不写的样子）
2. 产出 `internal/style/style-guide.md`
3. 用户审阅修改（这步不可省，AI 反推会有偏差）

### 13.3 Few-shot 样例段落萃取

**流程：**

1. 对 3 个先做原型，每原型从对应参考书里抽 3-5 段代表性段落（200-500 字/段）
2. 段落选择标准：能体现原型该部（part）的写作风格、术语使用、论证方式
3. 产出 `internal/style/samples/<archetype-id>.md`
4. 用户审阅

---

## 14. 决策日志

完整决策记录（46 条），按主题分组：

### 14.1 受众与定位

1. 受众：C — 先做祝融私人引擎（v0.1），后续开放为公开 SaaS（v1.0）。引擎优先。

### 14.2 输出与粒度

2. 输出粒度：D — 默认"大纲+脚手架"，可对单章触发"展开成稿"。
3. 知识源：D — 可插拔；脚手架阶段用参数化知识，展开阶段默认开 web search 锚定。
4. 结构来源：D — jianwu 自己的原型库，初始用 zhurongshuo 25+ 本书萃取 seed。
5. 生成单位：D — v0.1 只做单本。
6. 内容类型：C — 单类型"book" + length(short/medium/long)。

### 14.3 交互与流程

7. 问诊方式：grill-me 模式 — 按设计树一个一个问，每问带推荐答案。
8. 设计树：6 核心 + 6 条件维度。

### 14.4 技术栈与架构

9. 技术栈：Go。独立仓库。jianwu = 库+CLI，mouqin = Web SaaS。
10. 与 zhurongshuo 关系：完全独立。
11. 文件结构：B — 目录 + 多文件扁平（meta.json + outline.json + grill.log + chapters/NN-MM.md）。
12. Schema：jianwu 原生字段，不借 zhurongshuo。
13. CLI 命令前缀：`jianwu`。

### 14.5 LLM 编排

14. LLM 编排：C — 分阶段混合。
15. LLM 选型：D — Gemini + GLM 两家 provider，从 day 1 做实。
16. 分档：问诊 GLM-4.6 / 大纲 Gemini 2.5 Pro / 脚手架 Gemini 2.5 Flash / 展开 GLM-4.6（默认）或 Gemini Pro（高质）。
17. Prompt caching：Gemini 用 context cache，GLM 客户端层做静态 prefix hash 简易缓存。
18. Provider 抽象：Chat / Stream / Tools / Embedding。
19. 模型映射配置驱动，每阶段独立配 + 备选，失败自动 fallback（可关）。

### 14.6 原型库

20. 原型库 schema：D — schema_version 字段版本化。
21. v0.1 原型库：6 个（v0.1 先做 3，v0.1.x 补 3）。
22. 原型库存储：B — 内置 + 用户覆盖。
23. 原型库萃取：B + D — AI 辅助 + 先 3 个。

### 14.7 质量与风格

24. 事实可信度：D — v0.1 做"强制引用" + 人工签发；自动事实复核留 v0.2。
25. 风格：D — 风格规约 + few-shot 样例。
26. 风格规约：AI 基于 zhurongshuo 反推萃取 + 用户审阅。
27. Citation 用 `[^N]` 脚注语法。
28. 章节质量信号字段：status / word_count / citations_count / unverified_claims / coherence_score(v0.1=null) / expanded_with / reviewed_at/by。

### 14.8 参考语料

29. 参考语料管理：D — 分层（内置最小 + sync 扩展）。
30. 萃取物范围：outline 摘要 + few-shot 段落 + 元数据，不含全文。
31. 相似检索：v0.1 就做 embedding。
32. corpus 子命令集：list / sync / show / stats / reindex。
33. 独立性检验：zhurongshuo 消失时 jianwu 仍能正常运行。

### 14.9 Workspace

34. Workspace：B — 显式 workspace。
35. Workspace 结构：.jianwu/ + books/ + exports/ + archive/。
36. Config 分层：CLI > env > workspace > 全局 > 内置。
37. API keys 不入 workspace，只在全局或环境变量。
38. Workspace 命令：init / config / info。schema 版本化。

### 14.10 Web Search

39. Web search：D — 多 search provider + URL reader 组合。
40. 免费优先：Brave（主）+ Serper（备）+ Jina Reader（URL 内容）。
41. 接口：SearchProvider.Search / URLReader.Read。
42. 失败处理：search 切 fallback；reader 失败标 unverified；全失败标 status=failed。

### 14.11 MVP 范围

43. v0.1 切分：核心闭环（new→expand→review→finalize）；多 export target / 自动复核 / corpus sync / 章节迭代命令都砍到 v0.2。
44. v0.1 成功标准：从 0 跑通完整闭环，输出质量配得上 zhurongshuo 同书架。

### 14.12 路线图

45. v0.2：章节迭代、corpus sync、embedding 检索、自动复核、多 export target、后 3 原型。
46. v1.0：mouqin SaaS；v2+：本地模型、第三方原型库、翻译流水线、plugin system。

---

## 15. 待解决问题（写代码时再定）

这些是实施层面的细节，不需要现在锁定：

- 错误处理 / 重试策略 / circuit breaker
- 日志 / 可观测性 / metrics
- 测试策略（unit / integration / e2e）
- 分发方式（homebrew / scoop / 直接下载 / go install）
- 文档站点（是否做 docs.jianwu.io）
- 许可证选择（MIT / Apache 2.0 / GPL / 商业）
- 国际化（CLI 输出中英文切换）
- CI/CD 流水线

---

## 附录 A：术语表

| 术语 | 含义 |
|---|---|
| **book** | jianwu 的唯一内容单位（不区分 books/practices/posts） |
| **workspace** | 一个 `.jianwu/` 目录标记的工作区，含多本 book |
| **archetype** | 结构原型，决定 book 的 parts 角色与排序逻辑 |
| **scaffolding** | 脚手架——outline.json + 章节占位 |
| **expand** | 把脚手架章节展开为带引用的成稿 |
| **grill-me** | 问诊模式，按设计树带推荐问诊 |
| **design tree** | 6 核心 + 6 条件维度的问诊决策树 |
| **provider** | LLM / search / URL reader 的抽象接口实现 |
| **corpus** | 参考语料库，内置 + sync 扩展两层 |
| **promote / finalize** | 状态工作流的人工签发动作 |
