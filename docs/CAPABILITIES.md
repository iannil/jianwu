# jianwu 功能概览

> 开发版本：0.3.6-dev（尚未发布） | 最后更新：2026-09-27

---

## CLI 命令

所有命令均支持全局标志 `-L`/`--verbose`、`--debug`、`-d`/`--dir`（指定 workspace 根目录）。

| 命令 | 版本 | 说明 |
|---|---|---|
| `init [--bare] [path]` | v0.1.0 | 初始化 workspace |
| `info` | v0.1.0 | 工作区诊断信息 |
| `config get/set/list` | v0.1.0 | 配置查询与修改 |
| `new [--force]` | v0.1.0 | 完整创作流程：grill → outline → scaffolding |
| `scaffolding <slug> --retry-failed` | v0.3.9 | 只重试框架失败的章节（`new` 后的恢复路径） |
| `expand <slug> <NN-MM> [--force]` | v0.1.1 | 单章展开（research → draft → validate，支持 streaming） |
| `review <slug> <NN-MM>` | v0.1.3 | 标记章节为已审阅 |
| `finalize <slug> [--dry-run]` | v0.1.3 | 全书定稿 |
| `export <slug> [--target md\|hugo\|pdf]` | v0.1.3 | 导出全书（markdown / Hugo / PDF） |
| `status <slug>` | v0.1.3 | 章节进度概览 + 下一步提示 |
| `factcheck <slug> <NN-MM>` | v0.2.0 | 自动事实复核 |
| `revise <slug> <NN-MM>` | v0.2.0 | 基于事实复核结果修订章节 |
| `rewrite <slug> <NN-MM>` | v0.2.2 | 重写章节（等价于 expand --force --force） |
| `add-chapter <slug> --after <NN-MM> --topic "..."` | v0.2.2 | 插入新章节 |
| `move-chapter <slug> <NN-MM> <target-part>` | v0.2.2 | 移动章节到其他 part |
| `delete-chapter <slug> <NN-MM>` | v0.2.2 | 删除章节 |
| `expand --all <slug>` | v0.2.2 | 批量展开全书 |
| `corpus list/show/stats` | v0.2.3 | 查看参考语料列表/详情/统计 |
| `corpus sync --from <path>` | v0.2.3 | 从本地 JSON 目录导入语料 |
| `corpus collect --topic "..."` | v0.3.6-dev | 自动采集：搜索 → 阅读 → LLM 提取 → 保存 → 重建索引 |
| `corpus reindex` | v0.2.3 | 重建 embedding 索引（调用 embedder） |
| `serve [--addr]` | v0.3.6-dev | 启动本地 Web UI + HTTP API（见下节） |

---

## Web UI 与 HTTP API（`jianwu serve`）

`jianwu serve` 在本地启动一个 Web 服务（默认 `127.0.0.1:8787`，仅绑定 localhost），内嵌单页应用覆盖全部 CLI 功能：

- **工作区配置**：根目录按 `--dir` flag > 环境变量 `JIANWU_WORKSPACE` > 全局配置 `~/.config/jianwu/config.yaml` 的 `workspace:` 键 > 启动目录解析；Web 页面可随时切换（持久化到全局配置文件，CLI 共享）。**未配置/未初始化工作区时，新建图书、生成、展开、修订等操作被拦截**，页面引导先完成配置。
- **工作区仪表盘**：初始化工作区、书籍列表与进度条、累计 Token 用量、配置摘要。
- **新建图书向导**：12 维 grill 访谈逐题呈现，AI 推荐一键接受/修改/skip；完成后一键生成大纲与章节框架。
- **书籍详情**：分 part 章节表（状态/字数/引用/未验证论断/核查结论）；逐章展开、重写、事实核查、修订、审阅、删除、插入；展开全部、定稿、导出 md/hugo/pdf 并下载产物。
- **章节阅读**：markdown 渲染、脚注、引用来源、事实核查结论（含建议改写）。
- **语料管理**：列表/详情/统计、目录同步、重建 embedding 索引。
- **任务面板**：所有长耗时操作以后台任务执行，实时进度、日志、取消。

HTTP API 位于 `/api/v1/`（workspace / config / books / chapters / grill / corpus / jobs），可作为本地 API 集成点；CLI 与引擎层不受影响。工作区切换端点为 `POST /api/v1/workspace/select {path}`。

