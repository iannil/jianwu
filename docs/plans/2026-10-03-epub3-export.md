# EPUB3 导出技术设计（2026-10-03）

> 对应 [ADR 29](../decisions/29-publishing-layer.md) 实现顺序第 2 步。状态：**已实现（2026-10-03，v0.3.11）**——`internal/export` 包（goldmark 唯一新依赖）；`export --target epub` CLI + serve 端点 + Web UI 按钮；publish 的 release artifact 与导出产物字节一致；确定性/良构性/mimetype 首条 STORED 均有测试锁定。T5（Meta Author/License）随 publish 切片落地。

## 目标与非目标

**目标**

1. `export --target epub` 产出合法 EPUB 3，纯 Go 实现，不依赖 pandoc/xelatex 等外部工具链。
2. 溯源能力到达读者：claims/citations/verdicts 渲染为脚注 + 每章"来源与核验"节。
3. 构建过程确定性（同输入 → 字节相同产物），供 `publish`（ADR 29 第 1 步）直接消费做内容哈希。
4. CLI 与 serve 共用同一实现，不再复制第三份导出镜像。

**非目标**

- 不实现 `publish` 命令与发布硬门本身（另一切片）。
- 不做 DRM、内嵌字体、fixed-layout、SVG/数学公式、media overlays。
- 不迁移现有 md/hugo 导出的 cli/server 双份代码（epub 从第一天走共享包；旧目标重构另行立项）。

## 关键决策

### D1 纯 Go 原生生成，不走 pandoc

EPUB 本质是 zip + XML，标准库（`archive/zip`、`encoding/xml`）足够。pdf 目标依赖 pandoc/xelatex 是历史路径，**交付主格式不复制这个依赖**：发布产物不应因作者机器缺工具而不可产出。"来源与核验"节需要 outline.json 的结构化数据（Citations/Verdicts/Claims），pandoc 无从得知。代价是 markdown→XHTML 渲染需要一个依赖（D2）。

### D2 markdown→XHTML 用 goldmark（新增唯一直接依赖）

- goldmark 零传递依赖，Hugo 同款渲染器（与 `--target hugo` 呼应）。
- `extension.Footnote` 解析章内 `[^N]`；自定义 renderer 把输出改为 EPUB3 语义：行内 `<a epub:type="noteref">`、章末 `<aside epub:type="footnote">`，同步 ARIA `role="doc-noteref"/"doc-footnote"`。
- 默认安全模式（`html.WithUnsafe(false)`）转义正文中的原始 HTML 片段——LLM 产出不可控，转义后输出天然良构，满足 XHTML 的 XML 良构要求。
- 启用 GFM tables/strikethrough 扩展，覆盖章节正文的表格用法。

### D3 新包 `internal/export/`，epub 是第一个成员

现状：导出逻辑在 `internal/cli/export.go` 与 `internal/server/finalize_export.go` 双份镜像。epub 不再复制第三份——建共享包，两侧薄接线。这也是 ADR 29 预见"抽取 cli/server 共享 pipeline"的第一步落子。

```
internal/export/
  epub.go        // BuildEPUB(in Input, st storage.Storage, outPath string) error
  collect.go     // Collect(st, bookDir, outline) → []ChapterDoc
  opf.go         // content.opf 生成
  nav.go         // nav.xhtml（toc + landmarks）
  xhtml.go       // goldmark 装配 + EPUB3 脚注 renderer
  sources.go     // 每章“来源与核验”节生成
  style.css      // 内嵌最小阅读样式（go:embed）
  *_test.go      // 表格驱动，MemStorage
```

- `Input{Meta book.Meta; Outline book.Outline; Chapters []ChapterDoc}`；包不做章节文件 IO 之外的隐式读取，经注入的 `storage.Storage` 写出 → `MemStorage` 全覆盖测试。
- `ChapterDoc{PartIndex, ChapterIndex, PartTitle, Title, BodyMD string; Meta OutlineChapter; Missing bool}`；`Collect` 遍历 outline、读章节文件、缺失章标记 `Missing`（与 md 导出的占位语义一致）。

