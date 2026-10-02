# jianwu 入门

jianwu 是独立的本地图书创作 CLI。需要 Go 1.25+。当前修复尚未发布到 tag，先从本仓库构建：

```sh
go build -o ./bin/jianwu ./cmd/jianwu
./bin/jianwu --version
```

将 `bin/jianwu` 所在目录加入 PATH 后：

```sh
jianwu init my-workspace
cd my-workspace
```

默认配置用 GLM 进行访谈和展开、Gemini 生成大纲和框架、Brave 搜索、Jina 读取来源。根据已开通的服务设置环境变量：

```sh
export GLM_API_KEY='your-glm-key'
export GEMINI_API_KEY='your-gemini-key'
export BRAVE_API_KEY='your-brave-key'
export JINA_API_KEY='your-jina-key'
jianwu config list
```

也可编辑 `.jianwu/config.yaml` 为各阶段选择已支持的模型。密钥不要提交到 Git。

```sh
jianwu new --tokens
```

回答主题、受众、目标等问题。书的 slug 根据主题生成，并不等于工作区目录名。查看 `books/` 下的新目录，把实际名称赋给变量：

```sh
book_slug='replace-with-actual-book-slug'
jianwu status "$book_slug"
jianwu expand "$book_slug" --all --tokens
jianwu status "$book_slug"
```

对每章运行以下流程（示例地址 `01-01`，其他章节从 status 查看）：

```sh
jianwu factcheck "$book_slug" 01-01 --tokens
# 若存在需要修改的论断：
jianwu revise "$book_slug" 01-01 --tokens
jianwu factcheck "$book_slug" 01-01 --tokens
# 阅读章节及原始来源，确认可接受后：
jianwu review "$book_slug" 01-01
```

所有章节完成审阅后：

```sh
jianwu finalize "$book_slug" --dry-run
jianwu finalize "$book_slug"
jianwu export "$book_slug" --target md
```

导出无需定稿也能执行，适合先看草稿。PDF 需要另行安装 pandoc 与 xelatex；EPUB（`--target epub`）为纯 Go 实现，无外部依赖，每章带"来源与核验"节。`--tokens` 显示用量但不影响记录；用量不完整时不会把未知消耗当作零费用。

## 发布与阅读站（可选）

定稿后可发布不可变的版本快照（ADR 29 出版层）。发布硬门要求全书 final 且 `books/<slug>/meta.json` 设置 `license`（建议同时设置 `author`）：

```sh
jianwu publish "$book_slug" --dry-run   # 查看发布门、下一版本与将产出的文件
jianwu publish "$book_slug"             # 写入 books/<slug>/releases/<X.Y>/（含 EPUB 产物）
jianwu site                              # 从已发布 release 生成静态阅读站 + OPDS
```

- `releases/` 内版本不可变：修改内容后需重新 review/finalize，再次 `publish` 自动递增版本（结构变化升 major、内容修订升 minor）。
- `site/` 目录（书架、章节阅读页、EPUB 下载、`opds.xml`）是派生状态，每次整体重建，可部署到任意静态托管；工作稿不会出现在书架上。
- 手工校验 EPUB 可运行 `scripts/epubcheck.sh <file.epub>`（需 Java + epubcheck，缺省退化为结构检查）。

运行写入命令前备份工作区；不要同时用多个 CLI 修改同一本书。旧书缺少 citation_ids 时，需要备份后重新展开以建立正文与来源关联。

## 用浏览器完成整个流程（可选）

不想记命令时，在同一工作区运行：

```sh
jianwu serve
```

打开输出的地址（默认 http://127.0.0.1:8787，仅本机可访问）。网页中可以完成入门指南的全部步骤：初始化工作区、回答 12 维设计访谈（AI 逐维推荐，可接受或修改）、生成大纲与章节框架、逐章展开/事实核查/修订/审阅、批量展开、定稿、导出并下载。生成类操作显示实时进度与日志；写操作在服务内排队串行执行。API key 与配置与 CLI 完全共用。

工作区位置可用三种方式配置（优先级从高到低）：命令行 `--dir`、环境变量 `JIANWU_WORKSPACE`、全局配置文件 `~/.config/jianwu/config.yaml` 的 `workspace:` 键；未显式配置时使用启动目录。也可以直接在网页的工作区卡片中切换并初始化。未配置工作区时，新建图书与生成操作会被拦截。