**可靠性模型：** 服务为单进程、可切换工作区。所有写操作经单一后台任务队列串行执行（同书不并发写入约束不变）；长任务进度可轮询（`GET /api/v1/jobs/{id}`），进程终止时未保存的生成结果仍会丢失（与 CLI 相同）。访谈/事实核查/修订只装配所需 provider——访谈不需要搜索 API key。

---

## 创作引擎

### 4 阶段管线

```
grill → outline → scaffolding → expand → factcheck → [revise → factcheck] → review → finalize → export
```

| 阶段 | 包 | 说明 |
|---|---|---|
| **Grill** | `engine/grill` | 12 维度设计决策树问诊。LLM 逐维推荐，用户接受/修改/跳过。stateful session 可 Ctrl+C 恢复。 |
| **Outline** | `engine/outline` | 单次 LLM 调用 + JSON Schema 强制输出，生成全书目录结构。 |
| **Scaffolding** | `engine/scaffolding` | N 章并行生成章节框架（errgroup，continue-on-error），产出章节目录 + 关键概念。 |
| **Expand** | `engine/expand` | 3 迭代 agent：① Research（web_search + read_url）→ ② Draft（注入 archetype + style + samples）→ ③ Validate（自检 + 修订）。产出带 `[^N]` 引用标记的 markdown。支持 streaming 输出。 |
| **Factcheck** | `engine/factcheck` | 按 claim 的 citation_ids 核对来源支持程度，结果写入 outline.json；未知与失败保留未通过结论。 |
| **Revise** | `engine/revise` | 基于 factcheck 的 `SuggestedRewrite`，LLM 修订未通过章节。 |

### 状态机

```
scaffolded → expanded → reviewed → final → export
```

`outline.json` 为状态真相源，.md frontmatter 镜像同步。

---

## Provider 抽象

### LLM（Chatter / Embedder / Streamer）

| Provider | 实现方式 | 用途 |
|---|---|---|
| Gemini | 官方 `google.golang.org/genai` SDK | outline、scaffolding |
| GLM | OpenAI-compatible REST + SSE（共享 `openaicomp` 层，json_schema 严格输出） | intake、expand |
| Kimi | OpenAI-compatible REST + SSE（api.moonshot.ai，K 系列支持 json_schema） | `kimi-k3` 等 |
| DeepSeek | OpenAI-compatible REST + SSE（json_object 模式；无 embeddings 端点） | `deepseek-chat` / `deepseek-reasoner`（仅对话阶段） |
| Ollama | HTTP REST（localhost:11434） | 本地模型（Qwen3 等） |
| Mock | 内存 mock | 单元测试 |

**可靠性：** Retry 3 次（指数退避 + jitter）→ fallback provider 兜底。每个阶段可独立配置模型和超时。支持 streaming 逐 token 输出。

### 搜索

| Provider | 用途 |
|---|---|
| Brave Search | 主要搜索 |
| Serper | 备用搜索 |

### 阅读器

| Provider | 用途 |
|---|---|
| Jina Reader | URL → markdown（含 SSRF 安全校验 + 10MB body 限制） |

### 工厂

`llmfactory` / `searchfactory` / `readerfactory` — 独立包避免 import cycle。Provider 显式注入；产品仍有进程级存储与配置默认值。

---

## 数据模型

### Workspace 结构

```
<jianwu-workspace>/
  .jianwu/
    config.yaml              # workspace 配置
    sessions/<id>.json       # grill 运行中会话
    corpus_index.json        # embedding 索引缓存（corpus reindex 生成）
  books/<slug>/
    meta.json                # 图书元数据（含累计 token_usage）
    outline.json             # 目录 + 章节状态（含 Verdicts[]）
    .session.json            # grill 已完成会话（audit log）
    chapters/NN-MM.md        # 展开后的章节（YAML frontmatter + markdown）
    export/                  # export 输出（md/hugo/pdf）
```

### 关键类型（`internal/book/types.go`）

- `Meta` — 图书元数据、archetype 选择、参数（受众/深度/目标/篇幅）
- `Outline` — 多 part 结构，每 part 含 chapters[]
- `OutlineChapter` — 标题、状态、字数、引用、`Verdicts[]`
- `ChapterFrontmatter` — 单章 YAML 头（状态/字数/模型/时间戳）
- `Claim` / `ClaimVerdict` — 声明 + 事实复核结果（含 `SuggestedRewrite`）
- `ClaimWhitelist` — 旧字段读取兼容，不再绕过来源验证

