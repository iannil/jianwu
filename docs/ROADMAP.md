# jianwu 路线图

> 本文档跟踪 v0.1.0 之后的迭代计划。每个版本应该有明确范围、可验收标准、合理工作量。
> 最后更新：2026-06-23（v0.1.3 已 ship 完整闭环；新增 v0.3 SaaS-ready 内核改造里程碑）
>
> **范围说明**：v0.1.0 tag (2026-06-22) 是过早的——ship 时的实际范围是库 API + new CLI，
> 未含用户可用的 expand CLI 与 zhurongshuo 风格注入。v0.1.x 的目标是把 v0.1 承诺真正补齐。
> v0.1.5 ship 后才视作 v0.1 真正交付。详见 `docs/decisions/26-grill-decisions.md` § v0.1.x 完成度审计决策。

---

## 当前状态

**v0.1.3 已 ship（2026-06-23）**：完整 4 阶段引擎 + 完整 CLI 闭环 `new → expand → review → finalize → export`，prompt 注入已补齐。剩 v0.1.4–v0.1.6 收尾后视为 v0.1 真正交付。详见 `PROJECT_STATUS.md`。

---

## v0.1.x — 让 v0.1 真正名副其实

> 目标：把 v0.1 承诺（用户能从 CLI 跑出 zhurongshuo 风格的章节）真正补齐。
> v0.1.5 ship 后视为 v0.1 真正交付。切片顺序按依赖（决策 Q21=A）。

### v0.1.1 — Expand CLI（**已交付** 2026-06-23）

**范围：** 加 `jianwu expand <slug> <NN-MM>` 命令，调 `expand.Generate`，写 `chapters/NN-MM.md`。

**任务（详见 `docs/plans/2026-06-22-v0.1.1-expand-cli.md`）：**
- [x] `ProviderDeps` + `providerDepsHook` 在 `internal/cli/providers.go`（决策 Q20=B）
- [x] `internal/book/chapter.go`：ChapterFrontmatter + WriteChapter + ReadChapter（决策 Q2=B）
- [x] `buildToolRegistry` helper（v0.1.1 outlineFn 是 stub）
- [x] CLI 命令 `expand` 在 `internal/cli/expand.go`，含 `--force` 语义（决策 Q3=B）
- [x] 调 `expand.Generate`，写 chapter file（frontmatter + markdown）
- [x] 同时更新 `outline.json` 的 chapter status + citations + word_count + unverified_claims
- [x] E2E test 用 `providerDepsHook` 注入 mock
- [x] Live integration test（SKIP if no API keys）

**验收：** `jianwu new` 之后 `jianwu expand <slug> 01-01` 能产出 `books/<slug>/chapters/01-01.md`。

### v0.1.2 — Expand Prompt 注入（**已交付** 2026-06-23）

**范围：** 把 archetype YAML + style samples + adjacent chapters 真正注入 expand prompts（原先全是占位符）。

**为什么是 P0：** 原先 expand 输出是 generic LLM markdown，不是 zhurongshuo 风格。这是 v0.1.0 承诺"配得上 zhurongshuo 同书架"的核心缺口（决策 Q16=C）。

**任务：**
- [x] expand.Generate 加载 archetype YAML（基于 in.ArchetypeID）
- [x] expand.Generate 调 `style.LoadSamples()` 取对应 archetype 的 samples
- [x] iter_draft 调 `ReadAdjacentChapter` 取前一章节选段
- [x] prompt 模板 render 时用真实数据替换原占位符
- [x] Live integration test：跑 3 章并人工审阅"像不像 zhurongshuo"
- [ ] ~~iter_research 调 `LookupSimilarBook`~~ — 切出独立切片（embedding 检索增强，依赖 Embedder），见 v0.2.1

**验收：** zhurong 读完后说"这是 zhurongshuo 风格"（剩人读终验）。

### v0.1.3 — 状态机命令（**已交付** 2026-06-23）

**范围：** review + finalize + export + status 命令（决策 Q7=R1+F1+X1、Q8=A、Q9=C）。

**任务：**
- [x] `jianwu review <slug> <NN-MM>` — expanded → reviewed（其他状态拒绝）
- [x] `jianwu finalize <slug>` — 全书 reviewed 后转 final（含 `--dry-run`）
- [x] `jianwu export <slug>` — 合并 chapters 为单 markdown，全局脚注重编号（含 `--dry-run`；写 `books/<slug>/export/<slug>.md`）
- [x] `jianwu status <slug>` — 显示各章状态 + 下一步动作提示

**验收：** 完整闭环 `new → expand → review → finalize → export` 端到端跑通。✅

### v0.1.4 — Fallback Model Wiring

**范围：** 全局单一 fallback（决策 Q10=A）。

**任务：**
- [ ] `config.ModelRef` 加 `Fallback *ModelRef` 字段（顶层全局，不按阶段）
- [ ] `cli.buildChatter` 检测 Fallback，非空则 wrap with `FallbackWrapper`
- [ ] fallback provider == primary provider 时打 warning + 不装 wrapper
- [ ] 配置示例更新到 workspace 默认模板
- [ ] E2E test：primary 失败 → fallback 接管

