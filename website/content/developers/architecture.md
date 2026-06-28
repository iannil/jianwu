---
title: "架构概览"
description: "项目架构与包结构"
---

## 技术栈

- **语言：** Go 1.25
- **CLI：** cobra + pflag
- **配置：** YAML（5 层合并）
- **LLM：** Gemini SDK / OpenAI-compatible REST / Ollama HTTP
- **搜索：** Brave Search / Serper
- **阅读器：** Jina Reader

## 8 个关键内部包

```
cmd/jianwu/main.go
  └── cli/              # cobra 命令树
      ├── engine/       # 创作引擎（6 个子包）
      │   ├── grill/    # 访谈
      │   ├── outline/  # 大纲
      │   ├── scaffolding/ # 框架
      │   ├── expand/   # 展开（调研→草稿→验证）
      │   ├── factcheck/# 事实复核
      │   └── revise/   # 修订
      ├── book/         # 领域类型
      ├── provider/     # 抽象层 + 工厂
      │   ├── llm/      # Chatter/Embedder/Streamer
      │   ├── search/   # Brave/Serper
      │   └── reader/   # Jina
      ├── storage/      # Storage 接口（OS/MemStorage）
      ├── config/       # 5 层配置
      ├── workspace/    # 工作区管理
      ├── corpus/       # 语料库
      └── archetypes/ + style/ # 嵌入资源
```

## 状态机

```
scaffolded → expanded → reviewed → final → export
```

`outline.json` 为状态单一真相源，`.md` frontmatter 镜像同步。

## Provider 抽象

- **Chatter / Embedder / Streamer** — Go 风格的窄接口
- **Retry 3 次**（指数退避 + jitter）→ fallback 兜底
- **每个阶段**可独立配置模型和超时