---

## 配置系统

**5 层合并（高 → 低优先级）：**

1. CLI flag（`--model glm-4.6`）
2. 环境变量（`JIANWU_OUTLINE_MODEL=...`）
3. Workspace `.jianwu/config.yaml`
4. 全局 `~/.config/jianwu/config.yaml`
5. 编译时默认值

**Secrets：** `~/.config/jianwu/secrets.yaml`（强制 0600 权限）或 ENV 变量（`GEMINI_API_KEY` / `GLM_API_KEY` / `KIMI_API_KEY` / `DEEPSEEK_API_KEY` / `BRAVE_API_KEY` 等）。ENV 高于文件；两者同时存在且值不同时输出告警（不改优先级）。SecretsProvider 是内部替换接口，不承诺多租户隔离。

**`models.embedder`（可选）：** 为语料索引 / similar-book 查找指定独立的 embedding provider。聊天 provider 无 embeddings 端点时（如 deepseek）配置它，即可用 glm/gemini/ollama 建索引而不影响聊天模型。缺省时从阶段模型推导（旧行为）。

---

## 数据资产（内置 embed）

- **6 个 archetype YAML：**
  - `ontology-epistemology-practice`（本体-认识-实践）
  - `diagnosis-decoding-breakthrough`（诊断-解码-破局）
  - `foundations-application-practice`（基础-应用-实战）
  - `micro-meso-macro`（宏-中-微）
  - `theory-dynamics-history-present`（理论-动力-历史-当下）
  - `mindset-method-practice`（心法-方法-实践）
- **风格指南** `style-guide.md`
- **6 个 few-shot samples**（对应各 archetype）
- **参考语料无内置**：全部来自工作区 `.jianwu/corpus/`（`corpus collect` 自动采集、`corpus sync` 导入或手写 JSON；outline 生成与 similar-book 查找消费）

---

## 导出目标

| 目标 | 说明 |
|---|---|
| `--target md` | 单文件 markdown（Pandoc 兼容 frontmatter，默认） |
| `--target hugo` | 章节分文件 Hugo content 结构（`_index.md` + 逐章文件） |
| `--target pdf` | 通过 pandoc + xelatex 自动生成 PDF |

脚注在跨章导出时自动全局重编号。

---

## 质量基础设施

- **30+ 测试包**全绿，`go vet` 干净，`go test -race` 全绿
- **Storage 接口**（`OS` + `MemStorage`）抽象文件 I/O
- **表格驱动测试**风格（`in`/`want` 命名用例，不用 testify）
- **错误分类：** `ErrNetwork` / `ErrRateLimit` / `ErrServer` → retry；`ErrLLMProvider` → 不重试
- **退出码：** 0 成功 / 1 通用 / 2 用法 / 3 workspace 未找到 / 4 LLM 错 / 5 网络错
- **SSRF 防护：** URL allowlist（仅 http/https，禁止 localhost/私有 IP）；`LimitReader`（10MB body + 4KB error body）

---

## 版本历史

| 版本 | 关键交付 |
|---|---|
| v0.1.0–0.1.6 | 核心管线（grill→outline→scaffolding→expand）+ 状态机 + streaming + fallback + 超时 |
| v0.2.0–0.2.3 | factcheck/revise + Ollama + 章节迭代命令 + corpus sync + embedding 索引 + 6 原型 + 10 语料 |
| v0.3.0–0.3.5 | Storage 接口 + 长任务进度模型 + Token 计量 + per-tenant Secrets + 并发安全装配 + SaaS 安全加固 |
| 0.3.6-dev | 独立产品、可靠批量写入、显式引用、累计已报告用量、本地发布 |

## 独立产品与可靠性边界

jianwu 不再服务 mouqin，内部引擎不承诺公共 SDK。批量展开最多 5 并发，任务结束后集中保存，失败退出非零。同书不支持跨进程并发写入。

new、expand（含 --all）、factcheck、revise 始终记录 provider 报告的 LLM 用量；--tokens 控制显示，status 展示累计。未报告部分标记不完整；不含搜索、阅读器、embedding 费用。旧书历史用量未知。

修订会作废旧审阅与 verdict，需要再次 factcheck/review。单文件原子替换和普通 I/O 回滚不等于跨文件崩溃事务。版本与发布见 [RELEASING](RELEASING.md)，真实样书评估见 [EVALUATION](EVALUATION.md)。
