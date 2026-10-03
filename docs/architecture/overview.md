# jianwu 架构总览

> 2026-10-03。jianwu 为独立、本地优先的 Go CLI 产品（ADR 28），不承诺公共 SDK。
> 本文档是给 LLM 的"只读一个文件就能上手改代码"的架构地图；命令与端点清单见 [CAPABILITIES](../CAPABILITIES.md)，当前状态见 [PROJECT_STATUS](../PROJECT_STATUS.md)。

## 一句话架构

CLI（cobra）与 Web/MCP 层（net/http + MCP go-sdk）是同一编排内核的两个壳；内核调用 7 个引擎子包完成"访谈→大纲→框架→展开→核查→修订"，领域数据在 `internal/book`，出版层（`internal/release` + `internal/export` + `internal/site`）把定稿书变成不可变版本与可分发的静态站。所有 LLM/搜索/阅读器经 `internal/provider` 小接口注入。

## 分层与数据流

```
接入层（三个壳，同一内核）
  internal/cli        jianwu <cmd>……（单进程，直接调引擎）
  internal/server     jianwu serve（Web UI + /api/v1，可选 Bearer token）
                      jianwu mcp（stdio MCP，16 工具，ADR 30）
                      └─ JobManager：单 worker 串行队列（同书不并发写）
        │
        ▼
引擎层 internal/engine
  grill（12 维访谈） → outline（结构） → scaffolding（并行框架）
  expand（调研→草稿→校验，web_search + read_url，[^N] 引用）
  factcheck（按 citation_ids 核对来源 → Verdicts）
  revise（按 verdicts 修订；作废旧 review/verdict → 重新人审）
  collect（语料自动采集：搜索→阅读→LLM 提取）
        │
        ▼
领域层 internal/book   Meta / Outline / Chapter / Claim / ClaimVerdict / Citation
  真相源 outline.json（状态机：scaffolded→expanded→reviewed→final）
  .md frontmatter 镜像同步；meta.json 含 TokenUsage 与 Author/License
        │
        ▼
出版层（ADR 29）
  internal/release    publish：硬门（final+license+版本未占用）→ releases/<MAJOR.MINOR>/
                      （manifest 哈希 + provenance + 快照 + 章节副本 + EPUB artifact）
  internal/export     EPUB3 纯 Go（goldmark）：脚注 aside + 每章"来源与核验"节；确定性构建
  internal/site       只读 releases/ 生成静态站：书架/章节页/EPUB 下载/RSS/OPDS
```

**核心不变量**（改代码时必须保持）：

1. 同一本书不并发写——serve/MCP 的所有写操作过 JobManager 单 worker；CLI 单进程。
2. `revise`/`expand` 会把 `Meta.Status` 打回 draft——finalize 后任何修改自动重新武装发布门，"新 edition 必须重新人审"由状态机保证，无独立状态。
3. 分发渠道（site）只读 releases/，绝不读工作稿；已发布版本不可覆盖。
4. 确定性输出：EPUB 与 site 的时间戳全部取 manifest/Meta.UpdatedAt，同状态字节相同（publish 的 artifact 哈希依赖此性质）。
5. claims 用 citation_ids 显式关联来源，不按位置推断；来源不可读 → 保守"未核验"，不是内容错误。
6. Token 只记 provider 已报告的 LLM 用量；缺失标"不完整"，未知不伪装为零。

## 包清单（`internal/`）

