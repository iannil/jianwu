# Agent 接入指南（ADR 30）

jianwu 向 AI agent 开放四种接入方式，共享同一工作区与同一引擎编排。安全模型：默认 localhost 免认证（本地单用户）；远端必须 Bearer token；`review` 是人工闸门、发布硬门（final + license）对 agent 不放宽。

## 1. MCP（推荐）

```sh
jianwu --dir <workspace> mcp
```

stdio 传输，注册为 agent 的 MCP server。工具集（16 个）：

| 工具 | 说明 |
|---|---|
| `workspace_status` / `list_books` / `book_status` / `read_chapter` | 只读：工作区、书列表、章节状态（含 verdicts/未核验数）、正文+引用+核验 |
| `create_book` | topic 必填，audience/goal/depth/length/language/archetype 可选（默认 beginner/understanding/intro/short/zh）；同步等待大纲+框架 |
| `expand_chapter` / `expand_all` / `factcheck` / `revise` | 长任务：返回 `job_id`，用 `job_status` 轮询 |
| `review_chapter` | **人工闸门**：必须先向用户展示正文与核验结论并获明确批准；`reviewer` 必填（如 `agent:名字`） |
| `finalize` | 全部 reviewed 后定稿（dry_run 可预检） |
| `export_book` | md / hugo / pdf / epub（后台任务） |
| `publish` | 发布门 dry_run 同步返回；正式发布为后台任务；license 是用户决策，不代填 |
| `list_releases` / `generate_site` / `job_status` | 版本清单 / 静态阅读站 / 任务轮询 |

同一串行队列约束适用：不要对同一本书并行发起写任务。

## 2. HTTP API

```sh
jianwu serve --addr 127.0.0.1:8787            # 本地（默认免认证）
jianwu serve --addr 0.0.0.0:8787 --token XXX  # 远端（必须 token）
```

端点在 `/api/v1/`（workspace / books / chapters / grill / corpus / jobs / publish / releases / site），完整清单见 [CAPABILITIES](CAPABILITIES.md)。启用 token 后所有 `/api/v1` 请求需带 `Authorization: Bearer <token>`（也可用环境变量 `JIANWU_SERVER_TOKEN`）；绑定非 localhost 且无 token 会拒绝启动。

典型 agent 循环：`POST /books/{slug}/expand` → 202 `{job_id}` → 轮询 `GET /jobs/{id}` 直到 `succeeded/failed`。

## 3. RSS / OPDS

`jianwu site [--base https://your-host]` 产出的静态站带两个订阅源：

- `rss.xml` — RSS 2.0，每个已发布版本一个 item（EPUB 以 enclosure 附带，披露未核验论断数）
- `opds.xml` — OPDS 1.x acquisition feed，阅读器 App 可直接发现并下载

agent 可轮询 rss.xml 感知新版本；`--base` 生成绝对链接。

## 4. SKILL

```sh
jianwu skill                    # 安装到 ~/.agents/skills/jianwu/SKILL.md
jianwu skill --dir <agents-dir> # 指定技能根目录
jianwu skill --stdout           # 打印后自行重定向
```

SKILL.md 内含工作流、四接入方式、闸门规则（review 人工批准、license 用户决策、保守失败语义、同书串行）与判断指引。

## 通用约束（所有通道一致）

- **review = 人工批准**：agent 代执行前必须获得用户对正文+核验结论的明确同意，并如实记录 reviewer。
- **发布硬门**：全书 final + `meta.json` 的 `license`；未核验论断会以警告披露进 manifest，不隐藏。
- **保守失败**：来源不可读 → 论断"未核验/未通过"，≠ 内容错误；不要据此盲目重写。
- **同书单写者**：长任务全部进串行队列；跨进程不要并发写同一本书。
