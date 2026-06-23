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

---

## v1.0.x 完成度审计决策（2026-06-22，21 项）

> v1.0.0 tag 后审计发现：CLI 缺 expand/review/finalize/export/status；expand 引擎 prompt 注入全是占位符。
> 以下 21 项决策对齐 v1.0.x 切片的实施边界。完整推理在原始 grill-me transcript。

### 范围重定

| # | 维度 | 决定 | 影响 |
|---|---|---|---|
| Q1 | v1.0.0 完成标准 | 用户能从 CLI 跑出至少一章节 = ship 门槛；当前 tag 过早 | v1.0.x 系列补齐到 v1.0.5 |
| Q16 | Expand Prompt 注入 | 提到 v1.0.2 单独切片（原计划 v1.1.6） | **最关键缺口**：当前是 generic LLM 输出，非 zhurongshuo 风格 |
| Q17 | v1.0.0 tag 处理 | 保留 + 文档说明"过早"（不重打 tag） | 工程上零成本 |
| Q21 | 切片顺序 | 严格依赖序：CLI → 注入 → 状态机 → fallback → timeout → streaming 可选 | v1.0.5 = v1.0 promise fully delivered |

### v1.0.1 Expand CLI

| # | 维度 | 决定 | 影响 |
|---|---|---|---|
| Q2 | chapter.md frontmatter | 中等 schema：title/part_id/chapter_id/status/word_count/generated_at/model/engine_version/citations/unverified_claims_count | 自包含到"光读文件就能审核" |
| Q3 | 重复运行覆盖 | 默认拒绝；`--force` 覆盖；reviewed/final 需 `--force --force` | 与 `new --force` 一致 |
| Q4 | `expand --all` | 不支持（v1.1 再加） | 防止并发烧钱烧注意力 |
| Q5 | embedder 来源 | 与 chatter 同 provider；不支持时 fallback Gemini + warning | 配置最小惊讶 |
| Q6 | 测试策略 | mock E2E + live integration（手动） | 不写 golden file（LLM 输出非确定） |
| Q20 | E2E hook 覆盖 | `providerDepsHook` 单一结构体（预演 v1.1 重构） | 旧 chatterProviderHook 不动 |

### v1.0.2 Expand Prompt 注入

**范围收敛**：原 scope 把 archetype + samples + similar book + adjacent 四件打包。grill 发现 similar-book 是死方法（`LookupSimilarBook` 从未被调用、需 embedder + 语料内容加载），与另三个静态注入不是一个量级 → **切出 v1.0.2**。v1.0.2 锁定为「四个静态注入」中的三个静态项（archetype + style guide/samples + adjacent），全程零新网络依赖、可用 mock chatter 测。验收：祝融读后说"这是 zhurongshuo 风格"。

复用既有先例：**outline 引擎已有可跑、已测的注入模式**（`outline.go:buildPromptData`：解析 archetypeID → 整份 YAML + verbatim samples，archetype-miss 硬失败、sample-miss 降级）。v1.0.2 expand 对齐这套，避免双轨。

| # | 维度 | 决定 | 影响 |
|---|---|---|---|
| Q1 | 注入架构 | `Generate` 一次性 load archetype+samples+guide，经 `DraftContext` 共享给 draft+validate（非每 iter 自 load） | 单一 load 点；engine 给定 ID 自洽；CLI 保持薄 |
| Q4 | 风格规约 | 完整 `style-guide.md` 注入，删 system_draft/system_validate 的 inline 缩写规则（guide 成唯一风格真相源） | guide 的反例集/硬规则/自检清单是 samples 给不了的；~3K token 吃缓存 |
| Q5 | 相邻章节 | prev/next 的 Title+Abstract+KeyConcepts 进 `user_draft`，nil（首/末/跨 Part）省略该段 | KeyConcepts 给概念交接硬边界（别重定义上章、铺垫下章） |
| Q6 | similar book | 切出 v1.0.2 单独切片；删死方法 `ReadAdjacentChapter`（被 Q5 取代），留 `LookupSimilarBook`。将来仿 outline 的 `CorpusOutlines` 走 archetype-match，不必 embedding | 保持 v1.0.2 零网络依赖 |
| Q7 | 迭代覆盖 | guide→draft+validate 双投；samples+archetype→draft only；research 不动 | guide 是"写"和"查"共享真相源（零漂移）；samples 是 write-only few-shot |
| Q8 | system/user 切分 | system=guide+samples+archetype（Q11 后全书稳定→跨全章单段缓存）；user=本章上下文+相邻章 | 按语义切，不为缓存扭曲布局（Q11 后缓存自然最优） |
| Q9 | 失败模式 | load-error/archetype-miss/guide 硬失败（wrap）；sample-miss 降级 `(no samples for this archetype)` | 对齐 outline；archetype 静默退回 generic 正是在修的 v1.0.1 病，必须 loud。embed 读错误不写额外防御 |
| Q10 | 测试契约 | 抽纯函数 `buildDraftPrompts` + 拼装契约断言（含/不含旧占位符 + nil 省略 + Q9 两例）；人读=验收不进 CI | LLM 输出非确定不写 golden file；可测表面是"prompt 拼没拼进素材" |
| Q11 | 对齐 outline | expand 用整份 archetype YAML + verbatim samples（取代早期 Part-裁剪/剥头草案） | 一致性 + 少写代码（无 Part 定位器/header 剥离器）；whole-YAML 噪声在缓存下成本近零；章节级精修留到验收暴露问题再做 |

**顺手清（各自单独 commit，memory Q15 风格）**：① 删 `ReadAdjacentChapter` + `ToolRegistry.Outline` 字段 + `NewToolRegistry` 对应参数；② 修 `user_draft.tmpl` 尾部陈旧文案"按 schema 输出"（draft 是 free-form markdown，无 schema）。

