---
name: jianwu
description: 用 jianwu（肩吾）创作、核验与出版可溯源的非虚构图书：设计访谈→大纲→逐章展开→事实核查→人工审阅→定稿→发布 EPUB/静态阅读站。当用户想写书、写教程、整理知识成册、核查论断来源或发布电子书时使用。
---

# jianwu（肩吾）操作技能

把 LLM 的知识结构化为人类可阅读的图书，每个论断带显式来源引用。本地优先：一个工作区目录 = 一个项目（建议 git 备份）。

## 接入方式（四选一，共享同一工作区）

1. **MCP**：`jianwu mcp`（stdio）。工具：create_book / list_books / book_status / read_chapter / expand_chapter / expand_all / factcheck / revise / review_chapter / finalize / export_book / publish / list_releases / generate_site / job_status。长任务返回 job_id，用 job_status 轮询。
2. **HTTP API**：`jianwu serve`（默认 127.0.0.1:8787）→ `/api/v1/*`。远端启用 `--token` 后带 `Authorization: Bearer <token>`。
3. **RSS/OPDS**：`jianwu site` 产出的静态站含 `rss.xml` 与 `opds.xml`，订阅发布动态。
4. **CLI**：本文件命令均可直接执行。

## 核心工作流

```
jianwu new --tokens                     # 12 维访谈 → 大纲 → 章节框架（生成 slug）
jianwu expand <slug> --all --tokens     # 逐章：调研→草稿→校验（引用 [^N]）
jianwu factcheck <slug> <NN-MM> --tokens# 按引用核对来源支持度
jianwu revise <slug> <NN-MM> --tokens   # 基于核查结果修订（作废旧 review/verdict）
jianwu review <slug> <NN-MM>            # 人工批准（见下方闸门规则！）
jianwu finalize <slug>                  # 全部 reviewed 后定稿
jianwu publish <slug> --dry-run         # 发布门预检 → publish 写不可变 release
jianwu site                             # 静态阅读站 + RSS/OPDS
```

章节地址格式 `NN-MM`（部-章，如 01-02）。工作区用 `-d <path>` 指定。

## 必须遵守的闸门（agent 不可越过）

- **review 是人工批准**：只有向用户展示正文与来源核验结论、并获得明确同意后，才执行 review/review_chapter，并在 reviewer 参数注明操作者。绝不为消化流程而自动标记。
- **发布硬门**：publish 要求全书 final 且 `meta.json` 有 `license`。缺 license 是设计行为，让用户决定许可证，不要代填。
- **保守失败语义**：来源不可读时论断标"未核验/未通过"，这不等于内容错误；不要据此自动重写正文。
- **同书不并发写**：所有写操作走同一串行队列；不要并行发起同书任务。
- **版本不可变**：已发布版本不可覆盖；内容修改后自动递增版本（结构变化 major / 内容修订 minor）。

## 判断指引

- 用户给主题 → create_book（topic 必填；audience/goal/depth/length/language/archetype 可选，默认 beginner/understanding/intro/short/zh）。
- 用户要"检查/核实" → factcheck 后读 verdicts，未通过的给出 revise 建议，由用户确认再修订。
- 用户要"发布/上架" → 检查 license 与 final 状态，先 dry-run 展示门报告与警告（未核验论断数会如实披露），用户确认再发布。
- 用户要"看进度" → book_status / list_releases。
- 失败重试只针对基础设施错误（网络/超时）；内容质量问题走 revise 流程，不盲目重跑 expand 覆盖正文。

## 输出物

- `books/<slug>/releases/<X.Y>/`：manifest（哈希）+ provenance（模型/用量/AI 披露/来源清单）+ EPUB
- `site/`：书架 + 章节阅读页（含"来源与核验"节）+ RSS/OPDS，可部署任意静态托管
