# Agent 接入层：MCP / API / RSS / SKILL（2026-10-03）

## 决定

jianwu 支持四种方式接入 AI agent，共享同一编排内核（`internal/server` 的引擎调用与 JobManager 串行约束），不另起第二套管线：

1. **MCP**：`jianwu mcp` 启动 stdio MCP 服务器（官方 `modelcontextprotocol/go-sdk`）。工具集覆盖完整创作-出版管线（create_book / expand / factcheck / revise / review / finalize / export / publish / site / job_status 等）。长任务走 job 模式（工具返回 job_id，agent 用 job_status 轮询），与 serve 同一单写者约束。
2. **API**：既有 `/api/v1` 正式化为 agent 契约（[AGENT_ACCESS](../AGENT_ACCESS.md)）。新增可选 Bearer token 认证（`--token` / `JIANWU_SERVER_TOKEN`）：启用后 `/api/v1` 全部要求 `Authorization: Bearer <token>`，SPA 静态资源不拦截；**绑定非 localhost 且未设 token 时拒绝启动**。
3. **RSS**：静态阅读站新增 `rss.xml`（RSS 2.0），与 OPDS 并列；每个已发布版本一个 item，EPUB 以 enclosure 附带。`--base <url>` 可选生成绝对链接。
4. **SKILL**：`jianwu skill [--dir <agent-skills-root>]` 安装内嵌 SKILL.md（含工作流、约束与人审闸门说明）到 `<dir>/jianwu/`，默认 `~/.agents/skills/`；`--stdout` 直接打印供重定向。

## 边界与原则

- **人审闸门不因 agent 放宽**：review/finalize/publish 工具照常暴露（agent 可代跑流程），但工具描述与 SKILL 明确要求 review 仅在向人类展示正文与 verdicts 并获明确批准后标记；`reviewed_by` 记录操作者标识。发布硬门（final + license）与版本不可变语义对 agent 一视同仁。
- **安全模型**：默认 localhost 免认证（本地单用户现状不变）；token 为远端/agent 共享场景的最小认证，非 localhost 无 token 拒绝启动；MCP stdio 是进程级信任（同机）。
- **grill 简化**：agent 的 create_book 不暴露逐题访谈——核心 7 维（topic/audience/goal/archetype/depth/length/language）显式传参，其余维度取设计树默认值。
- **四通道一致性**：四种方式消费同一 workspace 与同一引擎；任一通道发起的写操作都进同一类串行约束（serve 内嵌 JobManager，MCP 自持 JobManager，CLI 单进程）。

## 非目标

- 不做 OpenAPI 规范文件、多用户账号、API 速率限制（后续按真实需求）。
- MCP 不做 SSE/streamable HTTP 传输（stdio 覆盖本地 agent；远端经 API）。
- RSS 不做增量/分页（书架规模小，全量 feed 足够）。
