---
title: "配置指南"
description: "5 层配置体系详解"
---

jianwu 采用五层配置合并策略，低 → 高优先级：

1. **编译时默认值**（`internal/config/defaults.go`）
2. **全局用户配置**（`~/.config/jianwu/config.yaml`）
3. **工作区配置**（`<workspace>/.jianwu/config.yaml`）
4. **环境变量**（如 `JIANWU_OUTLINE_MODEL=glm-4.6`）
5. **CLI 标志**（如 `--model glm-4.6`）

## 常用配置

```bash
# 查看当前配置
jianwu config list

# 获取特定值
jianwu config get models.outline.provider

# 设置值
jianwu config set scaffolding.concurrency 10
```

## 环境变量

| 变量 | 说明 |
|------|------|
| `GEMINI_API_KEY` | Gemini API 密钥 |
| `GLM_API_KEY` | GLM API 密钥 |
| `JIANWU_OUTLINE_MODEL` | 大纲阶段模型 |
| `JIANWU_EXPAND_MODEL` | 展开阶段模型 |
| `BRAVE_API_KEY` | Brave 搜索 API 密钥 |
| `SERPER_API_KEY` | Serper 搜索 API 密钥 |
| `JINA_API_KEY` | Jina Reader API 密钥 |

## LLM Provider 配置

| Provider | 环境变量 | 默认模型 |
|----------|----------|----------|
| Gemini | `GEMINI_API_KEY` | gemini-2.0-flash |
| GLM | `GLM_API_KEY` | glm-4.6 |
| Ollama | 无需密钥（本地） | qwen3:14b |

每种 Provider 支持独立的模型和超时配置：

```yaml
models:
  outline:
    provider: gemini
    model: gemini-2.0-flash
    timeout: 120s
  expand:
    provider: glm
    model: glm-4.6
    timeout: 300s
```
