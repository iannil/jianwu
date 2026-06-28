# jianwu 功能概览

> 当前版本：v0.2.2 | 最后更新：2026-06-28

---

## CLI 命令

所有命令均支持全局标志 `-L`/`--verbose`、`--debug`、`-d`/`--dir`（指定 workspace 根目录）。

| 命令 | 版本 | 说明 |
|---|---|---|
| `init [--bare] [path]` | v0.1.0 | 初始化 workspace |
| `info` | v0.1.0 | 工作区诊断信息 |
| `config get/set/list` | v0.1.0 | 配置查询与修改 |
| `new [--force]` | v0.1.0 | 完整创作流程：grill → outline → scaffolding |
| `expand <slug> <NN-MM> [--force]` | v0.1.1 | 单章展开（research → draft → validate，支持 streaming） |
| `review <slug> <NN-MM>` | v0.1.3 | 标记章节为已审阅 |
| `finalize <slug> [--dry-run]` | v0.1.3 | 全书定稿 |
| `export <slug> [--target md\|hugo\|pdf]` | v0.1.3 | 导出全书（markdown / Hugo / PDF） |
| `status <slug>` | v0.1.3 | 章节进度概览 |
| `factcheck <slug> <NN-MM>` | v0.2.0 | 自动事实复核 |
| `revise <slug> <NN-MM>` | v0.2.0 | 基于事实复核结果修订章节 |
| `rewrite <slug> <NN-MM>` | v0.2.2 | 重写章节（等价于 expand --force --force） |
| `add-chapter <slug> --after <NN-MM> --topic "..."` | v0.2.2 | 插入新章节 |
| `move-chapter <slug> <NN-MM> <target-part>` | v0.2.2 | 移动章节到其他 part |
| `delete-chapter <slug> <NN-MM>` | v0.2.2 | 删除章节 |
| `expand --all <slug>` | v0.2.2 | 批量展开全书 |

---

## 创作引擎

### 4 阶段管线

```
grill → outline → scaffolding → expand → [factcheck → revise →] review → finalize → export
```

| 阶段 | 包 | 说明 |
|---|---|---|
| **Grill** | `engine/grill` | 12 维度设计决策树问诊。LLM 逐维推荐，用户接受/修改/跳过。stateful session 可 Ctrl+C 恢复。 |
| **Outline** | `engine/outline` | 单次 LLM 调用 + JSON Schema 强制输出，生成全书目录结构。 |
| **Scaffolding** | `engine/scaffolding` | N 章并行生成章节框架（errgroup，continue-on-error），产出章节目录 + 关键概念。 |
| **Expand** | `engine/expand` | 3 迭代 agent：① Research（web_search + read_url）→ ② Draft（注入 archetype + style + samples）→ ③ Validate（自检 + 修订）。产出带 `[^N]` 引用标记的 markdown。 |
| **Factcheck** | `engine/factcheck` | 逐 claim 读取 cited URL，LLM 验证真实性。结果写入 `outline.json` + 跨章 `ClaimWhitelist`。 |
| **Revise** | `engine/revise` | 基于 factcheck 的 `SuggestedRewrite`，LLM 修订未通过章节。 |

### 状态机

```
scaffolded → expanded → reviewed → final → export
```

`outline.json` 为状态真相源，.md frontmatter 镜像同步。

### 状态机命令

| 命令 | 作用 |
|---|---|
| `review` | `expanded` → `reviewed` |
| `finalize` | 全部 `reviewed` → `final` |
| `export` | 任意状态可导出 |

---

## Provider 抽象

### LLM（Chatter / Embedder / Streamer）

| Provider | 实现方式 | 用途 |
|---|---|---|
| Gemini | 官方 `google.golang.org/genai` SDK | outline、scaffolding |
| GLM | OpenAI-compatible REST + SSE | intake、expand |
| Ollama | HTTP REST（localhost:11434） | 本地模型（Qwen3、DeepSeek 等） |
| Mock | 内存 mock | 单元测试 |

**可靠性：** Retry 3 次（指数退避 + jitter）→ fallback provider 兜底。每个阶段 (`intake/outline/scaffolding/expand`) 可独立配置模型。

