# jianwu 路线图

> 本文档跟踪 v1.0.0 之后的迭代计划。每个版本应该有明确范围、可验收标准、合理工作量。
> 最后更新：2026-06-22（v1.0.0 shipped）

---

## 当前状态

**v1.0.0 已交付（tag 在 master）**：完整 4 阶段引擎 + `jianwu new` 命令闭环。详见 `PROJECT_STATUS.md`。

---

## v1.0.x — 让 v1.0 真正好用

> 目标：把 v1.0 的库 API 全部接到 CLI，让用户不用写 Go 代码就能完整跑通 new → expand → review → finalize → export。

### v1.0.1 — Expand CLI

**范围：** 加 `jianwu expand <slug> <NN-MM>` 命令，调 `expand.Generate`，写 `chapters/NN-MM.md`。

**任务（参考 S7 计划）：**
- [ ] CLI 命令 `expand` 在 `internal/cli/expand.go`
- [ ] 装配 ToolRegistry（search + reader + embedder + outline callback）
- [ ] 调 `expand.Generate`，写 chapter file（frontmatter + markdown）
- [ ] 同时更新 `outline.json` 的 chapter status + citations + word_count + unverified_claims
- [ ] E2E test：mock chatter + 验证 chapter 文件产生

**验收：** `jianwu new` 之后 `jianwu expand <slug> 01-01` 能产出 `books/<slug>/chapters/01-01.md`。

### v1.0.2 — Review + Finalize + Export

**范围：** 状态工作流的剩余 3 个命令。

**任务：**
- [ ] `jianwu review <slug> <NN-MM>` — 标 chapter.status = reviewed
- [ ] `jianwu finalize <slug>` — 全书 reviewed 后转 final
- [ ] `jianwu export <slug> --target md --out <path>` — 合并 chapters 为单 markdown
- [ ] `jianwu status <slug>` — 显示各章状态（之前漏了）

**验收：** 完整闭环 `new → expand → review → finalize → export` 端到端跑通。

### v1.0.3 — Fallback Model Wiring

**范围：** Config 加 fallback 字段 + 自动装配 FallbackWrapper。

**任务：**
- [ ] `config.ModelRef` 加 `Fallback *ModelRef` 字段
- [ ] `cli.buildChatter` 检测 Fallback，非空则 wrap with `FallbackWrapper`
- [ ] 配置示例更新到 README + workspace 默认模板
- [ ] E2E test：primary 失败 → fallback 接管

**验收：** 配 primary=gemini-2.5-pro + fallback=glm-4.6，断网 Gemini 时自动切到 GLM。

### v1.0.4 — Streaming Output

**范围：** Grill + Expand 长时间运行时 token 流式输出。

**任务：**
- [ ] `llm.Streamer` 接口（之前预留，S2 注释里）
- [ ] Gemini + GLM 实现流式（SSE）
- [ ] `TerminalPrompt.Ask` + expand 显示流式 token
- [ ] 测试：mock stream + verify 显示

**验收：** `jianwu new` 答问题时看到推荐答案逐字显示；`jianwu expand` 看到正文流式生成。

### v1.0.5 — LLM Timeout + Cancellation

**范围：** 避免长调用 hang。

**任务：**
- [ ] Config 加 `models.<stage>.timeout` 字段（默认 90s）
- [ ] CLI 给每个 chatter.Chat 包 ctx.WithTimeout
- [ ] 用户 Ctrl+C 时立即取消（ctx-aware 已经做了，但要确认全局信号处理）

**验收：** LLM 卡 5 分钟，90s 后命令报超时退出。

---

## v1.1 — 功能扩展

> 目标：把 v1.0 的"能跑"提升到"真好用"。多个独立功能，按价值排序做。

### v1.1.0 — 章节迭代命令

- [ ] `jianwu rewrite <slug> <NN-MM>` — 重写已 expand 章节
- [ ] `jianwu add-chapter <slug> --after <NN-MM> --topic "..."`
- [ ] `jianwu move-chapter <slug> <NN-MM> <target-part>`
- [ ] `jianwu delete-chapter <slug> <NN-MM>`

### v1.1.1 — Corpus Sync

- [ ] `jianwu corpus list / show / stats`
- [ ] `jianwu corpus sync --from <path>` — 从 zhurongshuo 同步扩展语料
- [ ] `jianwu corpus reindex` — 重建 embedding 索引文件

### v1.1.2 — 自动事实复核

- [ ] Expand 后跑 claims 抽取 agent
- [ ] 每条 claim 跑独立 web search 验证
- [ ] outline.json 加 `verified_claims` / `disputed_claims` 字段

### v1.1.3 — Workspace Migration

- [ ] `jianwu migrate` 命令（schema v1 → v2）
- [ ] 检测旧 workspace + 升级

### v1.1.4 — 多 Export Target

- [ ] `--target zhurongshuo`（适配 zhurongshuo hugo 结构）
- [ ] `--target hugo`（通用 hugo 站点）
- [ ] `--target pdf`（pandoc 集成）

### v1.1.5 — 后 3 个原型

- [ ] `micro-meso-macro`（参考：data-as-the-boundary）
- [ ] `theory-dynamics-history-present`（参考：revisiting-history）
- [ ] `mindset-method-practice`（参考：open-map / barbaric-order）

### v1.1.6 — Expand Prompt 增强

- [ ] 注入 archetype YAML 到 expand prompts（当前占位符）
- [ ] 注入 style samples（当前占位符）
- [ ] 注入 read_adjacent_chapters 结果（工具已就绪，未调用）

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