**验收：** 配 primary=gemini-2.5-pro + fallback=glm-4.6，断网 Gemini 时自动切到 GLM。

### v0.1.5 — LLM Timeout

**范围：** 避免长调用 hang（决策 Q12=C，全局默认 + 阶段覆盖）。

**任务：**
- [ ] Config 加 `llm.timeout`（默认 90s）+ `models.<stage>.timeout` 覆盖
- [ ] expand 默认 600s（3 次 LLM + 工具调用）
- [ ] CLI 给每个 chatter.Chat 包 ctx.WithTimeout
- [ ] 用户 Ctrl+C 时立即取消（确认全局信号处理）

**验收：** LLM 卡 5 分钟，按阶段超时退出（grill 90s / expand 600s）。

**v0.1.5 ship = v0.1 promise fully delivered.**

### v0.1.6 — Streaming Output（可选 polish）

**范围：** 只 draft 流式（决策 Q11=D+B1）。

**任务：**
- [ ] `llm.Streamer` 接口
- [ ] Gemini + GLM 实现 SSE 流式
- [ ] `jianwu expand` 命令显示 draft token 流（research/validate 不流式，是 JSON）
- [ ] jianwu 不感知 streaming + caching 兼容性（SDK 自处理）

**验收：** `jianwu expand` 看到正文流式生成。

---

## v0.2 — 功能扩展

> 目标：把 v0.1 的"能跑"提升到"真好用"。多个独立功能，按价值排序做。

### v0.2.0 — 章节迭代命令

- [ ] `jianwu rewrite <slug> <NN-MM>` — 重写已 expand 章节
- [ ] `jianwu add-chapter <slug> --after <NN-MM> --topic "..."`
- [ ] `jianwu move-chapter <slug> <NN-MM> <target-part>`
- [ ] `jianwu delete-chapter <slug> <NN-MM>`
- [ ] `jianwu expand --all` — 批量扩展全书（决策 Q4=B，v0.1.x 不做）

### v0.2.1 — Corpus Sync

- [ ] `jianwu corpus list / show / stats`
- [ ] `jianwu corpus sync --from <path>` — 从 zhurongshuo 同步扩展语料
- [ ] `jianwu corpus reindex` — 重建 embedding 索引文件

### v0.2.2 — 自动事实复核

- [ ] Expand 后跑 claims 抽取 agent
- [ ] 每条 claim 跑独立 web search 验证
- [ ] outline.json 加 `verified_claims` / `disputed_claims` 字段

### v0.2.3 — Workspace Migration

- [ ] `jianwu migrate` 命令（schema v1 → v2，决策 Q18=C 引入结构性变更时）
- [ ] 检测旧 workspace + 升级

### v0.2.4 — 多 Export Target

- [ ] `--target zhurongshuo`（适配 zhurongshuo hugo 结构）
- [ ] `--target hugo`（通用 hugo 站点）
- [ ] `--target pdf`（pandoc 集成）

### v0.2.5 — 后 3 个原型

- [ ] `micro-meso-macro`（参考：data-as-the-boundary）
- [ ] `theory-dynamics-history-present`（参考：revisiting-history）
- [ ] `mindset-method-practice`（参考：open-map / barbaric-order）

### v0.2.6 — chatterProviderHook 重构（从代码债迁移）

> **已并入 v0.3.4**：并发安全的 provider 装配是 SaaS 化的硬前提，不再单列为 v0.2 代码债，提到 v0.3 里程碑。

- [ ] 把 `cli.chatterProviderHook` + `cli.providerDepsHook` 重构为显式注入（见 v0.3.4）
- ~~决定 `book.Citation.UsedInParagraph` / `expand.ExpandOutput.Draft` 字段去留~~ — v0.1.1-post 已删除

---

## v0.3 — SaaS-ready 内核改造（mouqin 前置）

> 目标：把 jianwu 从"单用户 + 本地文件系统"改造成可被多租户 Web 服务**安全嵌入**的库。
>
> **为什么单列一个里程碑：** v1.0 的 mouqin 表面是 web 前后端 + 鉴权 + 账单，但真正的前置工作在 **jianwu 侧**——
> 当前代码全程假设"单用户 + 本地"：`workspace/` 与 `book/` 有 12 处直接 `os.WriteFile/ReadFile/MkdirAll`、
> secrets 是全局单文件、expand 是阻塞几分钟且无进度回调的调用、`ChatResponse` 不含 token usage、
> provider 装配靠全局可变 var。若不先在 jianwu 把这些长出来，mouqin 一开工就被迫回头改底层 I/O 与装配，
> 是隐藏的工期黑洞。这一里程碑专门补这层内核能力，**不含任何 web UI**（那是 v1.0 mouqin 的事）。
>
> 顺序按依赖：存储抽象（v0.3.0）是其余几项的地基，建议先做。

