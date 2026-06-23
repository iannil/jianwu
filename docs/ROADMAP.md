# jianwu 路线图

> 本文档跟踪 v1.0.0 之后的迭代计划。每个版本应该有明确范围、可验收标准、合理工作量。
> 最后更新：2026-06-22（v1.0.0 shipped；v1.0.x 审计重排）
>
> **范围说明**：v1.0.0 tag (2026-06-22) 是过早的——ship 时的实际范围是库 API + new CLI，
> 未含用户可用的 expand CLI 与 zhurongshuo 风格注入。v1.0.x 的目标是把 v1.0 承诺真正补齐。
> v1.0.5 ship 后才视作 v1.0 真正交付。详见 `docs/decisions/26-grill-decisions.md` § v1.0.x 完成度审计决策。

---

## 当前状态

**v1.0.0 已 tag（范围过早）**：完整 4 阶段引擎库 + `jianwu new` 命令。v1.0.x 把 CLI 扩到真正可用。详见 `PROJECT_STATUS.md`。

---

## v1.0.x — 让 v1.0 真正名副其实

> 目标：把 v1.0 承诺（用户能从 CLI 跑出 zhurongshuo 风格的章节）真正补齐。
> v1.0.5 ship 后视为 v1.0 真正交付。切片顺序按依赖（决策 Q21=A）。

### v1.0.1 — Expand CLI（**已交付** 2026-06-23）

**范围：** 加 `jianwu expand <slug> <NN-MM>` 命令，调 `expand.Generate`，写 `chapters/NN-MM.md`。

**任务（详见 `docs/plans/2026-06-22-v1.0.1-expand-cli.md`）：**
- [x] `ProviderDeps` + `providerDepsHook` 在 `internal/cli/providers.go`（决策 Q20=B）
- [x] `internal/book/chapter.go`：ChapterFrontmatter + WriteChapter + ReadChapter（决策 Q2=B）
- [x] `buildToolRegistry` helper（v1.0.1 outlineFn 是 stub）
- [x] CLI 命令 `expand` 在 `internal/cli/expand.go`，含 `--force` 语义（决策 Q3=B）
- [x] 调 `expand.Generate`，写 chapter file（frontmatter + markdown）
- [x] 同时更新 `outline.json` 的 chapter status + citations + word_count + unverified_claims
- [x] E2E test 用 `providerDepsHook` 注入 mock
- [x] Live integration test（SKIP if no API keys）

**验收：** `jianwu new` 之后 `jianwu expand <slug> 01-01` 能产出 `books/<slug>/chapters/01-01.md`。

### v1.0.2 — Expand Prompt 注入（**最关键**）

**范围：** 把 archetype YAML + style samples + similar book + adjacent chapters 真正注入 expand prompts（当前全是占位符）。

**为什么是 P0：** 当前 expand 输出是 generic LLM markdown，不是 zhurongshuo 风格。这是 v1.0.0 承诺"配得上 zhurongshuo 同书架"的核心缺口（决策 Q16=C）。

**任务：**
- [ ] expand.Generate 加载 archetype YAML（基于 in.ArchetypeID）
- [ ] expand.Generate 调 `style.LoadSamples()` 取对应 archetype 的 samples
- [ ] iter_research 调 `LookupSimilarBook` 取 zhurongshuo 类似书片段
- [ ] iter_draft 调 `ReadAdjacentChapter` 取前一章节选段
- [ ] prompt 模板 render 时用真实数据替换 `iter_draft.go:25-26` 的占位符
- [ ] Live integration test：跑 3 章并人工审阅"像不像 zhurongshuo"

**验收：** zhurong 读完后说"这是 zhurongshuo 风格"，而不是"这是 generic LLM 输出"。

### v1.0.3 — 状态机命令

**范围：** review + finalize + export + status 命令（决策 Q7=R1+F1+X1、Q8=A、Q9=C）。

**任务：**
- [ ] `jianwu review <slug> <NN-MM>` — expanded → reviewed（其他状态拒绝）
- [ ] `jianwu finalize <slug>` — 全书 reviewed 后转 final（含 `--dry-run`）
- [ ] `jianwu export <slug> --target md --out <path>` — 合并 chapters 为单 markdown（含 `--dry-run`）
- [ ] `jianwu status <slug>` — 显示各章状态 + unverified_claims

**验收：** 完整闭环 `new → expand → review → finalize → export` 端到端跑通。

### v1.0.4 — Fallback Model Wiring

**范围：** 全局单一 fallback（决策 Q10=A）。

**任务：**
- [ ] `config.ModelRef` 加 `Fallback *ModelRef` 字段（顶层全局，不按阶段）
- [ ] `cli.buildChatter` 检测 Fallback，非空则 wrap with `FallbackWrapper`
- [ ] fallback provider == primary provider 时打 warning + 不装 wrapper
- [ ] 配置示例更新到 workspace 默认模板
- [ ] E2E test：primary 失败 → fallback 接管

