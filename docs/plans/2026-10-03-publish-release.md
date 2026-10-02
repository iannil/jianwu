# `publish` 与 Release 模型技术设计（2026-10-03）

> 对应 [ADR 29](../decisions/29-publishing-layer.md) 实现顺序第 1 步。状态：**已实现（2026-10-03，v0.3.11）**——`internal/release` 包 + `publish` CLI 命令 + serve 端点（`POST /books/{slug}/publish`、`GET /books/{slug}/releases`）+ Web UI 发布入口；表格驱动测试、`go test -race`、vet、gofmt 通过。与 [EPUB3 导出设计](2026-10-03-epub3-export.md) 解耦：`Options.BuildEPUB` 注入钩子待 EPUB 切片接线。

## 目标与非目标

**目标**

1. `jianwu publish <slug>` 产出 `books/<slug>/releases/<version>/`：manifest、provenance、状态快照、章节副本，EPUB 可用时附产物。
2. 发布硬门：全书 final + License 必填；已存在的版本号不可覆盖。
3. publish 对书籍源文件**只读**——不修改 outline/meta/章节状态；Release 是独立产物目录。

**非目标**

- 不做分发（静态阅读站 / OPDS 是 ADR 29 第 3 步）。
- 不做账号、支付、跨书目录服务。
- 不强制依赖 EPUB 切片（见"衔接"）。
- 不维护独立版本索引文件——`releases/` 目录本身就是版本清单，无可损坏状态。

## 状态机衔接（已核实）

`finalize` 要求全部章节 reviewed 才转 final 并设 `Meta.Status=final`；`revise`（revise.go:219）与 `expand`（expand.go:159）都会把 `Meta.Status` 打回 draft。因此 **finalize 之后的任何内容修改天然重新武装发布门**——"新 edition 必须重新 factcheck 与 review"（ADR 29）由现有状态机自动保证，publish 只需校验、不需新增状态。

## 版本模型

- 版本号两段 `MAJOR.MINOR`（ADR 29 只定义两级；勘误与内容修订都是 minor）。
- 首个 release = `1.0`。
- 默认自动推导：与上一 release 的 manifest 对比——parts 数量变化，或任一 part 的章节数量变化（对应 add/delete/move chapter）→ `MAJOR+1`、MINOR 归 0；否则 `MINOR+1`。
- `--major` 强制升 major；`--version X.Y` 显式指定（校验格式，拒绝 ≤ 已存在的最大版本）。
- 结构对比的数据：manifest `content.chapters` 的 (part, chapter, title) 列表。

## 发布门（gate）

**硬门（拒绝发布，退出非零）**

1. `Meta.Status == final` 且全部章节 `status == final`（双保险，防手改 outline.json）。
2. `Meta.License` 非空（ADR 29 硬门；字段定义在 EPUB 设计 T5，若本切片先行则自带同样的字段迁移）。
3. 目标版本目录不存在（不可变；无 `--force` 覆盖路径，改书重新发新版）。
4. 全部章节文件存在（finalize 理论上排除了缺章，防御性检查）。

**警告（放行，但必须显示并计入 manifest）**

- 未核验论断数 > 0（claims 无 verdict 或无 citation_ids）——EPUB 来源核验页会向读者披露，发布不隐藏。
- verdicts 中未通过（`Verified=false`）数量。
- `reviewed_by` 为空的章节（review 是人工批准，缺署名只提示）。

`--dry-run`：输出完整 gate 报告 + 下一个版本号 + 将产出的文件清单，不写任何东西。

## Release 目录结构

```
books/<slug>/releases/<MAJOR.MINOR>/
  manifest.json
  provenance.json
  outline.json          # 状态快照（claims/verdicts/citations 真相源；第 4 步勘误回路按版本比对）
  meta.json             # 状态快照
  content/NN-MM.md      # 定稿章节副本（含 frontmatter）
  artifact/<slug>.epub  # 仅当 EPUB 构建器可用
```

**manifest.json**（schema 1，字段蛇形 JSON 标签，遵守项目约定）：

```json
{
  "schema": 1,
  "book_id": "…", "slug": "…", "version": "1.0",
  "created_at": "…UTC…",
  "jianwu_version": "…",
  "content": {
    "outline_sha256": "…", "meta_sha256": "…",
    "chapters": [
      {"part": 1, "chapter": 1, "title": "…", "path": "content/01-01.md",
       "sha256": "…", "word_count": 123, "status": "final"}
    ],
    "claims_total": 40, "claims_unverified": 3,
    "citations_total": 22, "verdicts_failed": 1
  },
  "epub": {"path": "artifact/x.epub", "sha256": "…", "bytes": 12345}
}
```