### v1.0.3 状态机命令

**审计决策（v1.0.x 审计轮，开发前）：**

| # | 维度 | 决定 | 影响 |
|---|---|---|---|
| Q7 | status 转换 | R1+F1+X1：只 expanded→reviewed；全书 reviewed 才 final；failed 拒绝 | 严格状态机 |
| Q8 | export 目标 | 只 md 单文件（v1.0.3） | zhurongshuo/hugo/pdf 推 v1.1.4 |
| Q9 | dry-run 范围 | finalize + export 加 | review/expand 不加 |

**实施决策（2026-06-23 grill，11 项）：** 命令 `review` / `finalize` / `export` / `status`，均纯状态/文件操作（不调 LLM），解析路径统一 `FindWorkspace(".")`→`books/<slug>`→`LoadMeta`+`LoadOutline`。

| # | 维度 | 决定 | 影响 |
|---|---|---|---|
| Q1 | 状态真相源 | outline.json 为查询真相源；review/finalize 写 outline.json 并镜像同步章节 .md frontmatter（read-modify-write 保留正文）；对齐 expand 既有双写 | 全书操作一次 LoadOutline 扫完；单个 .md 自描述诚实 |
| Q2 | ReviewedBy | OS 用户名（`os/user.Current().Username`），不加 `--by` flag | 填上已有字段、不为单作者投机加 flag（YAGNI） |
| Q3 | review 守卫 | 必须 `expanded`，其余（scaffolded/reviewed/final/failed）全拒并报当前状态；无重复 review、无 force、无内容门槛 | 唯一合法源 expanded；重审走重跑 expand（会重置回 expanded） |
| Q4 | finalize 语义 | 全部 reviewed→final（镜像 frontmatter）＋ `Meta.Status="final"`（新增 `BookStatusFinal` 常量），原子写 | 让章节 `final` 枚举可达闭环；finalize 是 final 唯一写者，不发散 |
| Q5 | finalize 前置 + DRY | 扫 outline 收集非 reviewed 章节，有则拒绝列出；空书/已 final 拒绝。**不抽**共享状态机助手，各命令内联守卫 | 2 调用点形状不同，YAGNI（Q14/Q15 风格） |
| Q6 | export 前置 | 不门控 final，任意状态可导出；缺正文章节插占位（`> （本章尚未展开）`）不静默丢；输出头标书状态 | 支持定稿前预览整本草稿，避免盲终稿 |
| Q7' | export 格式 | (1) 脚注**全局重新编号**（全书递增序号，每章正文后跟该章注释块）；(2) 标题页+`##`Part+`###`章三级结构，无 TOC；(3) 输出到 `books/<slug>/export/<slug>.md`，无 `--out` flag | 解决多章拼接 `[^N]` 全局冲突；阅读体验像书 |
| Q8' | status 命令 | slug 必填、单本书；纯文本：头部(书名/状态)+按 Part 树(逐章 status + word count/unverified)+汇总计数+下一步提示；无全书仪表盘、无 `--json` | 与其它 slug-scoped 命令一致；info≠status |
| Q9' | dry-run 行为 | 跑完整校验+算出将做什么并打印，**不写任何文件**；通过/失败结果与真跑一致（既预演又预检） | finalize/export 均如此 |
| Q10 | 测试契约 | `t.TempDir()` 造真实 book 的端到端行为测试 + 文件系统状态断言；不 mock、不 golden；断言结构/子串（含脚注全局唯一、镜像同步、守卫退出码、dry-run 零写入） | 命令不调 LLM 故完全确定可测 |
| Q11 | 命令装配 | 抽 `loadBook(slug)` 共享解析器（5 调用点，含改造 expand）+ `mirrorChapterStatus(...)` frontmatter 镜像助手（review/finalize 共用）；一命令一文件（review/finalize/export/status.go），root.go 注册 4 个 | 消除重复解析 + 易错的 read-modify-write |

### v1.0.4 Fallback Wiring

| # | 维度 | 决定 | 影响 |
|---|---|---|---|
| Q10 | fallback 粒度 | 全局单一 fallback | 简单；v1.1 再考虑按阶段 |

### v1.0.5 LLM Timeout

| # | 维度 | 决定 | 影响 |
|---|---|---|---|
| Q12 | timeout 粒度 | 全局默认 + 阶段覆盖（expand 默认 600s） | 90s 全局默认 |

### v1.0.6 Streaming（可选）

| # | 维度 | 决定 | 影响 |
|---|---|---|---|
| Q11 | streaming + caching | jianwu 不感知兼容性；只 draft 流式 | research/validate 是 JSON，流式无意义 |

### 跨切

| # | 维度 | 决定 | 影响 |
|---|---|---|---|
| Q13 | 模型 deprecation | 锁版本；用户手动升级 | 可复现性优先 |
| Q14 | chatterProviderHook 重构 | 推到 v1.1 | v1.0.x 不动现有 hook |
| Q15 | 技术债打包 | B 为主 C 为辅：顺手清 + 需设计的推 v1.1 | 每 cleanup 单独 commit |
| Q18 | config schema bump | v1.0.x 保持 "1"；v1.1 引入结构性变更才 bump | 可选字段不破坏兼容 |
| Q19 | SDD 节奏 | 每切片走 SDD（小切片可 lite） | 维持 v1.0 工作流纪律 |

### 原"没问过的开放问题"现状

- ✅ **按阶段 fallback 策略**：Q10 决定全局单一，v1.1 再考虑
- ✅ **流式 + caching 兼容**：Q11 决定 jianwu 不感知（SDK 自处理）
- ✅ **模型 deprecation**：Q13 决定锁版本