**验收：** 配 primary=gemini-2.5-pro + fallback=glm-4.6，断网 Gemini 时自动切到 GLM。

### v1.0.5 — LLM Timeout

**范围：** 避免长调用 hang（决策 Q12=C，全局默认 + 阶段覆盖）。

**任务：**
- [ ] Config 加 `llm.timeout`（默认 90s）+ `models.<stage>.timeout` 覆盖
- [ ] expand 默认 600s（3 次 LLM + 工具调用）
- [ ] CLI 给每个 chatter.Chat 包 ctx.WithTimeout
- [ ] 用户 Ctrl+C 时立即取消（确认全局信号处理）

**验收：** LLM 卡 5 分钟，按阶段超时退出（grill 90s / expand 600s）。

**v1.0.5 ship = v1.0 promise fully delivered.**

### v1.0.6 — Streaming Output（可选 polish）

**范围：** 只 draft 流式（决策 Q11=D+B1）。

**任务：**
- [ ] `llm.Streamer` 接口
- [ ] Gemini + GLM 实现 SSE 流式
- [ ] `jianwu expand` 命令显示 draft token 流（research/validate 不流式，是 JSON）
- [ ] jianwu 不感知 streaming + caching 兼容性（SDK 自处理）

**验收：** `jianwu expand` 看到正文流式生成。

---

## v1.1 — 功能扩展

> 目标：把 v1.0 的"能跑"提升到"真好用"。多个独立功能，按价值排序做。

### v1.1.0 — 章节迭代命令

- [ ] `jianwu rewrite <slug> <NN-MM>` — 重写已 expand 章节
- [ ] `jianwu add-chapter <slug> --after <NN-MM> --topic "..."`
- [ ] `jianwu move-chapter <slug> <NN-MM> <target-part>`
- [ ] `jianwu delete-chapter <slug> <NN-MM>`
- [ ] `jianwu expand --all` — 批量扩展全书（决策 Q4=B，v1.0.x 不做）

### v1.1.1 — Corpus Sync

- [ ] `jianwu corpus list / show / stats`
- [ ] `jianwu corpus sync --from <path>` — 从 zhurongshuo 同步扩展语料
- [ ] `jianwu corpus reindex` — 重建 embedding 索引文件

### v1.1.2 — 自动事实复核

- [ ] Expand 后跑 claims 抽取 agent
- [ ] 每条 claim 跑独立 web search 验证
- [ ] outline.json 加 `verified_claims` / `disputed_claims` 字段

### v1.1.3 — Workspace Migration

- [ ] `jianwu migrate` 命令（schema v1 → v2，决策 Q18=C 引入结构性变更时）
- [ ] 检测旧 workspace + 升级

### v1.1.4 — 多 Export Target

- [ ] `--target zhurongshuo`（适配 zhurongshuo hugo 结构）
- [ ] `--target hugo`（通用 hugo 站点）
- [ ] `--target pdf`（pandoc 集成）

### v1.1.5 — 后 3 个原型

- [ ] `micro-meso-macro`（参考：data-as-the-boundary）
- [ ] `theory-dynamics-history-present`（参考：revisiting-history）
- [ ] `mindset-method-practice`（参考：open-map / barbaric-order）

### v1.1.6 — chatterProviderHook 重构（从代码债迁移）

- [ ] 把 `cli.chatterProviderHook` + `cli.providerDepsHook` 重构为 CLI struct field
- [ ] 迁移所有现有 E2E 测试到 struct 注入模式
- ~~决定 `book.Citation.UsedInParagraph` / `expand.ExpandOutput.Draft` 字段去留~~ — v1.0.1-post 已删除

---

## v2 — mouqin SaaS

> 目标：把 jianwu 包装成多用户 Web 服务。独立仓库 `mouqin`。

### v2.0 — mouqin MVP

- [ ] mouqin web app（前后端）
- [ ] 多用户 / 鉴权 / 账单（Stripe）
- [ ] 公开 book 分享链接
- [ ] 在线 grill-me（web 版交互）
- [ ] 部署 mouqin 直接 import jianwu 库（祝融是 copyright holder，不受 AGPL 自身约束）

### v2.x — 协作功能

- [ ] 多 book 卷管理
- [ ] 评论 / 评审
- [ ] 共享 workspace

---

## v3+ — 长期

- [ ] 本地模型支持（Qwen3 / Ollama）
- [ ] 第三方原型库市场
- [ ] 多语言翻译流水线集成
- [ ] Plugin system

---

## 决策原则

新功能优先级评估：

1. **核心闭环阻断**（v1.0.x）：用户能用 jianwu 跑完整流程吗？
2. **质量瓶颈**（v1.1）：当前输出质量是否配得上 zhurongshuo 同书架？
3. **规模扩展**（v2+）：能否服务更多用户？

不做的：

- 不做 jianwu 自己的 web UI（v2 的 mouqin 才做）
- 不做不支持 zh / en 之外语言的 i18n（v1 祝融自用 + 中文非虚构为主）
- 不做本地 GUI（CLI + 未来 web 已够）