`created_at` 允许墙钟时间：发布是事件而非确定性内容（对比 EPUB 构建的确定性要求——那是为了同状态可复现；release 不重复生成同版本，两者不矛盾）。

**provenance.json**（schema 1）：

- engine：jianwu 版本 + `Meta.Engine` 全部资源版本（ArchetypeLibraryVersion 等，字段现成）。
- 书参数：archetype / audience / depth / goal / length / language。
- 模型与用量：`Meta.TokenUsage` 汇总 + 每章 `ExpandedWith`（provider/model/tokens）。Token 语义遵守交付约束：只记 provider 已报告的 LLM 用量，未知不伪装为零。
- AI 披露：`generated_by: "jianwu <version> 辅助生成"`；`human_review`：各章 reviewed_at 时间戳列表。
- sources：全书去重 citation 列表（id / url / title / accessed_at / search_provider / reader_provider）。`Citation` 无 license 字段：v1 以此清单为商用前人工 license 审计的依据；给 Citation/collect 管线补可选 license 字段是后续独立项，不在本切片。

## 包结构与写入协议

新包 `internal/release/`（与 `internal/export/` 对称：export 面向开发交换，release 面向读者交付）：

```
internal/release/
  publish.go      // Publish(st storage.Storage, in Input, opts Options) (Result, error)
  gate.go         // CheckGate(bc) → GateReport{Blockers, Warnings}
  version.go      // NextVersion(st, bookDir, outline, opts) → string
  manifest.go     // manifest/provenance 组装 + sha256
  *_test.go
```

`Input` 持 Meta/Outline/章节内容（由 cli/serve 用 `loadBook` 装配），全部写经注入的 `storage.Storage` → `MemStorage` 可测。

写入协议：

1. gate 全过 → 读全部章节、计算哈希 → 逐文件写入 staging 目录 `releases/.staging-<version>/`。
2. staging 完整后 `storage.Rename` 为 `releases/<version>/`（接口现成）。
3. 任一步失败 → `storage.RemoveAll(staging)`，退出非零并报告；历史 release 不受影响。
4. 并发约束沿用交付约束：同书不跨进程并发写；serve 侧 publish 走 JobManager 与 expand/finalize 同一串行队列。

## CLI 与 serve 接线

- `internal/cli/publish.go`：`newPublishCmd()`（`--dry-run` / `--major` / `--version`）+ `runPublish()` 可测试核心；`root.go` 注册（模式同现有命令）。
- serve：`POST /api/v1/books/{slug}/publish`（`{major?, version?, dry_run?}`）——dry_run 同步返回 gate 报告，实际发布走 job；`GET /api/v1/books/{slug}/releases` 列出已发布版本摘要。
- Web UI 书籍详情工具栏新增发布分组（发布按钮 + 版本历史）；前置条件不满足时展示 gate 报告（与现有 428 拦截一致的交互模式）。
- 错误维持 `InfoError` 语义：gate 拒绝 = ExitCodeGeneric，用法错误 = ExitCodeUsage。

## 与 EPUB 切片的衔接

`Options.BuildEPUB func() (epubBytes []byte, err error)` 注入：`internal/export` 存在时由 cli/serve 装配（确定性构建，同一书籍状态字节相同），未实现时为 nil，manifest 无 `epub` 节。**release 不 import export**——依赖方向单向（接线层组合），两切片实现顺序自由，与 ADR 29 排序"由评估证据驱动"的立场一致。

## 测试策略

- **单元（表格驱动）**：gate 四类拒绝 + 三类警告；版本推导（首版 1.0 / 增删章 → major / 仅内容 → minor / `--version` 格式与回退拒绝）；manifest 哈希与落盘文件一致（读回校验）；不可变（同版本二次 publish 失败且原目录未动）；staging 失败清理（注入写错误）。
- **serve 集成**：Deps 注入 mock，dry_run 报告 + job 完成后断言 releases 目录。
- **端到端**：final + license 书 → publish 1.0 → 模拟 revise（书打回 draft）→ publish 被拒 → 重新 review/finalize → 自动推导 publish 1.1。这条用例直接验证 ADR 29 的"新 edition 必须重新走 review"。

## 风险与对策

- **TOCTOU**（gate 与快照之间书被改）：单进程串行约束 + serve 队列；gate 后立即快照，manifest 哈希锁定的是快照内容。
- **releases/ 被手改**：发布前对已存在版本做 manifest 存在性检查，缺失即报错，不静默重建。
- **meta 快照含 TokenUsage 漂移**：快照语义正确（记录发布时点状态），manifest 哈希只对快照文件本身负责，不与当前 meta 比较。
