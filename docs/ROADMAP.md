# jianwu 路线图

> 更新：2026-10-03。开发版本 0.3.12（未打 tag）。定位：[ADR 28 独立产品](decisions/28-independent-product.md) · [ADR 29 出版层](decisions/29-publishing-layer.md) · [ADR 30 agent 接入](decisions/30-agent-access.md)。

## 已完成（按里程碑）

| 里程碑 | 内容 |
|---|---|
| v0.1.x | 核心管线（grill→outline→scaffolding→expand）+ 状态机 + streaming + fallback |
| v0.2.x | factcheck/revise 闭环 + 章节迭代命令 + 语料（sync/collect/embedding）+ 6 原型 |
| v0.3.0–0.3.10 | Storage 接口 + 长任务进度 + Token 计量 + 批量并发写入 + serve Web UI + Kimi/DeepSeek + 独立产品化（ADR 28） |
| v0.3.11（ADR 29 第 1–3 步） | `publish` 不可变 Release（硬门/版本推导/manifest+provenance）；`export --target epub` 纯 Go EPUB3（脚注 aside + 来源核验节 + 确定性）；`site` 静态阅读站 + OPDS |
| v0.3.12（ADR 30） | agent 接入四通道：`jianwu mcp`（16 工具）、`/api/v1` Bearer token、site RSS、`jianwu skill` |

样书评估：probability 单本机器部分完成（2026-10-03 运行记录），人工部分未开始。

## 当前主线：样书质量验证（EVALUATION）

按 [评估准备](evaluations/2026-09-27/README.md) 与 [运行记录](evaluations/2026-10-03/run-probability.md) 推进：

1. **probability 书人工审阅**（书在 `~/Code/zhurong/jianwu-eval`）：逐章读正文、对来源、抽查论断 → 决定 revise → review → finalize →（用户定 license）→ publish → site。
2. **补 jina key 后重跑 factcheck 轮**（显式记录重跑）：当前 02-01 等章的"未通过"实为来源读取失败（免费额度耗尽），核验结论不可靠。
3. 三类样书各完成一轮（概率/代码审查/检索练习），按 [EVALUATION.md](EVALUATION.md) 记录模板留证。
4. 3 位真实读者前后测；实际账单与 token 对账。

**通过标准先行确定，不用事后阈值掩盖问题**；评估结论驱动后续功能取舍。

## 开放事项（LLM 可直接领取的迭代项）

按优先级排序；领取前读 [架构总览](architecture/overview.md) 的"核心不变量"与"已知边界"。

| # | 事项 | 位置/背景 | 验收要点 |
|---|---|---|---|
| 1 | 评估环境修复：secrets 补 `jina_api_key` 后重跑 factcheck | 用户操作 + agent 辅助 | 重跑记录进 evaluations/，verdicts 可靠 |
| 2 | md/hugo/pdf 导出收编进 `internal/export`（消除 cli/server 双份镜像） | `internal/cli/export.go` + `internal/server/finalize_export.go` | 行为不变，测试迁移，ADR 29 预留的共享包抽取 |
| 3 | Citation 增加可选 `license` 字段 + collect 管线回填（provenance license 审计前置） | `internal/book/types.go`、`engine/collect` | 商用前审计有结构化依据（ADR 29 风险条） |
| 4 | 勘误回路（ADR 29 第 4 步）：site 勘误提交 → 结构化文件 → 回流 factcheck/revise → 新 minor release | `internal/site` + 新 `internal/errata` | **依赖真实读者数据**，评估完成前不动工 |
| 5 | epubcheck 持续校验脚本接入 release 演练（可选 CI 化） | `scripts/epubcheck.sh` | 公开分发前全量校验 |
| 6 | OpenAPI 规范（ADR 30 非目标，按需启动）：`/api/v1` 机器可读契约 | 新 `docs/openapi.yaml` | 外部 agent 集成需求出现再做 |

## 不在路线

mouqin 集成、多租户/SaaS、公共 Go SDK、DRM、OpenAPI 强制化（见 ADR 28/29/30 非目标节）。

旧路线快照：[archive/status](archive/status/)。
