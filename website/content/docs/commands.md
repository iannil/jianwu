---
title: "命令参考"
description: "完整 CLI 命令列表"
---

## 全局标志

所有命令均支持：

| 标志 | 简写 | 说明 |
|------|------|------|
| `--verbose` | `-L` | 详细输出 |
| `--debug` | | 调试模式 |
| `--dir` | `-d` | 指定 workspace 根目录 |

---

## 命令一览

### 工作区管理

| 命令 | 版本 | 说明 |
|------|------|------|
| `init [--bare] [path]` | v0.1.0 | 初始化 workspace |
| `info` | v0.1.0 | 工作区诊断信息 |
| `config get/set/list` | v0.1.0 | 配置查询与修改 |

### 创作流程

| 命令 | 版本 | 说明 |
|------|------|------|
| `new [--force]` | v0.1.0 | 完整创作流程：grill → outline → scaffolding |
| `expand <slug> <NN-MM> [--force]` | v0.1.1 | 单章展开（research → draft → validate） |
| `expand --all <slug>` | v0.2.2 | 批量展开全书 |
| `review <slug> <NN-MM>` | v0.1.3 | 标记章节为已审阅 |
| `finalize <slug> [--dry-run]` | v0.1.3 | 全书定稿 |
| `export <slug> [--target md\|hugo\|pdf]` | v0.1.3 | 导出全书 |
| `status <slug>` | v0.1.3 | 章节进度概览 |

### 质量保障

| 命令 | 版本 | 说明 |
|------|------|------|
| `factcheck <slug> <NN-MM>` | v0.2.0 | 自动事实复核 |
| `revise <slug> <NN-MM>` | v0.2.0 | 基于复核结果修订章节 |
| `rewrite <slug> <NN-MM>` | v0.2.2 | 重写章节 |

### 章节编辑

| 命令 | 版本 | 说明 |
|------|------|------|
| `add-chapter <slug> --after <NN-MM> --topic "..."` | v0.2.2 | 插入新章节 |
| `move-chapter <slug> <NN-MM> <target-part>` | v0.2.2 | 移动章节 |
| `delete-chapter <slug> <NN-MM>` | v0.2.2 | 删除章节 |

### 语料库

| 命令 | 版本 | 说明 |
|------|------|------|
| `corpus list/show/stats` | v0.2.3 | 查看参考语料 |
| `corpus sync --from <path>` | v0.2.3 | 同步扩展语料 |
| `corpus reindex` | v0.2.3 | 重建 embedding 索引 |
