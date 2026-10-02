# jianwu 功能概览

> 开发版本：0.3.12（尚未发布） | 最后更新：2026-10-03

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
| `export <slug> [--target md\|hugo\|pdf\|epub]` | v0.1.3 | 导出全书（markdown / Hugo / PDF / EPUB3） |
| `publish <slug> [--dry-run] [--major] [--version X.Y]` | v0.3.11 | 发布不可变版本（Release，见下节） |
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
| `site [--out] [--base] [--dry-run]` | v0.3.11 | 从已发布 release 生成静态阅读站 + RSS/OPDS（见"静态阅读站"节） |
| `mcp` | v0.3.12 | 以 MCP stdio 服务器运行，16 个工具覆盖全管线（见 [Agent 接入](AGENT_ACCESS.md)） |
| `skill [--dir] [--stdout]` | v0.3.12 | 安装/打印 agent 技能文件 SKILL.md（工作流+闸门规则） |
| `serve [--addr] [--token]` | v0.3.6-dev | 启动本地 Web UI + HTTP API（见下节；token 见 Agent 接入） |

---

## Web UI 与 HTTP API（`jianwu serve`）

`jianwu serve` 在本地启动一个 Web 服务（默认 `127.0.0.1:8787`，仅绑定 localhost），内嵌单页应用覆盖全部 CLI 功能：

- **工作区配置**：根目录按 `--dir` flag > 环境变量 `JIANWU_WORKSPACE` > 全局配置 `~/.config/jianwu/config.yaml` 的 `workspace:` 键 > 启动目录解析；Web 页面可随时切换（持久化到全局配置文件，CLI 共享）。**未配置/未初始化工作区时，新建图书、生成、展开、修订等操作被拦截**，页面引导先完成配置。
- **工作区仪表盘**：初始化工作区、书籍列表与进度条、累计 Token 用量、配置摘要。
- **新建图书向导**：12 维 grill 访谈逐题呈现，AI 推荐一键接受/修改/skip；完成后一键生成大纲与章节框架。
- **书籍详情**：分 part 章节表（状态/字数/引用/未验证论断/核查结论）；逐章展开、重写、事实核查、修订、审阅、删除、插入；展开全部、定稿、导出 md/hugo/pdf/epub 并下载产物。
- **章节阅读**：markdown 渲染、脚注、引用来源、事实核查结论（含建议改写）。
- **语料管理**：列表/详情/统计、目录同步、重建 embedding 索引。
- **任务面板**：所有长耗时操作以后台任务执行，实时进度、日志、取消。

HTTP API 位于 `/api/v1/`（workspace / config / books / chapters / grill / corpus / jobs），可作为本地 API 集成点；CLI 与引擎层不受影响。工作区切换端点为 `POST /api/v1/workspace/select {path}`。

**可靠性模型：** 服务为单进程、可切换工作区。所有写操作经单一后台任务队列串行执行（同书不并发写入约束不变）；长任务进度可轮询（`GET /api/v1/jobs/{id}`），进程终止时未保存的生成结果仍会丢失（与 CLI 相同）。访谈/事实核查/修订只装配所需 provider——访谈不需要搜索 API key。

---

## 发布（Release 模型，v0.3.11）

`publish` 把已定稿的图书固化为不可变的版本化发布包（[ADR 29](decisions/29-publishing-layer.md) 出版层第 1 步）：

```
books/<slug>/releases/<MAJOR.MINOR>/
  manifest.json     # 版本、内容 sha256、claims/verdicts 统计、（可选）EPUB 产物哈希
  provenance.json   # 模型与用量、AI 生成披露、人工 review 时间线、去重来源清单
  outline.json      # 状态快照（claims/verdicts/citations 真相源）
  meta.json         # 状态快照
  content/NN-MM.md  # 定稿章节副本
```

