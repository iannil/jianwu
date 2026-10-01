# jianwu 项目状态

> 更新：2026-09-27。开发版本 **0.3.6-dev**，本轮修改尚未正式发布。

## 产品

本地优先的非虚构图书创作 CLI。通过设计访谈、结构原型、逐章展开、来源复核、人工审阅与导出，帮助作者交付可追溯的知识作品。jianwu 为独立产品，不再为 mouqin 提供内核或承担其前置开发。见 [ADR 28](decisions/28-independent-product.md)。

`internal/` 是产品内部实现，不是外部 SDK。仓库保留的 mouqin 官网和部署资产属于历史资产，本轮没有上线或迁移。

## 当前实现

- `new`：12 维访谈 → outline → 并行 scaffolding，6 个原型、10 本内置参考语料。
- `expand`：调研 → 草稿 → 校验，保存完整 claims、脚注、章节状态与模型信息。
- `expand --all`：最多 5 个并发任务；大纲在生成期间只读；所有任务结束后逐个保存。失败返回非零退出码，独立成功章节保留。
- `factcheck`：按 citation_ids 查找来源，不按位置关联。无关联、失效来源和读取失败保留未通过结论；同一论断的多个来源分别核对。
- `revise`：改写后重新校验正文、重建 claims 和引用，并撤销旧审阅/verdict。仍需再次 factcheck 与人工 review。
- `review`/`finalize`：明确的人类确认与定稿。`review` 是人工批准，不代表系统保证事实正确。
- `export`：Markdown、Hugo、PDF（后者依赖 pandoc/xelatex）。
- Token：new/expand/factcheck/revise 累计 provider 报告的 LLM 用量；--tokens 控制即时显示，status 显示书级累计。失败响应、重试、fallback、流式均通过 tracking wrapper；未报告用量标为不完整。
- 发布：--version、-v、version 输出相同构建信息；release.sh 构建本地产物与校验和，不自动打标签或推送。

## 架构与边界

`cmd/jianwu` → `internal/cli` → `engine/{grill,outline,scaffolding,expand,factcheck,revise}`。

领域数据在 `book`；配置在 `config`；Provider 通过小接口注入；storage 提供 OS 与测试实现。S3Storage 仍是未实现占位。

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

本轮全量竞态测试、vet、格式检查、发布保护测试与本地发布演练通过，详见 [交付记录](DELIVERY_2026-09-27.md)。真实样书/读者评估尚未完成，不将 mock 集成测试当作质量评估。下一步按 [样书评估方案](EVALUATION.md) 形成证据。

## 文档

[入门](getting-started.md) · [功能](CAPABILITIES.md) · [架构](architecture/overview.md) · [路线图](ROADMAP.md) · [发布](RELEASING.md) · [旧状态快照](archive/status/2026-06-30-project-status.md)
