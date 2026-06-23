# 归档：SDD 切片计划（v0.1.0 已交付）

> 这 7 份计划是 v0.1.0 开发过程中用 superpowers:writing-plans 技能产出的 task-by-task 实施计划。
> v0.1.0 ship 后归档于此，作为历史参考 + 后续切片的模板示例。
> 不再维护——以 `docs/PROJECT_STATUS.md` 为当前真相源。

---

## 切片总览

| 切片 | 文件 | 内容 | 任务数 | 状态 |
|---|---|---|---|---|
| S1 | `2026-06-21-s1-foundation.md` | workspace + config + CLI shell | 17 | ✅ v0.0.1 |
| S2 | `2026-06-21-s2-providers.md` | LLM/search/reader provider 抽象 | 14 | ✅ v0.0.2 |
| S3 | `2026-06-22-s3-outline.md` | Outline 引擎阶段 | 6 | ✅ v0.0.3 |
| S4 | `2026-06-22-s4-scaffolding.md` | Scaffolding 并行阶段 | 6 | ✅ v0.0.4 |
| S5 | `2026-06-22-s5-grill.md` | Grill 状态化问诊 | 6 | ✅ v0.0.5 |
| S6 | `2026-06-22-s6-new-command.md` | `jianwu new` 命令编排 | 7 | ✅ v0.0.6 |
| S7 | `2026-06-22-s7-expand.md` | Expand agent 阶段 | 10 | ✅ v0.1.0 |

总计：~66 个 task，每个都单独 TDD + subagent-driven review。

## 用作模板

新切片（v0.1.x expand CLI、v0.2 章节迭代命令等）应该按相同模式：

1. 在 `docs/plans/` 下新建 `YYYY-MM-DD-<slice-name>.md`（注意：不再放 `superpowers/plans/`，那是 v0.1 流程遗留）
2. 头部含 superpowers:subagent-driven-development 引用
3. Global Constraints 段落摘抄 PROJECT_STATUS.md 相关部分
4. File Structure 表列出所有将创建/修改的文件
5. 每个 task 含 Files / Interfaces / TDD steps / commit message
6. 末尾 Self-Review 检查 spec 覆盖 + 类型一致性

参考 S3-S6 的中等大小切片（6-7 个 task）作为模板，比 S1（17 task）和 S7（10 task）更易管理。

## 历史 commit

每个切片对应一个 git tag：

```
v0.0.1 → S1 完成
v0.0.2 → S2 完成
v0.0.3 → S3 完成
v0.0.4 → S4 完成
v0.0.5 → S5 完成
v0.0.6 → S6 完成
v0.1.0 → S7 完成
```

`git log v0.0.1..v0.1.0 --oneline` 可看到完整 80+ commits。
