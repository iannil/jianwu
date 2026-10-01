# jianwu 路线图

> 本文档跟踪 v0.1.0 之后的迭代计划。每个版本应该有明确范围、可验收标准、合理工作量。
> 最后更新：2026-06-30（v0.3 审计后调整：v0.3 重定义为 single-tenant，v0.4 推迟到触发式启动，新增 v0.3.6）
>
> **注意：** v0.1.x–v0.3.5 已全部交付。下一步：v0.3.6（发布流程 + Token 扩展 + 测试补全）→ mouqin MVP（单租户）→ v0.4（触发式多租户）。

---

## v0.3 审计调整（2026-06-30）

v0.3.5 ship 后整体审计发现 v0.3 "SaaS-ready" 表述与实现有偏差：3 个全局可变 var（`secretsProvider` / `DefaultStorage` / `cliWorkspaceDir`）+ `LoadSecretsFor(_)` stub 与"多租户安全嵌入"目标冲突。

**调整决定（详见 `docs/decisions/27-v0.3-audit-decisions.md`）：**

- v0.3 重定义为 **"single-tenant SaaS-ready"**（启动期注入安全、运行期不可变）
- v0.4 多租户接线**改为触发式启动**（mouqin 上线后真实需求触发，不进关键路径）
- 新增 **v0.3.6** 切片：发布流程 + Token 扩展 + 测试补全

---

## 当前状态

**全线贯通：v0.1.x → v0.2.x → v0.3.x 已全部交付：**

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
| Corpus sync + embedding 索引 | v0.2.3 | ✅ 已交付 |
| 后 3 个原型 + 4 本新语料 | v0.2.3 后 | ✅ 已交付 |
| Storage 接口（OS + MemStorage + 全量迁移） | v0.3.0 | ✅ 已交付 |
| 长任务进度模型（expand callback） | v0.3.1 | ✅ 已交付 |
| Token/成本计量（Usage 字段） | v0.3.2 | ✅ 已交付 |
| per-tenant Secrets（SecretsProvider 接口 + DI） | v0.3.3 | ✅ 已交付 |
| 并发安全 provider 装配（显式参数注入，-race 全绿） | v0.3.4 | ✅ 已交付 |
| SaaS 安全加固（SSRF allowlist / LimitReader / 错误截断） | v0.3.5 | ✅ 已交付 |

---

## v0.2 — 功能扩展（✅ 已全部交付）

> 目标：把 v0.1 的"能跑"提升到"真好用"。所有功能已交付。

### 章节迭代命令（✅ 已交付）

### Corpus Sync + Embedding 索引（✅ 已交付 — v0.2.3）

- [x] `jianwu corpus list` — 列出所有语料书
- [x] `jianwu corpus show <slug>` — 显示语料详情
- [x] `jianwu corpus stats` — 语料统计
- [x] `jianwu corpus sync --from <path>` — 从 zhurongshuo 同步 JSON
- [x] `jianwu corpus reindex` — 使用 embedder 生成 embedding 索引
- [x] `corpus.LoadWithWorkspace(wsRoot)` — 分层加载（workspace 覆盖 + builtin 回退）

- [x] `jianwu rewrite <slug> <NN-MM>` — 重写已 expand 章节
- [x] `jianwu add-chapter <slug> --after <NN-MM> --topic "..."`
- [x] `jianwu move-chapter <slug> <NN-MM> <target-part>`
- [x] `jianwu delete-chapter <slug> <NN-MM>`
- [x] `jianwu expand --all` — 批量扩展全书（决策 Q4=B）

### Embedding 索引缓存（✅ 已交付 — v0.2.3）

- [x] `corpus.BuildIndex` — 为所有语料书生成 embedding 向量
- [x] `corpus.LoadIndex` / `corpus.SaveIndex` — 索引文件 I/O
- [x] `corpus.CorpusIndex.FindSimilar` — 余弦相似度相似搜索
- [x] `expand.ToolRegistry.LookupSimilarBook` — 懒加载缓存索引，避免实时调用 embedder
- [x] CLI 自动配置索引路径（`buildToolRegistry` 传递 wsRoot）

### Workspace Migration

Workspace migration 已取消。schema_version 校验已移除，不再需要迁移命令。

### 后 3 个原型（✅ 已交付）

- [x] `micro-meso-macro`（参考：data-as-the-boundary）
- [x] `theory-dynamics-history-present`（参考：revisiting-history）
- [x] `mindset-method-practice`（参考：open-map / barbaric-order）
- [x] 3 个新 few-shot samples 各原型对应
- [x] 4 本新语料 JSON：barbaric-order / data-as-the-boundary / open-map / revisiting-history

---

## v0.3 — SaaS-ready 内核改造（✅ 已全部交付，mouqin 前置条件已满足）

> 目标：把 jianwu 从"单用户 + 本地文件系统"改造成可被多租户 Web 服务**安全嵌入**的库。
> **SaaS-ready 内核已全线交付。** v1.0 mouqin 前置条件满足。

### v0.3.0 — 存储抽象（✅ 已交付）

