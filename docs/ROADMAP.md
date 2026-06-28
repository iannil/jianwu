# jianwu 路线图

> 本文档跟踪 v0.1.0 之后的迭代计划。每个版本应该有明确范围、可验收标准、合理工作量。
> 最后更新：2026-06-28（v0.2.3 — corpus sync delivered）
>
> **注意：** 代码实际迭代已大幅超越文档。v0.1.4–v0.2.0 + Ollama + Storage 接口 + hugo/pdf 导出
> 已在一次大规模工程提交中全部实现。详见 `docs/PROJECT_STATUS.md`。

---

## 当前状态

**v0.1.x 全线贯通 + v0.2 多能力已交付：**

| 能力 | 版本对应 | 状态 |
|---|---|---|
| Expand CLI | v0.1.1 | ✅ 已交付 |
| Prompt 注入（archetype/style/samples/adjacent） | v0.1.2 | ✅ 已交付 |
| 状态机命令（review/finalize/export/status） | v0.1.3 | ✅ 已交付 |
| Fallback model wiring | v0.1.4 | ✅ 已交付 |
| LLM 超时 + Ctrl+C | v0.1.5 | ✅ 已交付 |
| Streaming 输出 | v0.1.6 | ✅ 已交付 |
| 自动事实复核（factcheck + revise） | v0.2.0 | ✅ 已交付 |
| Ollama 本地模型支持 | v0.2.1 | ✅ 已交付 |
| 章节迭代命令（rewrite/add-chapter/move-chapter/delete-chapter/expand --all） | v0.2.2 | ✅ 已交付 |
| Corpus sync（list/show/stats/sync/reindex） | v0.2.3 | ✅ 已交付 |
| Storage 接口地基 | v0.3.0 | ⏳ 地基已交付（接口 + OS + MemStorage，book/workspace 已迁移） |

---

## v0.2 — 功能扩展（剩余）

> 目标：把 v0.1 的"能跑"提升到"真好用"。factcheck/revise/Ollama/hugo/pdf 已交付。
> 章节迭代命令（rewrite/add-chapter/move-chapter/delete-chapter/expand --all）已交付。

### 章节迭代命令（✅ 已交付）

### Corpus Sync（✅ 已交付 — v0.2.3）

- [x] `jianwu corpus list` — 列出所有语料书
- [x] `jianwu corpus show <slug>` — 显示语料详情
- [x] `jianwu corpus stats` — 语料统计
- [x] `jianwu corpus sync --from <path>` — 从 zhurongshuo 同步 JSON
- [x] `jianwu corpus reindex` — 重建 embedding 索引（暂为 no-op）
- [x] `corpus.LoadWithWorkspace(wsRoot)` — 分层加载（workspace 覆盖 + builtin 回退）

- [x] `jianwu rewrite <slug> <NN-MM>` — 重写已 expand 章节
- [x] `jianwu add-chapter <slug> --after <NN-MM> --topic "..."`
- [x] `jianwu move-chapter <slug> <NN-MM> <target-part>`
- [x] `jianwu delete-chapter <slug> <NN-MM>`
- [x] `jianwu expand --all` — 批量扩展全书（决策 Q4=B）

### Corpus Sync（✅ 已交付 — v0.2.3）

- [x] `jianwu corpus list` — 列出所有语料书
- [x] `jianwu corpus show <slug>` — 显示语料详情
- [x] `jianwu corpus stats` — 语料统计
- [x] `jianwu corpus sync --from <path>` — 从 zhurongshuo 同步 JSON
- [x] `jianwu corpus reindex` — 使用 embedder 生成 embedding 索引
- [x] `corpus.LoadWithWorkspace(wsRoot)` — 分层加载（workspace 覆盖 + builtin 回退）

### Embedding 索引缓存（✅ 已交付 — v0.2.3）

- [x] `corpus.BuildIndex` — 为所有语料书生成 embedding 向量
- [x] `corpus.LoadIndex` / `corpus.SaveIndex` — 索引文件 I/O
- [x] `corpus.CorpusIndex.FindSimilar` — 余弦相似度相似搜索
- [x] `expand.ToolRegistry.LookupSimilarBook` — 懒加载缓存索引，避免实时调用 embedder
- [x] CLI 自动配置索引路径（`buildToolRegistry` 传递 wsRoot）

