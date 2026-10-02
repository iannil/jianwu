# jianwu（肩吾）

[English](README.md) | 中文

把知识整理成有结构、可审阅、可追溯的非虚构图书。

jianwu 是独立、本地优先的 Go CLI 产品，面向写作者、研究者与知识整理者。通过设计访谈、图书原型、逐章写作、来源复核和人工审阅组织长篇创作。内容、引用和进度保存在自己的工作区。

**开发版本：0.3.11，尚未正式发布。** [项目状态](docs/PROJECT_STATUS.md) · [功能概览](docs/CAPABILITIES.md) · [路线图](docs/ROADMAP.md)

## 工作流程

```text
设计访谈 → 目录 → 章节框架 → 调研/草稿/校验
                             ↓
                        来源复核 ↔ 修订
                             ↓
              人工审阅 → 定稿 → 发布（Release）
                                ↓
                EPUB 产物 · 静态阅读站 · OPDS 订阅
```

- 6 种结构原型、内置参考语料与风格指南。
- Gemini、GLM、Ollama；搜索与来源阅读工具。
- 章节状态、显式 claim/引用关联、来源核对结论。
- 批量展开最多 5 并发，集中保存进度，失败退出非零。
- 累计 provider 已报告的 LLM Token；流式、重试及 fallback 用量跟踪，缺失报告明确提示。
- Markdown、Hugo、PDF 与纯 Go EPUB3 导出；EPUB 每章带"来源与核验"节（论断/引用/核验结论），阅读站章节页与之逐字一致。
- 出版层（ADR 29）：`publish` 在硬门（全书 final + 显式 license）之后写入不可变的版本化 release（manifest 内容哈希、provenance 含 AI 披露与去重来源清单）。结构变化升 major、内容修订升 minor；已发布版本不可覆盖。
- `jianwu site` 只从已发布 release 生成静态书架：目录、章节阅读页、EPUB 下载与 OPDS 订阅源；确定性输出，可部署到任意静态托管。

来源复核是辅助判断，人工审阅仍然必要。真实样书和学习效果尚待 [评估](docs/EVALUATION.md)。

## 开始使用

需要 Go 1.25+。当前修复尚未发布到 tag，从源码构建：

```sh
git clone https://github.com/iannil/jianwu
cd jianwu
go build -o ./bin/jianwu ./cmd/jianwu
./bin/jianwu --version
```

将 `bin/jianwu` 所在目录加入 PATH。默认配置使用 GLM、Gemini、Brave 和 Jina，按服务设置 API key；见 [完整入门指南](docs/getting-started.md)。

```sh
jianwu init my-workspace
cd my-workspace
jianwu new --tokens
# 从 books/ 查看实际生成的书名 slug，它由访谈主题决定
book_slug='replace-with-actual-book-slug'
jianwu expand "$book_slug" --all --tokens
jianwu factcheck "$book_slug" 01-01
# 有需要时 revise，然后再次 factcheck；阅读正文和来源后 review
jianwu review "$book_slug" 01-01
# 对每章完成复核与审阅后
jianwu finalize "$book_slug"
jianwu export "$book_slug" --target epub
# 出版（需要在 books/<slug>/meta.json 设置 license）：
jianwu publish "$book_slug" --dry-run
jianwu publish "$book_slug"    # 不可变 release，内含 EPUB 产物
jianwu site                    # 从已发布 release 生成静态书架 + OPDS
```

不要用多个 CLI 进程同时修改同一本书。写入操作前备份工作区。旧书缺少 citation_ids 时不会自动推断来源；备份后重新展开可补齐关联。

## Web 界面

偏好可视化操作时，用 `jianwu serve` 在本地启动 Web 界面（默认 http://127.0.0.1:8787，仅绑定 localhost）：

```sh
cd my-workspace
jianwu serve
```

浏览器中即可完成全部流程：初始化工作区、12 维设计访谈、生成大纲与章节框架、逐章展开/事实核查/修订/审阅、批量展开、定稿与导出下载，以及语料管理与配置查看。长耗时操作以后台任务运行，实时显示进度与日志；所有写操作在服务内串行执行，遵守与 CLI 相同的"同书不并发写"约束。

同一工作区也可以通过 `http://127.0.0.1:8787/api/v1/` 的 JSON API 集成到其他本地工具；CLI 命令与引擎层保持不变。详见[功能概览](docs/CAPABILITIES.md)。

## 开发与发布

```sh
go test -race ./...
go vet ./...
scripts/release_test.sh
scripts/release.sh 0.3.11 --dry-run
```

[发布流程](docs/RELEASING.md)生成本地二进制、构建信息与校验和，不自动打标签或推送。核心代码位于 `internal/`，服务本产品，尚无公共 Go SDK。

jianwu 不再为 mouqin 提供内核。[独立产品决策](docs/decisions/28-independent-product.md)替代旧 SaaS 路线；仓库内历史官网资产保留，本轮不迁移线上站点。

## 许可

代码：AGPL-3.0，见 [LICENSE](LICENSE)。内置 zhurongshuo 参考数据（`internal/archetypes/`、`internal/style/`、`internal/corpus/`）：© zhurong，仅内部使用，不可再分发。