### D4 脚注策略：按章编号、章末 aside

- 章内 `[^N]` 引用渲染为行内 noteref；`[^N]:` 定义行不进正文，集中渲染为章末脚注 asides。阅读距离近，符合 EPUB 通行做法。
- **不做全书全局重编号**（那是 md 单文件合并才需要的）；每章调用 `book.RenumberFootnotes(body, 1)` 归一，防御 LLM 跳号。
- 渲染前调用 `book.NormalizeFootnoteDates`（复用现有函数），accessed 日期以结构化 `Citation.AccessedAt` 为准，不信正文里的日期。
- 引用无定义（`[^5]` 无对应定义行）时 goldmark 原样输出文本，可接受；T2 加测试锁定该行为。

### D5 "来源与核验"节（差异化核心）

数据源全部在 `OutlineChapter` 内，无需额外文件：

| 数据 | 字段 | 用途 |
|---|---|---|
| 来源 | `Citations[]`（ID/URL/Title/AccessedAt/Snippet） | 来源列表（脚注号、标题链接、访问日期） |
| 论断 | `Claims[]`（Text/CitationIDs/HasCitation） | 论断列表 |
| 核验 | `Verdicts[]`（ClaimText/Verified/Reasoning/CitationID） | 核验状态与理由 |

- 匹配规则：verdict 按 `(CitationID, ClaimText)` 对到 claim；claim 无匹配 verdict → 未核验；claim 无 citation_ids → 无引用，单列。
- factcheck 的 `SourceErrors` 是运行期结果、不持久化；失效来源以对应 claim 的未通过 verdict 呈现，本节不依赖 SourceErrors。
- 版式：每章末尾 `<section epub:type="endnotes">`——来源列表 + 论断核验表（状态徽标：支持 / 未通过 / 未核验 / 无引用，reasoning 以 `<details>` 折叠）。
- **永远生成**，不受开关控制——这是产品差异化，不是可选装饰。

### D6 OPF 元数据映射与 Meta 增量字段

| OPF | 来源 | 说明 |
|---|---|---|
| `dc:identifier` | `urn:uuid:<Meta.ID>` | 建书即 UUID（new_flow.go），跨导出稳定；**禁止**每次导出重新生成 |
| `dc:title` / `dc:title(refine subtitle)` | `Meta.Title/Subtitle` | |
| `dc:language` | `Meta.Language` | |
| `dc:creator` | `Meta.Author`（新字段） | 缺省输出 "jianwu 生成（作者未署名）" |
| `dc:rights` | `Meta.License`（新字段） | 有则输出 |
| `dcterms:modified` | `Meta.UpdatedAt` | **非墙钟时间**，服务 D8 确定性 |
| `<meta name="generator">` | `jianwu <Version>` | 生成披露 |

- Meta 增加可选 `Author`、`License`（`json:"author"/"license"`，omitempty）。仅新增可选字段，旧 meta.json 读取不受影响（`ClaimWhitelist` 兼容读取是先例）。ADR 29 第 1 步的 publish 硬门直接消费 `License`——本切片先落字段、不动门，两个切片在 schema 上衔接。
- 扉页注明：AI 辅助生成、人工审阅状态、jianwu 版本。

### D7 包内布局与 spine

```
mimetype                 application/epub+zip；必须是首个 zip 条目且 STORED（不压缩、无 extra field）
META-INF/container.xml
OEBPS/content.opf
OEBPS/nav.xhtml          properties="nav"；不进 linear spine
OEBPS/style.css
OEBPS/title.xhtml        扉页
OEBPS/part-NN.xhtml      部页（标题 + intro）；nav 中为嵌套目录层级
OEBPS/ch-NN-MM.xhtml     章正文 + 脚注 asides + 来源核验节
```

