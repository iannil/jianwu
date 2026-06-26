# 归档：SDD 切片计划（v0.1.x 已交付）

> 本文档归档 v0.1.0（S1-S7）+ v0.1.x（v0.1.1–v0.1.3）所有已完成切片的 SDD 实施计划。
> 不再维护——以 `docs/PROJECT_STATUS.md` 为当前真相源。

---

## 切片总览

### v0.1.0 基础切片（7 份）

| 切片 | 文件 | 内容 | 任务数 | 状态 |
|---|---|---|---|---|
| S1 | `2026-06-21-s1-foundation.md` | workspace + config + CLI shell | 17 | ✅ v0.0.1 |
| S2 | `2026-06-21-s2-providers.md` | LLM/search/reader provider 抽象 | 14 | ✅ v0.0.2 |
| S3 | `2026-06-22-s3-outline.md` | Outline 引擎阶段 | 6 | ✅ v0.0.3 |
| S4 | `2026-06-22-s4-scaffolding.md` | Scaffolding 并行阶段 | 6 | ✅ v0.0.4 |
| S5 | `2026-06-22-s5-grill.md` | Grill 状态化问诊 | 6 | ✅ v0.0.5 |
| S6 | `2026-06-22-s6-new-command.md` | `jianwu new` 命令编排 | 7 | ✅ v0.0.6 |
| S7 | `2026-06-22-s7-expand.md` | Expand agent 阶段 | 10 | ✅ v0.1.0 |

### v0.1.x 切片（3 份，新增）

| 切片 | 文件 | 内容 | 状态 |
|---|---|---|---|
| v0.1.1 | `2026-06-22-v0.1.1-expand-cli.md` | Expand CLI 命令 + chapter I/O + providerDepsHook | ✅ v0.1.1 |
| v0.1.2 | `2026-06-23-v0.1.2-prompt-injection.md` | Archetype + style + sample + adjacent 注入 | ✅ v0.1.2 |
| v0.1.3 | `2026-06-23-v0.1.3-state-machine-commands.md` | review / finalize / export / status 命令 | ✅ v0.1.3 |

总计：~66 个 task（v0.1.0）+ ~20 个 task（v0.1.x），每个都单独 TDD + subagent-driven review。

## 用作模板

新切片（v0.1.4 Fallback Wiring、v0.2 等）应该按相同模式：

1. 在 `docs/plans/` 下新建 `YYYY-MM-DD-<slice-name>.md`（注意：不再放 `superpowers/plans/`，那是 v0.1 流程遗留）
2. 头部含 superpowers:subagent-driven-development 引用
3. Global Constraints 段落摘抄 PROJECT_STATUS.md 相关部分
4. File Structure 表列出所有将创建/修改的文件
5. 每个 task 含 Files / Interfaces / TDD steps / commit message
6. 末尾 Self-Review 检查 spec 覆盖 + 类型一致性

参考 S3-S6 的中等大小切片（6-7 个 task）作为模板，比 S1（17 task）和 S7（10 task）更易管理。
