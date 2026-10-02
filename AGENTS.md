# jianwu (肩吾)

把 LLM 的训练知识结构化为人类可阅读、可学习的图书。  
独立、本地优先的 Go CLI 产品。内部引擎不承诺公共 SDK。入口：`cmd/jianwu/main.go`。

## 项目

- **当前版本：** 0.3.11（独立产品；可靠生成、显式引用、累计用量与本地发布、Kimi/DeepSeek 提供商；本轮：出版层第 1–3 步——`publish` 命令与不可变 Release 模型（ADR 29：发布硬门 final+license、版本自动推导、manifest/provenance、staging 原子落位）+ `export --target epub` 纯 Go EPUB3（脚注 aside、每章"来源与核验"节、确定性构建）+ `site` 静态阅读站与 OPDS（只读 release，章节页与 EPUB 同源渲染）；CLI + serve 全接线。版本号统一在 `internal/cli/version.go` 管理。
- **技术栈：** Go 1.25 + cobra (CLI) + spf13/pflag + YAML 配置 + gemini/glm/kimi/deepseek/ollama LLM 提供商 + net/http 内嵌 Web UI（`internal/server`）
- **入口点：** `cmd/jianwu/main.go` → `cli.NewRootCmd()` → cobra 子命令；`jianwu serve` → `internal/server`（Web UI + `/api/v1` HTTP API）
- **工作区模型：** 每个项目 = 一个本地目录（建议用 git 备份），包含 `.jianwu/` 配置 + `books/<slug>/` 输出（含 `releases/` 不可变发布版本）
- **下一迭代：** 完整样书与真实读者评估，见 `docs/EVALUATION.md`；出版层后续见 `docs/decisions/29-publishing-layer.md`（EPUB3 → 静态阅读站/OPDS → 勘误回路）
- **产品决策：** jianwu 不再服务 mouqin；见 `docs/decisions/28-independent-product.md`。历史官网资产保留，不代表产品路线，不自动部署。

## 命令

| 操作 | 命令 |
|--------|---------|
| 构建 | `go build -o ./bin/jianwu ./cmd/jianwu` |
| 全部测试 | `go test ./internal/...` |
| 单包测试 | `go test ./internal/cli/...` |
| 静态检查 | `go vet ./...` |
| 发布演练 | `scripts/release.sh 0.3.6 --dry-run`（仅本地） |
| 运行 | `go run ./cmd/jianwu <command>` |
| Web UI | `go run ./cmd/jianwu serve`（默认 http://127.0.0.1:8787，`--addr` 可改） |

## 架构

11 个关键内部包（含 2 个新包 v0.2.0 + storage v0.3.0 + release/export/site v0.3.11）：

- **`internal/cli/`** — cobra 命令树；薄封装层，调用 engine + book + workspace。  
  每个子命令有 `newXxxCmd()` + `runXxx()` 可测试核心。所有 CLI 命令列表见 `docs/PROJECT_STATUS.md §9`。
  全局标志：`--verbose`/`-L`、`--debug`、`--dir`/`-d`（指定 workspace 根目录，详见 `root.go`）。
  共享辅助函数在 `book_resolve.go`（`loadBook`/`findChapter`/`findPart`/`parseChapterAddr`/`mirrorChapterStatus`）。
