---
title: "快速开始"
description: "五分钟上手 jianwu"
---

## 安装

```bash
go install github.com/iannil/jianwu/cmd/jianwu@latest
```

或从源码构建：

```bash
git clone https://github.com/iannil/jianwu
cd jianwu
go build -o ./bin/jianwu ./cmd/jianwu
```

## 前置条件

- **Go 1.25+**
- **API 密钥**（至少一个）：Gemini API Key 或 GLM API Key

可写入 `~/.config/jianwu/secrets.yaml`（权限 0600）：

```yaml
gemini:
  api_key: "your-gemini-key"
glm:
  api_key: "your-glm-key"
```

或使用环境变量：

```bash
export GEMINI_API_KEY=your_gemini_key
export GLM_API_KEY=your_glm_key
```

## 快速上手

```bash
# 初始化工作区
jianwu init my-library
cd my-library

# 开始创作：交互式问诊 → 大纲生成 → 章节框架
jianwu new

# 展开第一章（地址：第一部分，第一章）
jianwu expand my-book 01-01

# 标记为已审阅
jianwu review my-book 01-01

# 查看进度
jianwu status my-book

# 定稿并导出
jianwu finalize my-book
jianwu export my-book --target md
```

## 核心概念

| 概念 | 说明 |
|------|------|
| **Workspace** | 一个 git 仓库，包含 `.jianwu/` 配置 + `books/` 输出目录 |
| **Slug** | 书籍的 kebab-case 标识，也是 `books/` 下的子目录名 |
| **Chapter Address** | `<NN-MM>` 格式：部 NN、章 MM，如 `01-01` |
| **State Machine** | `scaffolded → expanded → reviewed → final → export` |