### 搜索

| Provider | 用途 |
|---|---|
| Brave Search | 主要搜索 |
| Serper | 备用搜索 |

### 阅读器

| Provider | 用途 |
|---|---|
| Jina Reader | URL → markdown |

### 工厂

`llmfactory` / `searchfactory` / `readerfactory` — 独立包避免 import cycle。

---

## 数据模型

### Workspace 结构

```
<jianwu-workspace>/
  .jianwu/
    config.yaml              # workspace 配置
    schema_version           # 内容 = "1"
    sessions/<id>.json       # grill 运行中会话
  books/<slug>/
    meta.json                # 图书元数据（含 ClaimWhitelist）
    outline.json             # 目录 + 章节状态（含 Verdicts[]）
    .session.json            # grill 已完成会话（audit log）
    chapters/NN-MM.md        # 展开后的章节（YAML frontmatter + markdown）
    export/                  # export 输出
```

### 关键类型（`internal/book/types.go`）

- `Meta` — 图书元数据、archetype 选择、参数（受众/深度/目标/篇幅）
- `Outline` — 多 part 结构，每 part 含 chapters[]
- `OutlineChapter` — 标题、状态、字数、引用、`Verdicts[]`
- `ChapterFrontmatter` — 单章 YAML 头（状态/字数/模型/时间戳）
- `Claim` / `ClaimVerdict` — 声明 + 事实复核结果（含 `SuggestedRewrite`）
- `ClaimWhitelist` — 跨章已验证声明集合，避免重复验证

---

## 配置系统

**5 层合并（高 → 低优先级）：**

1. CLI flag（`--model glm-4.6`）
2. 环境变量（`JIANWU_OUTLINE_MODEL=...`）
3. Workspace `.jianwu/config.yaml`
4. 全局 `~/.config/jianwu/config.yaml`
5. 编译时默认值

**Secrets：** `~/.config/jianwu/secrets.yaml`（强制 0600 权限）或 ENV 变量（`GEMINI_API_KEY` / `GLM_API_KEY` / `BRAVE_API_KEY` 等）。ENV 高于文件。

---

## 数据资产（内置 embed）

- **3 个 archetype YAML：**
  - `ontology-epistemology-practice`（本体-认识-实践）
  - `diagnosis-decode-solution`（诊断-解码-破局）
  - `basics-application-practice`（基础-应用-实战）
- **风格指南** `style-guide.md`
- **3 个 few-shot samples**（对应 archetype）
- **6 本 builtin corpus JSON**（zhurongshuo 萃取）

---

## 导出目标

| 目标 | 说明 |
|---|---|
| `--target md` | 单文件 markdown（Pandoc 兼容 frontmatter，默认） |
| `--target hugo` | 章节分文件 Hugo content 结构（`_index.md` + 逐章文件） |
| `--target pdf` | 通过 pandoc + xelatex 自动生成 PDF |

脚注在跨章导出时自动全局重编号。

---

## 质量基础设施

- **27 个测试包**全绿，`go vet` 干净
- **Storage 接口**（`OS` + `MemStorage`）抽象文件 I/O，16 个测试
- **表格驱动测试**风格（`in`/`want` 命名用例，不用 testify）
- **错误分类：** `ErrNetwork` / `ErrRateLimit` / `ErrServer` → retry；`ErrLLMProvider` → 不重试
- **退出码：** 0 成功 / 1 通用 / 2 用法 / 3 workspace 未找到 / 4 LLM 错 / 5 网络错

---

## 下一迭代（v0.2 剩余）

- corpus sync 扩展语料
- Embedding 索引文件缓存
- Workspace migration（schema v1 → v2）
- 后 3 个原型（micro-meso-macro / theory-dynamics-history-present / mindset-method-practice）
- `corpus sync` 扩展语料
- Embedding 索引文件缓存
- Workspace migration（schema v1 → v2）

## 再往后（v0.3 → v1.0）

- SaaS-ready 内核：多租户存储 / 任务进度 / Token 计量 / 并发安全装配
- mouqin web app（多用户 / 鉴权 / 在线 grill-me / 协作）
