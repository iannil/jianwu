# 静态阅读站与 OPDS（2026-10-03）

> 对应 [ADR 29](../decisions/29-publishing-layer.md) 实现顺序第 3 步。状态：**已实现（2026-10-03，v0.3.11）**——`internal/site` 包 + `jianwu site` CLI + `POST /api/v1/site/generate` serve 端点（dry_run 同步返回书架）。

## 设计决策

**阅读站从 Release 生成，不走 hugo 导出。** ADR 29 原文写"`--target hugo` 演进为读者主题"，实现时细化为：`internal/site` 直接消费 `releases/`——这是 ADR 29 自己的原则（分发渠道只读 Release、不触碰 workspace）的直接推论。工作稿永远不上书架；hugo 目标保留为开发产物。此细化已回注 ADR 29 第 3 步。

**产物与结构**（`<workspace>/site/`，可整体部署到任意静态托管）：

```
site/
  index.html            书架目录（书名/副题/作者/版本/日期/未核验论断数）
  opds.xml              OPDS 1.x acquisition feed（Atom；acquisition 链接指向 epub/）
  site.css              内嵌阅读样式（衬线、无脚本）
  epub/<slug>.epub      release artifact 的逐字节副本（下载物）
  <slug>/index.html     书页：colophon（许可/AI 披露）、历史版本、目录、EPUB 下载（含 sha256 摘要）
  <slug>/ch-NN-MM.html  章节阅读页：正文（复用 export.RenderXHTML）、脚注、来源核验节、上一章/下一章
```

**复用关系**：章节渲染直接复用 EPUB 切片的 `export.RenderXHTML` / `export.SourcesXHTML` / `export.CollectDir`（新增的目录参数形式：release 的 `content/` 快照与书籍 `chapters/` 同构）——**EPUB 里的脚注与来源核验节和网页版逐字一致**，两种阅读形态同源。

**确定性**：与 EPUB/publish 相同纪律——时间戳全部取 manifest（feed `<updated>` = 各书 release created_at 的最大值），迭代按 slug 排序，同一书架状态生成字节相同（测试锁定）。`site/` 是完全派生状态：每次生成先清空重建，不做增量。

**容错**：损坏的 release（缺 manifest）跳过并在结果中报告（`Result.Skipped`），不中断整站生成；publish 侧对同情况是拒绝，两侧策略按各自职责（发布严格、分发韧性强）。

## 验收

- 未发布的书（无 releases/）绝不出现在书架；书架只读最新 release。
- OPDS feed 良构（XML 测试），acquisition 链接目标文件存在；无 artifact 的书保留 alternate HTML 链接。
- 章节页含 EPUB 同款"来源与核验"节与脚注；末章无"下一章"。
- 两次生成产物字节一致；dry-run 不写盘。