- 封面：`books/<slug>/cover.{png,jpg}` 存在则作为 cover-image 并加扉页前 cover.xhtml；缺失则跳过，不生成占位封面。
- spine 顺序：cover → title → parts/chapters（outline 顺序）。

### D8 确定性构建（为 publish 预留）

- zip 条目固定顺序；`FileHeader.Modified` 固定常量；产物内容不含墙钟时间（`dcterms:modified` 取 `Meta.UpdatedAt`）。
- 效果：同一书籍状态下两次构建 `bytes.Equal` → `publish` 对产物做 sha256 记入 manifest，可复现可校验（ADR 29 验收"内容哈希"的输入前提）。

### D9 CLI 与 serve 接线

- `internal/cli/export.go`：`case "epub"` → `export.BuildEPUB`；输出 `books/<slug>/export/<slug>.epub`；help 文本与 `--dry-run` 报告（章数/缺失数）与现有目标一致；错误维持 `InfoError` + 退出码语义。
- `internal/server/finalize_export.go`：target 白名单加 `epub`；`handleExportFile` 加 epub 分支（`Content-Type: application/epub+zip`，文件本体即下载物，无需 zipDir）；`runExport` 加 case。Web UI 导出分组加 epub 按钮（纯静态 web/ 改动）。

### D10 测试策略

- **单元（表格驱动）**：nav 结构、来源核验节（覆盖 支持/未通过/未核验/无引用 四态）、脚注映射（正常/跳号/引用无定义）、OPF 元数据（identifier 稳定、modified 来自 UpdatedAt）。
- **集成**：MemStorage 上 Build → 内存解包断言：mimetype 是首条目且 Method=Store；container.xml 存在；content.opf / nav / 全部章节 XHTML 用 `encoding/xml` 解析通过（良构性）；spine 引用的 href 全部存在。
- **确定性**：同输入两次构建 `bytes.Equal`。
- **手工验收**：`scripts/epubcheck.sh`（Java epubcheck，不进 CI）；Apple Books / Calibre / KOBO 三端打开检查中文排版。真实读者质量验证归 [EVALUATION](../EVALUATION.md)，不在本切片。

## 切片任务分解（供 SDD 展开）

| 任务 | 内容 |
|---|---|
| T0 | `go get github.com/yuin/goldmark`；目录骨架；build 通过 |
| T1 | `collect.go` + `ChapterDoc`（含每章 RenumberFootnotes(1)、NormalizeFootnoteDates 接线）+ 测试 |
| T2 | `xhtml.go`：goldmark 装配、EPUB3 脚注 renderer、良构性用例 |
| T3 | `sources.go`：核验节生成 + 四态映射测试 |
| T4 | `opf.go`/`nav.go`/title/part/cover + `epub.go` zip 装配（mimetype 首条 STORED、确定性）+ 集成测试 |
| T5 | Meta 增量字段 `Author`/`License`（读写兼容用例） |
| T6 | cli + serve 接线、Web UI 按钮 |
| T7 | 文档更新（CAPABILITIES、getting-started）、epubcheck 手工脚本 |

每任务后 `go test ./internal/...`、`go vet ./...`。

## 风险与对策

- **LLM 正文含 HTML 片段**：`unsafe=false` 转义解决，输出保持良构。
- **阅读器兼容性**：`aside`/`noteref` 在旧阅读器降级为普通链接与普通节，可接受；三端手工验证兜底。
- **Meta schema 变更**：仅新增可选字段 + 兼容读取测试，风险低。
- **执行顺序**：ADR 29 排序中 publish（第 1 步）先于 EPUB（第 2 步）。本设计使 EPUB 构建器完全独立于 publish，可先做或后做；先做本切片会顺带落掉 `Author/License` 字段，为 publish 铺路。顺序以评估证据驱动，不因设计文档改变 ADR 结论。
