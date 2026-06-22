# jianwu 26 项核心决策记录

> 本文档归档 2026-06-21 grill-me 会话中达成的 26 项实施层决策。后续迭代若需变更，先看这里。
> 完整推理过程在原始 grill-me transcript；本文档是结论速查。

---

## 决策表（按主题分组）

### 架构层

| # | 维度 | 决定 | 影响 |
|---|---|---|---|
| Q1 | 项目结构 | 数据/代码混在 `internal/`，每数据目录加 `embed.go` | 数据和代码同包，`//go:embed` 邻居 |
| Q2 | 构建顺序 | 纵向切片 S1→S8，风险递增 | 每切片端到端可验收 |
| Q3 | Workspace | 严格 + walk up（类 git） | 任何子目录都能跑命令 |
| Q4 | CLI/Config | cobra + 自己写 5 层 resolver（yaml.v3） | 不用 viper，避免重依赖 |

### Provider 层

| # | 维度 | 决定 | 影响 |
|---|---|---|---|
| Q5 | LLM SDK | Gemini 官方 SDK + GLM 直 REST（OpenAI-compatible 复用） | GLM 那份代码可复用给 Qwen/Moonshot/DeepSeek |
| Q6 | Prompt caching | Gemini context cache API + GLM 原生 caching | 删掉 DESIGN.md 原写的"客户端 hash 比对"（错的） |
| Q7 | Retry/Fallback | 4xx 不 fallback；网络/timeout/429/5xx 触发；retry 3 次指数退避 | 同 provider 先 retry，retry 完才 fallback |
| Q20.2 | Embedding lookup | v1.0 实时算、不预生成 index | 取消 Q8 决定（不需要 reindex） |

### 引擎层

| # | 维度 | 决定 | 影响 |
|---|---|---|---|
| Q9 | Prompt 模板 | 内嵌 `.md.tmpl` + `text/template`，每阶段独立 prompt 目录 | 改 prompt 不需重编译（dev 时）；release 时 embed 进二进制 |
| Q10 | LLM 输出解析 | Structured output API（Outline/Scaffolding/Grill）+ Expand 自由文本+脚注 | schema 用 `invopop/jsonschema` 从 Go struct 生成 |
| Q17 | Grill tree walk | 代码驱动 + LLM 辅助（Go own 依赖图、LLM 生成推荐） | 不让 LLM 自由走（避免漏问、错序） |
| Q18 | Outline archetype | 骨架+填充：代码注入 archetype YAML，LLM 按 role 填具体 title/abstract | hints 软引导 |
| Q20.1 | Outline few-shot | 全用（archetype + corpus + samples） | token 成本可承受 |
| Q19 | `new` 流程 | 全自动 grill→outline→scaffolding + 智能 resume | 中断后重入跳过已完成阶段 |

### Grill/Expand 细节

| # | 维度 | 决定 | 影响 |
|---|---|---|---|
| Q11 | Grill session | `.jianwu/sessions/<id>.json` 暂存；resume 默认提示；完成后 archive 到 `books/<slug>/.session.json` | audit log 保留完整 conversation |
| Q12 | Scaffolding 并发 | `errgroup.SetLimit(5)`；continue-on-error；`--retry-failed` 子命令 | 一章失败不影响其他章 |
| Q13 | Expand agent | 固定 3 阶段（研究→草稿→校验+修订）；工具分类硬限（5/10/2）；每迭代覆盖持久化 | 不做自由 agent loop（v1.0 可控） |
| Q14 | Citation | 双写（chapter .md + outline.json）；LLM 自报 unverified_claims；agent loop 注册表收集 metadata | URL 是匹配 key |

### 质量/可观测

| # | 维度 | 决定 | 影响 |
|---|---|---|---|
| Q15 | 测试 | Pragmatic + mock provider + 1 e2e；库 TDD + LLM test-after | 不追求 100% 覆盖率 |
| Q16 | 错误/可观测 | 5 档 exit code；slog；`--debug` dump LLM | `0/1/2/3/4/5` 对应成功/通用/用法/workspace/LLM/网络 |
| Q21 | Secrets/Slug | ENV > file + 0600；懒报错 + `jianwu info` 诊断；slug 冲突 `--force` | secrets 不入 workspace |

### Workspace

| # | 维度 | 决定 | 影响 |
|---|---|---|---|
| Q22 | init/migrate | 完整目录 init；bare 仅 `.jianwu/`；schema_version 独立文件；migrate 检查代码就位 | v1.1 加真 migrate 命令 |
| Q26 | init defaults | 完整模板+注释；全局首次自动创；init 不 prompt key | keys 在 secrets.yaml 或 ENV |

### 打包

| # | 维度 | 决定 | 影响 |
|---|---|---|---|
| Q23 | 分发 | `go install` + GitHub 公开 | v1.0 祝融自用最快 |
| Q24 | License | AGPL-3.0（代码）；数据独立标 ©祝融 | 保护未来 mouqin SaaS |
| Q25 | 库选型 | goldmark + google/uuid + 全部推荐库 | 零争议 |

---

## 后续迭代需变更决策时

变更决策要更新：

1. 本文档对应行
2. 受影响的代码 + 测试
3. `PROJECT_STATUS.md` 的相关段落
4. README（如果用户可见）

变更原因（why）写到 commit message 里，不要只改结论不解释动机。

## 没问过的开放问题

- v1.1 加 fallback 时是否需要"按阶段独立 fallback 策略"？（如 outline fallback 到 glm-4.6 但 expand 不 fallback）
- 流式输出是否影响 prompt caching（Gemini context cache + 流式响应是否兼容）？
- 模型升级（gemini-2.5 → 2.6）时的 deprecation 策略？

这些是写代码时再敲的细节，不阻塞 v1.0.x。