- [x] `Storage` 接口（ReadFile/WriteFile/MkdirAll/RemoveAll/Rename/Stat/ReadDir）
- [x] `OS` 文件系统实现（默认）
- [x] `MemStorage` 内存实现（测试用）
- [x] book / workspace / config / cli / grill 已迁移到 `storage.OS`
- [x] per-tenant 命名空间隔离（Storage.Namespace 预留）
- [x] 预留对象存储（S3 等）实现点

### v0.3.1 — 长任务 / 进度模型（✅ 已交付）

- [x] `expand.Generate` 暴露进度事件（research / draft / validate 阶段 + 每次工具调用）
- [x] 全程 ctx 可取消、状态可恢复
- [x] `scaffolding.ScaffoldAll` 暴露 per-chapter 进度
- [x] 设计成可被任务队列驱动

### v0.3.2 — Token / 成本计量（✅ 已交付）

- [x] `ChatResponse` 加 `Usage{PromptTokens, CompletionTokens, TotalTokens}`
- [x] expand / outline / scaffolding 汇总 per-call token + 估算成本
- [x] 每本书累计 token 记账

### v0.3.3 — per-tenant Secrets（✅ 已交付）

- [x] `LoadSecrets` 接口化，支持注入
- [x] 支持 per-tenant key
- [x] CLI 路径保持 ENV + `~/.config/jianwu/secrets.yaml` 行为不变

### v0.3.4 — 并发安全的 provider 装配 ✅

- [x] 全局可变 hook 已不存在（`runExpand`/`runNewFlow` 显式参数注入）
- [x] E2E 测试用 mock 构造后注入，无全局 var 覆盖
- [x] `go test -race ./...` 全绿 ✅

**验收通过：** `go test -race ./...` 全绿；无全局可变 provider 状态。

### v0.3.5 — SaaS 安全加固 ✅

- [x] Search / Reader 的 BaseURL allowlist — `reader.ValidateURL()` 仅允许 http/https、禁止 localhost/私有 IP/.local/.internal
- [x] Jina `io.ReadAll` 已用 `LimitReader`（10MB body + 4KB error body）
- [x] Search 错误消息截断 — Brave + Serper 均限制为 4KB + `truncateErrBody`
- [x] Citation / 外部 URL 做 SSRF 校验 — `reader.ValidateURL()` 集成到 Jina reader

---

## v0.3.6 — 发布流程 + Token 扩展 + 测试补全（**下一个切片**）

> v0.3.5 审计发现的实施层缺口集中处理。详见 `docs/decisions/27-v0.3-audit-decisions.md` F1-F3、F5。

- [ ] `jianwu --version` / `-v` flag + `jianwu version` 子命令
- [ ] `scripts/release.sh`：`git describe` 推 version → `go build -ldflags "-X ...Version=$V"` → 打 tag → push tag
- [ ] `Meta.TokenUsage` 字段持久化（meta.json）+ expand 命令累计 + `status` 显示
- [ ] `outline` / `scaffolding` 命令加 `--tokens` flag（复用 `TrackingChatter` wrapper）
- [ ] 补 `internal/engine/usage_test.go`（TokenTracker/TrackingChatter TDD）
- [ ] 补 `internal/engine/expand/progress_test.go` + `scaffolding` progress test
- [ ] **不**补打 v0.1.4–v0.3.5 缺失 tag；**不**上 GitHub Actions（YAGNI，等 mouqin 上线再评估）

**验收标准：** `jianwu --version` 输出 v0.3.6+tag；`go test -race ./...` 全绿；`status` 显示全书累计 token；每章 expand 后 outline/scaffolding 也能 `--tokens`。

---

## v0.4 — 多租户接线（**触发式启动**，不在关键路径）

> 启动条件：mouqin MVP 上线后由真实多租户需求触发（用户数据混杂事故、分账需求、单租户运维成本可见）。
> 不预设范围。`internal/storage/namespace.go` 已实现，多租户地基已铺，真启动时工作量约 2-3 天。

候选工作（触发时再确认范围）：

- [ ] 3 个全局可变 var → 显式参数 DI（`config.secretsProvider` / `book.DefaultStorage` / `cli.cliWorkspaceDir`）
- [ ] `defaultSecretsProvider.LoadSecretsFor(tenantID)` 真用 tenantID
- [ ] mouqin SaaS app 在请求路径上传递 tenantID 到 Storage.Namespace / SecretsProvider
- [ ] 审计所有 `os.IsNotExist` 调用点（CLI 3 处）对非 OS Storage 的兼容性

---

## 紧急 hotfix（2026-06-30，独立 commit）

> v0.3 审计 F4：mouqin.com waitlist 生产代码安全洞，不等 v0.3.6。

- [x] `website/functions/api/waitlist.js` HTML escape 邮件体（防邮件 HTML 注入）
- [x] Turnstile fail-closed（未配置 secret → 503，不再静默跳过）
- [x] KV 限流（10 min / IP / 最多 3 次，防 Resend 配额烧光）
- [x] `docs/DEPLOY_MOUQIN.md` 加 `TURNSTILE_SECRET_KEY` + `WAITLIST_KV` 绑定说明

---

## v1.0 — mouqin SaaS

> 目标：把 jianwu 包装成多用户 Web 服务。独立仓库 `mouqin`。
> **前置条件已满足：** v0.3 SaaS-ready 内核（存储 / 任务 / 计量 / 并发 / 安全）已全部交付。

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
