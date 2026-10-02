# 出版层：Release 模型与静态分发（2026-10-02）

## 决定

在创作引擎之上增加出版层。`jianwu publish` 从已定稿图书产出**不可变、版本化的发布包（Release）**；对读者的分发（EPUB、静态阅读站、目录 feed）只读 Release，不读取 workspace，也不反向写回。此决定在 [ADR 28](28-independent-product.md) 的独立产品边界内推进，不改变"多租户、SaaS 计费、公共 SDK 不作为里程碑"的结论。

分发的产品形态：静态优先。EPUB3 为第一格式，静态阅读站与 OPDS feed 次之；不建面向读者的应用服务器，不做 DRM。未来若出现真实的多作者/销售需求，平台服务以独立产品另行立项，且只消费 Release 包。

## 发布物模型

```
books/<slug>/releases/<version>/
  manifest.json     # 版本、章节内容哈希、outline 哈希、claims/verdicts 快照
  content/          # 定稿章节副本
  provenance.json   # 模型、配置快照、累计 token、AI 生成披露、语料来源 URL 与 license
```

- 发布硬门：全部章节 reviewed 且全书 final；license 缺失拒绝发布。既有"修订作废旧审阅与 verdict"规则延伸为：**新 edition 必须重新走 factcheck 与人工 review**。
- 版本语义：major = 结构变化（增删章/part），minor = 内容修订；勘误修订递增 minor。
- `export` 与 `publish` 分离：export 面向开发与草稿交换，publish 是对读者的版本化交付。
- 分发渠道只消费 Release；`serve` 保持单用户创作台定位，其单进程、单任务队列、同书不并发写的可靠性模型不变。

## 实现顺序（依赖样书评估）

0. 先执行 [EVALUATION](../EVALUATION.md)：读者任务数据是出版方向的第一批证据，不先建平台再验证质量。
1. `publish` 命令 + Release 目录 + license/provenance 硬门（技术设计：[plans/2026-10-03-publish-release.md](../plans/2026-10-03-publish-release.md)）。
2. EPUB3 导出：claims 的 citation_ids 映射为脚注与每章来源核验页，溯源能力首次到达读者（技术设计：[plans/2026-10-03-epub3-export.md](../plans/2026-10-03-epub3-export.md)）。
3. 阅读站（实现时细化为：`internal/site` 直接从 releases/ 生成静态站而非演进 hugo 导出——分发只读 Release 原则的推论；技术记录：[plans/2026-10-03-static-site-opds.md](../plans/2026-10-03-static-site-opds.md)）+ OPDS feed。
4. 勘误回路：读者 errata → 结构化勘误文件 → 回流 factcheck/revise → 新 minor Release。
5. （可选，独立决策）平台服务：抽取 cli/server 共享 pipeline 包、独立存储与队列；仅在分发数据成立后立项。

在第 1 步落地之前不进行任何公开分发。`corpus collect` 采集的网络来源用于支撑论断与直接商业化出版是两回事；商用前必须先完成 provenance 的 license 审计。

## 验收

- 未 review/final 或缺 license 的书无法 publish；不存在绕过状态机的发布路径。
- Release 产物带内容哈希，可校验不可变性。
- EPUB 中每个带引用的论断可溯源到来源与核验状态。
- 勘误回流产生的新版本必须重新通过 factcheck 与 review 才能发布。

当前状态与限制见 [项目状态](../PROJECT_STATUS.md)，评估协议见 [样书评估](../EVALUATION.md)。