### v0.3.0 — 存储抽象

- [ ] 抽 `Storage` 接口（read / write / list / delete + 路径命名空间），替换 `workspace/` + `book/` 的 12 处直接 `os.*` 文件调用
- [ ] 默认 filesystem 实现，保持现有 CLI 行为与 workspace 布局完全不变
- [ ] per-tenant 命名空间隔离：workspace 不再假设单一本地根目录
- [ ] 预留对象存储（S3 等）实现点，供 mouqin 接入

**验收：** CLI 行为零变化；同一套 book 读写逻辑能跑在 filesystem 与一个内存/对象存储 mock 上。

### v0.3.1 — 长任务 / 进度模型

- [ ] `expand.Generate` 暴露进度事件（research / draft / validate 阶段 + 每次工具调用）——回调或 channel
- [ ] 全程 ctx 可取消、状态可恢复（衔接 v0.1.5 超时）
- [ ] `scaffolding.ScaffoldAll` 暴露 per-chapter 进度
- [ ] 设计成可被任务队列（mouqin worker）驱动，而非只在 HTTP 请求内阻塞

**验收：** 一个非 CLI 调用方能订阅 expand 的逐阶段进度并中途取消；scaffolding 能报告 N 章实时进度。

### v0.3.2 — Token / 成本计量

- [ ] `ChatResponse` 加 `Usage{PromptTokens, CompletionTokens, TotalTokens}`，Gemini / GLM 各自透出
- [ ] expand / outline / scaffolding 汇总 per-call token + 估算成本
- [ ] 每本书累计 token 记账（outline.json 或独立 ledger）——计费与成本护栏的基础

**验收：** 跑完一章 expand 能拿到准确 token 数与成本估算。

### v0.3.3 — per-tenant Secrets

- [ ] `LoadSecrets` 从全局单文件改为可注入的 secrets provider（接口化）
- [ ] 支持 per-tenant key，或 平台统一 key + 用量按租户归属
- [ ] CLI 路径保持 ENV + `~/.config/jianwu/secrets.yaml` 行为不变

**验收：** 库调用方能为每次请求注入不同租户的 key，CLI 行为不变。

### v0.3.4 — 并发安全的 provider 装配（吸收 v0.2.6）

- [ ] 把 `cli.chatterProviderHook` + `cli.providerDepsHook` 全局可变 var 重构为显式注入（struct / 参数）
- [ ] 确认引擎与 CLI 层无全局可变状态，可在并发请求下安全复用
- [ ] 迁移现有 E2E 测试到注入模式

**验收：** `go test -race ./...` 全绿；并发跑多个独立 book 任务互不串扰。

### v0.3.5 — SaaS 安全加固（PROJECT_STATUS §11）

- [ ] Search / Reader 的 BaseURL allowlist（防 SSRF）
- [ ] Jina `io.ReadAll` 改 `LimitReader`（防超大响应 DoS）
- [ ] Search / Reader 错误消息截断，不回显完整 response body
- [ ] Citation / 外部 URL 做 SSRF 校验

**验收：** 这 4 项 PROJECT_STATUS §11 标注"v1.0 SaaS 必修"的安全债清零。

---

## v1.0 — mouqin SaaS

> 目标：把 jianwu 包装成多用户 Web 服务。独立仓库 `mouqin`。
> **前置：** 依赖 v0.3 SaaS-ready 内核（存储 / 任务 / 计量 / 并发 / 安全）落地后再开工。

### v1.0 — mouqin MVP

- [ ] mouqin web app（前后端）
- [ ] 多用户 / 鉴权 / 账单（Stripe）
- [ ] 公开 book 分享链接
- [ ] 在线 grill-me（web 版交互）
- [ ] 部署 mouqin 直接 import jianwu 库（祝融是 copyright holder，不受 AGPL 自身约束）

### v1.x — 协作功能

- [ ] 多 book 卷管理
- [ ] 评论 / 评审
- [ ] 共享 workspace

---

## v2+ — 长期

- [ ] 本地模型支持（Qwen3 / Ollama）
- [ ] 第三方原型库市场
- [ ] 多语言翻译流水线集成
- [ ] Plugin system

---

## 决策原则

新功能优先级评估：

1. **核心闭环阻断**（v0.1.x）：用户能用 jianwu 跑完整流程吗？
2. **质量瓶颈**（v0.2）：当前输出质量是否配得上 zhurongshuo 同书架？
3. **SaaS-ready 内核**（v0.3）：jianwu 能否被多租户 Web 安全嵌入？（存储抽象 / 长任务进度 / token 计量 / 并发安全 / 安全加固）
4. **规模扩展**（v1+）：能否服务更多用户？

不做的：

- 不做 jianwu 自己的 web UI（v1.0 的 mouqin 才做）
- 不做不支持 zh / en 之外语言的 i18n（v0.1 祝融自用 + 中文非虚构为主）
- 不做本地 GUI（CLI + 未来 web 已够）
