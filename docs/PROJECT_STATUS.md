# jianwu 项目状态

> 更新：2026-10-03。开发版本 **0.3.12**，本轮修改尚未正式发布。

## 产品

本地优先的非虚构图书创作 CLI。通过设计访谈、结构原型、逐章展开、来源复核、人工审阅与导出，帮助作者交付可追溯的知识作品。jianwu 为独立产品，不再为 mouqin 提供内核或承担其前置开发。见 [ADR 28](decisions/28-independent-product.md)。

`internal/` 是产品内部实现，不是外部 SDK。仓库保留的 mouqin 官网和部署资产属于历史资产，本轮没有上线或迁移。

## 当前实现

- `new`：12 维访谈 → outline → 并行 scaffolding，6 个原型、10 本内置参考语料。
- `scaffolding <slug> --retry-failed`：只重试框架失败章节（`new` 的恢复路径；serve 侧对应 `POST /api/v1/books/{slug}/scaffold-retry`）。
- `expand`：调研 → 草稿 → 校验，保存完整 claims、脚注、章节状态与模型信息。调研阶段拒收登录墙/空壳源（reader.ContentIssue）。
- `expand --all`：最多 5 个并发任务；大纲在生成期间只读；所有任务结束后逐个保存。失败返回非零退出码，独立成功章节保留。
- `factcheck`：按 citation_ids 查找来源，不按位置关联。无关联、失效来源和读取失败保留未通过结论；登录墙/空壳源标记为 source unusable 并计入 SourceErrors；同一论断的多个来源分别核对。
- `revise`：改写后重新校验正文（注入 style guide 保持行文连贯）、重建 claims 和引用，并撤销旧审阅/verdict；脚注日期按结构化引用数据回填。仍需再次 factcheck 与人工 review。
- `review`/`finalize`：明确的人类确认与定稿。`review` 是人工批准，不代表系统保证事实正确。
- `export`：Markdown、Hugo、PDF（后者依赖 pandoc/xelatex）、EPUB3（纯 Go）。
- `publish`（v0.3.11，ADR 29 第 1 步）：发布门（final + license + 版本未占用，警告披露不拦截）→ `releases/<MAJOR.MINOR>/`（manifest 内容哈希、provenance 来源与用量、状态快照、章节副本；EPUB 经注入钩子可选）。版本自动推导：结构变化升 major，内容修订升 minor；已发布版本不可覆盖；staging 目录 + rename 原子落位。serve 侧 `POST /books/{slug}/publish`（dry_run 同步返回门报告）与 `GET /books/{slug}/releases`，Web UI 有发布入口。
- `export --target epub`（v0.3.11，ADR 29 第 2 步）：纯 Go EPUB3（goldmark，无 pandoc 依赖）。脚注按章编号、章末 EPUB3 aside（noteref/doc-footnote ARIA）；每章"来源与核验"节由 Citations/Claims/Verdicts 生成（四态披露）；正文原始 HTML 转义保 XHTML 良构；`dc:identifier` 用建书 UUID，`dcterms:modified` 取 Meta.UpdatedAt，zip 条目固定——同状态字节可复现，release 内 artifact 与导出产物一致。封面约定 `cover.png|jpg`。
- `site`（v0.3.11，ADR 29 第 3 步）：从已发布 release 生成静态阅读站（书架/书页/章节阅读页/EPUB 下载/OPDS acquisition feed）。章节页与 EPUB 共用同一渲染器，来源核验节逐字一致；确定性生成（时间戳取 manifest）；损坏 release 跳过并报告。serve 侧 `POST /api/v1/site/generate`。
- Agent 接入层（v0.3.12，ADR 30）：`jianwu mcp`（stdio MCP 服务器，官方 go-sdk，16 工具复用 server 编排，长任务 job 模式）；`/api/v1` 正式化为 agent 契约并新增可选 Bearer token（非 localhost 无 token 拒绝启动）；site 新增 `rss.xml`（RSS 2.0，EPUB enclosure，`--base` 绝对链接）；`jianwu skill` 安装内嵌 SKILL.md（含人审闸门规则）。人工闸门不放宽：review 需显式 reviewer，发布硬门不变。
- Token：new/expand/factcheck/revise 累计 provider 报告的 LLM 用量；--tokens 控制即时显示，status 显示书级累计。失败响应、重试、fallback、流式均通过 tracking wrapper；未报告用量标为不完整。
- 发布：--version、-v、version 输出相同构建信息；release.sh 构建本地产物与校验和，不自动打标签或推送。

## 架构与边界

`cmd/jianwu` → `internal/cli` → `engine/{grill,outline,scaffolding,expand,factcheck,revise}`。

领域数据在 `book`；配置在 `config`；Provider 通过小接口注入；storage 提供 OS 与测试实现；发布层在 `release`（ADR 29）。S3Storage 仍是未实现占位。

保留进程级的 DefaultStorage、SecretsProvider 和 CLI 工作区标志；不支持在一个进程中按请求切换它们。Namespace 是路径前缀工具，不是安全沙箱。

## 可靠性与已知限制

- 元数据和章节用临时文件加 rename 替换；普通展开/修订写入失败尝试恢复旧文件，并报告恢复失败。
- 不提供跨文件崩溃事务或断电持久性保证。批量结果在本轮生成任务结束后保存；进程被强制终止可能丢失尚未保存的结果。
- 不支持多个 CLI 进程同时修改同一本书。运行前备份/提交工作区，写入失败时保留错误日志并检查正文与 outline。
- Token 是已报告的 LLM 消耗，不是账单；不含搜索、阅读器、embedding 费用，未报告的远端消耗不能还原。旧书没有记录的历史消耗未知。
- 旧 claims 没有 citation_ids 时显示未验证。重新展开会覆盖正文，应先备份，必要时手工合并。
- LLM 自检与来源支持判断仍需人工抽检；程序测试不能证明书籍事实正确或教学有效。

## 验证与下一步

验证命令：`go test -race ./...`、`go vet ./...`、`gofmt -l internal cmd`、`scripts/release_test.sh`、`scripts/release.sh 0.3.6 --dry-run`。

本轮全量竞态测试、vet、格式检查、发布保护测试与本地发布演练通过，详见 [交付记录](DELIVERY_2026-10-03.md)。真实样书/读者评估尚未完成，不将 mock 集成测试当作质量评估。下一步按 [样书评估方案](EVALUATION.md) 形成证据。

## 文档

[入门](getting-started.md) · [功能](CAPABILITIES.md) · [架构](architecture/overview.md) · [路线图](ROADMAP.md) · [发布](RELEASING.md) · [旧状态快照](archive/status/2026-06-30-project-status.md)