### Workspace Migration

Workspace migration 已取消。schema_version 校验已移除，不再需要迁移命令。

### 后 3 个原型

- [ ] `micro-meso-macro`（参考：data-as-the-boundary）
- [ ] `theory-dynamics-history-present`（参考：revisiting-history）
- [ ] `mindset-method-practice`（参考：open-map / barbaric-order）

---

## v0.3 — SaaS-ready 内核改造（mouqin 前置）

> 目标：把 jianwu 从"单用户 + 本地文件系统"改造成可被多租户 Web 服务**安全嵌入**的库。
>
> **为什么单列一个里程碑：** v1.0 的 mouqin 表面是 web 前后端 + 鉴权 + 账单，但真正的前置工作在 **jianwu 侧**——
> `Storage` 接口已做地基，但 secrets 仍是全局单文件、expand 阻塞无进度回调、
> `ChatResponse` 不含 token usage、provider 装配靠全局可变 var。
> 这一里程碑补内核能力，**不含任何 web UI**（那是 v1.0 mouqin 的事）。
>
> 顺序按依赖：存储抽象（v0.3.0）已部分落地，按剩余顺序推进。

### v0.3.0 — 存储抽象（⏳ 已交付地基）

- [x] `Storage` 接口（ReadFile/WriteFile/MkdirAll/RemoveAll/Rename/Stat/ReadDir）
- [x] `OS` 文件系统实现（默认）
- [x] `MemStorage` 内存实现（测试用）
- [x] book / workspace / config / cli / grill 已迁移到 `storage.OS`
- [ ] per-tenant 命名空间隔离
- [ ] 预留对象存储（S3 等）实现点

### v0.3.1 — 长任务 / 进度模型

- [ ] `expand.Generate` 暴露进度事件（research / draft / validate 阶段 + 每次工具调用）
- [ ] 全程 ctx 可取消、状态可恢复
- [ ] `scaffolding.ScaffoldAll` 暴露 per-chapter 进度
- [ ] 设计成可被任务队列驱动

### v0.3.2 — Token / 成本计量

- [ ] `ChatResponse` 加 `Usage{PromptTokens, CompletionTokens, TotalTokens}`
- [ ] expand / outline / scaffolding 汇总 per-call token + 估算成本
- [ ] 每本书累计 token 记账

### v0.3.3 — per-tenant Secrets

- [ ] `LoadSecrets` 接口化，支持注入
- [ ] 支持 per-tenant key
- [ ] CLI 路径保持 ENV + `~/.config/jianwu/secrets.yaml` 行为不变

### v0.3.4 — 并发安全的 provider 装配

- [ ] 把 `cli.chatterProviderHook` + `cli.providerDepsHook` 全局可变 var 重构为显式注入
- [ ] 确认引擎与 CLI 层无全局可变状态
- [ ] 迁移现有 E2E 测试到注入模式

**验收：** `go test -race ./...` 全绿；并发跑多个独立 book 任务互不串扰。

### v0.3.5 — SaaS 安全加固

- [ ] Search / Reader 的 BaseURL allowlist（防 SSRF）
- [ ] Jina `io.ReadAll` 改 `LimitReader`
- [ ] Search / Reader 错误消息截断
- [ ] Citation / 外部 URL 做 SSRF 校验

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

- [ ] 第三方原型库市场
- [ ] 多语言翻译流水线集成
- [ ] Plugin system

---

## 决策原则

新功能优先级评估：

1. **核心闭环阻断**（v0.1.x）：用户能用 jianwu 跑完整流程吗？
2. **质量瓶颈**（v0.2）：当前输出质量是否配得上 zhurongshuo 同书架？
3. **SaaS-ready 内核**（v0.3）：jianwu 能否被多租户 Web 安全嵌入？
4. **规模扩展**（v1+）：能否服务更多用户？

不做的：

- 不做 jianwu 自己的 web UI（v1.0 的 mouqin 才做）
- 不做不支持 zh / en 之外语言的 i18n
- 不做本地 GUI（CLI + 未来 web 已够）