| 包 | 职责 | 关键入口 |
|---|---|---|
| `cli` | cobra 命令树；`loadBook`/`findChapter` 等共享辅助 | `root.go`（注册全部命令），`book_resolve.go` |
| `engine/grill` | 访谈决策树 + 会话恢复 | `DefaultTree()`、`NewSession()` |
| `engine/outline` | 单次 LLM 结构生成（JSON Schema 强制） | `Generate(ctx, chatter, Input)` |
| `engine/scaffolding` | N 章并行框架（errgroup） | — |
| `engine/expand` | 3 轮迭代展开；拒收登录墙/空壳源 | `ExpandOutput`（claims/脚注/用量） |
| `engine/factcheck` | 按 citation_ids 逐来源核对 | `Result.Verdicts/SourceErrors` |
| `engine/revise` | 按 SuggestedRewrite 修订 + 重建引用 | — |
| `engine/collect` | 语料采集管线 | — |
| `book` | 领域类型 + JSON IO + 脚注重编号/日期规范化 | `RenumberFootnotes`、`NormalizeFootnoteDates` |
| `provider` | `llm`（Chatter/Embedder/Streamer）+ `search` + `reader` + 工厂 | gemini/glm/kimi/deepseek/ollama/mock；brave/serper；jina |
| `storage` | Storage 接口（OS + MemStorage 测试实现） | `storage.OS`、`WriteFileAtomic` |
| `config` | 5 层合并（默认→全局→工作区→env→flag） | secrets 在 `~/.config/jianwu/secrets.yaml`（0600） |
| `workspace` | 工作区发现/初始化（`--dir`>env>全局配置>CWD） | — |
| `archetypes`/`style`/`corpus` | 内嵌 YAML 资源；语料全部来自工作区 | — |
| `server` | Web UI + `/api/v1` + MCP（`mcp.go` 16 工具）+ JobManager | `Handler()`、`MCP()`、`SetToken()` |
| `release` | 发布门 + 版本推导 + manifest/provenance + 原子落位 | `Publish()`、`CheckGate()`、`NextVersion()` |
| `export` | EPUB3 装配（goldmark + EPUB3 脚注 renderer） | `BuildEPUB()`、`RenderXHTML()`、`SourcesXHTML()`、`CollectDir()` |
| `site` | 静态阅读站 + RSS/OPDS | `Scan()`、`GenerateAt()` |
| `skill` | agent 技能内嵌与安装（ADR 30） | `Install()`、`Content()` |

## 工作区磁盘结构

```
<workspace>/
  .jianwu/            config.yaml、sessions/、corpus/
  books/<slug>/
    meta.json         id(UUID)/author/license/token_usage/status
    outline.json      状态真相源：parts→chapters（claims/citations/verdicts）
    chapters/NN-MM.md frontmatter（镜像状态）+ 正文（[^N] 脚注）
    export/           md/hugo/pdf/epub 导出产物（开发用）
    releases/<X.Y>/   不可变发布：manifest/provenance/快照/content/artifact/
  site/               jianwu site 产出（派生状态，整体重建）
```

## Agent 接入（ADR 30，详见 [AGENT_ACCESS](../AGENT_ACCESS.md)）

四种通道共享同一编排：MCP（`jianwu mcp`，stdio，16 工具，长任务 job_id+轮询）、HTTP API（`/api/v1`，可选 Bearer token，非 localhost 无 token 拒绝启动）、RSS/OPDS（site 产物）、SKILL（`jianwu skill` 安装）。人工闸门不放宽：`review` 需显式 reviewer 且要求先向用户展示正文与核验结论；发布硬门（final + license）对 agent 一视同仁。

## 决策记录（ADR）

- [26-grill-decisions](../decisions/26-grill-decisions.md)：26 项核心决策 + v0.1.x 审计
- [27-v0.3-audit](../decisions/27-v0.3-audit-decisions.md)：v0.3 审计（含 mouqin 资产处置）
- [28-independent-product](../decisions/28-independent-product.md)：独立产品，不再服务 mouqin
- [29-publishing-layer](../decisions/29-publishing-layer.md)：出版层（Release 模型 + 静态分发）
- [30-agent-access](../decisions/30-agent-access.md)：agent 接入层（MCP/API/RSS/SKILL）

## 已知边界（不要"修复"这些）

- 单文件原子替换 ≠ 跨文件崩溃事务；断电可能丢失未保存的批量结果（设计如此，交付约束）。
- md/hugo/pdf 导出在 cli/server 双份镜像（EPUB 已收进 internal/export 单一实现）；迁移旧目标到共享包是允许的后续重构，不是 bug。
- `website/` + `scripts/deploy-mouqin.sh` 是 mouqin 历史资产（ADR 28：保留、不部署、不代表路线）。
- 真实样书质量与读者学习效果**未验证**（[EVALUATION](../EVALUATION.md) 进行中）；程序测试不证明内容合格。