- **`internal/export/`** — 交付格式装配层（v0.3.11，ADR 29 第 2 步）：EPUB3 纯 Go 构建（goldmark 唯一新依赖，零传递）。`Collect` 按章读文 + 脚注重编号 + 日期规范化；`RenderXHTML` 输出 XHTML 良构片段（原始 HTML 转义、EPUB3 脚注 aside + ARIA）；`sourcesXHTML` 从 Citations/Claims/Verdicts 生成"来源与核验"节（四态披露）；`BuildEPUB` 确定性装配（mimetype 首条 STORED、固定 zip 时间戳、dcterms:modified 取 Meta.UpdatedAt）→ 同书籍状态字节相同。md/hugo/pdf 旧目标仍在 cli/server 镜像，未迁移。
- **`internal/engine/`** — 7 个子包：`grill/`（访谈）、`outline/`（结构；参考语料经 `Input.CorpusBooks` 注入）、`scaffolding/`（框架）、`expand/`（3 轮迭代：调研 → 草稿 → 验证）、`factcheck/`（自动事实复核）、`revise/`（基于 verdicts 修订章节）、`collect/`（语料自动采集：搜索 → 阅读 → LLM 提取 → 校验）。核心创作 + 质量管线。
- **`internal/book/`** — 领域类型：`Meta`（含 `TokenUsage`、`Author`/`License` 发布字段；旧 `ClaimWhitelist` 仅兼容读取）、`Outline`（含 `Verdicts[]`）、`Chapter`、`Claim`（显式引用 ID）、`ClaimVerdict`、slug。纯数据 + IO。
- **`internal/release/`** — 出版层（v0.3.11，ADR 29 第 1 步）：`Publish` 产出不可变 `releases/<MAJOR.MINOR>/`（manifest + provenance + 状态快照 + 章节副本，EPUB 经注入钩子装配 artifact）；`CheckGate` 硬门（final + license + 版本未占用，警告披露不拦截）；`NextVersion` 自动推导（结构变化 major / 内容修订 minor）；staging 目录 + Rename 原子落位，失败清理。书籍状态只读。
- **`internal/site/`** — 静态阅读站（v0.3.11，ADR 29 第 3 步）：只读 releases/ 生成书架/书页/章节阅读页/EPUB 下载/OPDS acquisition feed。`Scan` 列书架（缺 manifest 的 release 跳过并报告）；`Generate` 整体重建派生目录，确定性输出（时间戳取 manifest）。章节渲染复用 export 包（RenderXHTML/SourcesXHTML/CollectDir），网页与 EPUB 同源。
- **`internal/provider/`** — 抽象层：`llm/`（Chatter/Embedder/Streamer 接口）、`search/`、`reader/`，以及工厂包。内置实现：gemini、glm、kimi、deepseek、ollama、mock（llm，其中 glm/kimi/deepseek 复用 `llm/openaicomp` 共享层）；brave、serper（搜索）；jina（阅读器）。
- **`internal/storage/`** — `Storage` 接口（v0.3.0 地基）：ReadFile/WriteFile/MkdirAll/RemoveAll/Rename/Stat/ReadDir。默认 `OS` 实现 + `MemStorage` 测试实现（含 16 个测试）。book/workspace/config/cli/grill 已迁移。
- **`internal/config/`** — 5 层合并配置（默认 → 全局 → 工作区 → 环境变量 → 命令行标志）。密钥在 `~/.config/jianwu/secrets.yaml`。
- **`internal/workspace/`** — 工作区的初始化、检测、加载、状态管理（使用 `storage.OS`）。
- **`internal/archetypes/`** + **`internal/style/`** — 嵌入的 YAML 资源（图书原型、风格指南）。**`internal/corpus/`** 不再内嵌语料：参考语料全部来自工作区（`corpus collect` 自动采集 / `corpus sync` 导入 / 手写 JSON）。
- **`internal/server/`** — Web 层（v0.3.6-dev）：`jianwu serve` 启动本地 HTTP 服务（默认 127.0.0.1:8787）。JSON API 在 `/api/v1/`（workspace/config/books/chapters/grill/corpus/jobs/publish/releases），内嵌 SPA 在 `web/`（vanilla JS，无构建步骤，`go:embed`）。长耗时操作经 `JobManager` 单 worker 串行执行（同书不并发写约束）；grill 访谈以"创建会话 → 逐题 answer → generate 任务"方式 HTTP 化。provider 按 `depsNeed` 按需装配（访谈不需要搜索 key）。工作区根可运行时切换（`POST /api/v1/workspace/select`，持久化到全局配置 `workspace:` 键，见 `config/global.go`）；启动解析优先级 `--dir` > `JIANWU_WORKSPACE` > 全局配置 > CWD（`workspace/resolve.go`），未初始化时写操作被 `requireWorkspace` 拦截（HTTP 428）。测试用注入 `Deps`（mock chatter/searcher/reader）。编排逻辑镜像 `internal/cli`（引擎调用相同；未来可抽取共享 pipeline 包）。

## 约定

- **测试：** 表格驱动测试（结构体切片，使用 `in`/`want` 字段，命名用例）。测试函数命名 `TestXxx`（不用 `suite`）。使用 `t.Fatal` / `t.Errorf` / `t.Fatalf`。不用 testify/assert。测试包与源码同级。
- **错误处理：** 用 `fmt.Errorf("context: %w", err)` 包装错误 —— 始终用 `%w` 保持可解包。区分 `*InfoError`（面向用户，带退出码）和内部错误。
- **命名：** Go 标准（camelCase，短变量名）。所有导出的结构体加 JSON 标签（`json:"snake_case"`）。frontmatter 结构体加 YAML 标签。
- **导入：** 标准库第一组，第三方第二组，内部包第三组，组间空行分隔。
- **注释：** 导出的类型/函数写 doc comment（`// Foo does X.`）。设计决策可加交叉引用，如 `// Per decision Q2=B`。
- **库包中不使用 init()**（`embed.go` 中的 `//go:embed` 除外）。不新增全局可变状态；现有 DefaultStorage / secretsProvider / cliWorkspaceDir 限单进程 CLI 使用，不按请求切换。
- **小接口：** `Chatter`、`Embedder`、`Streamer` —— Go 风格的窄接口。
- **provider 装配：** 通过 `ProviderDeps` 结构体 + 工厂函数构建；~~`chatterProviderHook` / `providerDepsHook`~~ 已清除，v0.3.4 用显式参数注入。

## 文档索引

| 文档 | 用途 |
|---|---|
| `docs/CAPABILITIES.md` | 用户面向的功能概览（CLI 命令、引擎管线、Provider、配置、导出） |
| `docs/PROJECT_STATUS.md` | 独立产品当前状态与交付限制 |
| `docs/ROADMAP.md` | 可靠性 → 样书质量验证路线 |
| `docs/architecture/overview.md` | 架构图 + 数据流 + 关键接口 |
| `docs/decisions/26-grill-decisions.md` | 26 项核心决策 + v0.1.x 审计决策 |
| `docs/EXTRACTION_NOTES.md` | zhurongshuo 资产萃取记录 |
| `docs/archive/plans/` | 已完成切片的 SDD plan（v0.1.0–v0.1.6 + v0.2.0）|
| `docs/archive/DESIGN.md` | 原始设计文档（v0.1 锁定版，部分已过期） |

## 备注

<!-- 用于在会话中记录 agent 发现的快速备注空间。 -->

## 交付约束

- 批量展开生成阶段只读大纲，最多 5 并发，协调者顺序持久化；失败必须可见且退出非零。
- claims 用 citation_ids 关联来源，不按位置推断；缺失关联保留未验证。
- 修改正文后撤销旧 review/verdict；再次 factcheck 后人工 review。
- 同一本书不支持跨 CLI 进程并发写入；单文件原子替换不等于跨文件崩溃事务。
- Token 记录 provider 已报告的 LLM 用量，未知量和非 LLM 费用不得伪装为零。