- **发布硬门**：全书与全部章节 `final` + `meta.json` 的 `license` 字段非空 + 目标版本号未占用。finalize 后任何 revise/expand 会把书打回 draft，自动重新武装发布门（新 edition 必须重新 review）。
- **警告不拦截**：未核验论断、未通过 verdict、缺 reviewed_by 署名会显示并记入 manifest（披露原则），不阻止发布。
- **版本推导**：首个版本 1.0；结构变化（part 数或任一部章节数）→ 升 major，内容修订 → 升 minor；`--major` 强制、`--version X.Y` 显式（必须大于现有版本）。
- **原子落位**：先写 `releases/.staging-<version>/`，完整后改名就位；失败清理 staging，历史版本不受影响。已发布版本不可覆盖。
- **来源审计**：provenance 的去重来源清单（URL + 访问时间）是商用前人工 license 审计的依据。
- serve 侧：`POST /api/v1/books/{slug}/publish`（`dry_run` 同步返回门报告与下一版本）、`GET /api/v1/books/{slug}/releases`（版本列表）；Web UI 书籍详情页有发布入口与版本历史。

---

## 静态阅读站与 OPDS（v0.3.11）

`jianwu site` 从**已发布 release** 生成静态阅读站（`<workspace>/site/`，`--out` 可改），可直接部署到任意静态托管。工作稿绝不上架——分发渠道只读 Release（ADR 29）：

- **书架页** `index.html`：书名/副题/作者/版本/日期/未核验论断数，附 OPDS 订阅入口。
- **书页** `<slug>/index.html`：colophon（许可 + AI 生成披露）、历史版本、目录、EPUB 下载（附 sha256 摘要）。
- **章节页** `<slug>/ch-NN-MM.html`：正文 + 脚注 + 与 EPUB **逐字一致**的"来源与核验"节（复用同一渲染器），上一章/下一章导航。
- **OPDS** `opds.xml`：OPDS 1.x acquisition feed；阅读器 App 可发现并直接下载 `epub/<slug>.epub`（release artifact 逐字节副本）。
- **RSS** `rss.xml`：RSS 2.0 发布订阅源（v0.3.12）——每版本一个 item，EPUB 以 enclosure 附带，描述含未核验论断数披露；`--base <url>` 使 RSS/OPDS 链接绝对化。
- **确定性**：时间戳全部来自 manifest，同一书架状态重复生成字节相同；`site/` 为派生状态，每次整体重建。损坏的 release 跳过并报告，不中断生成。
- serve 侧：`POST /api/v1/site/generate`（`dry_run` 同步返回书架清单；实际生成走任务队列）。

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
    meta.json                # 图书元数据（含累计 token_usage、author/license）
    outline.json             # 目录 + 章节状态（含 Verdicts[]）
    .session.json            # grill 已完成会话（audit log）
    chapters/NN-MM.md        # 展开后的章节（YAML frontmatter + markdown）
    export/                  # export 输出（md/hugo/pdf）
    releases/<X.Y>/          # publish 产出的不可变发布版本
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
| `--target epub` | EPUB 3（纯 Go，无外部工具链；v0.3.11） |

脚注在跨章导出时自动全局重编号（epub 为按章编号、章末 EPUB3 aside）。

**EPUB3（v0.3.11，ADR 29 第 2 步）：** `dc:identifier` 用建书时的 UUID（`urn:uuid:`，跨导出稳定）；`dcterms:modified` 取 `Meta.UpdatedAt` 而非墙钟，zip 条目固定顺序与时间戳——同一书籍状态产出**字节相同**的 .epub（`publish` 的 manifest 产物哈希即依赖此性质）。每章包含"来源与核验"节：来源列表（URL + 访问日期）与论断核验表（✓ 来源支持 / ✗ 未通过 / 未核验 / 无引用，核验说明折叠展示）——溯源能力随发布直达读者。正文原始 HTML 一律转义，输出保持 XHTML 良构。封面约定 `books/<slug>/cover.png|jpg`，存在则作为 cover-image。`books/<slug>/releases/<X.Y>/artifact/` 内的 EPUB 与 `export --target epub` 字节一致。

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
