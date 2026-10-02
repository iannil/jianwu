# probability 单本试运行记录（2026-10-03）

> 按 [2026-09-27 评估准备](../2026-09-27/README.md) 运行规则执行的第一本试运行。
> 状态：**机器部分完成（new → expand → factcheck），人工部分未开始**（论断抽查、修订决策、review、读者测试）。

## 固定条件

- 二进制：`jianwu 0.3.11 (commit 3a00aac…-dirty, built 2026-10-02T17:00:04Z)`（README 未提交改动导致 dirty，如实记录）
- 工作区：`~/Code/zhurong/jianwu-eval`（独立评估工作区，未触碰全局配置与 `~/Developer/jianwu`）
- 模型：**deepseek-flash 全阶段**（intake/outline/scaffolding/expand/factcheck/revise）
- 搜索：serper（primary+fallback）；阅读器：jina **无 key**（免费额度）
- 固定任务参数（访谈显式作答）：beginner / understanding / intro / short / zh / foundations-application-practice；topic = 初学者如何区分条件概率与逆概率，避免忽略基准率；其余 4 维（scope/example_type/visualization/timeliness）接受 AI 推荐
- 运行时间窗：2026-10-02T17:08:44Z – 17:24:17Z（约 15.5 分钟，含环境配置）

## 环境发现（运行前）

1. **glm key 余额不足**（HTTP 429 code 1113"余额不足或无可用资源包"）→ 按运行规则改用 deepseek（与作者日常生产配置一致的模型族）。首版 glm-4.6 配置未产生任何 LLM 调用成功。
2. gemini/brave 无 key，init 模板默认值（outline/scaffolding=gemini、search=brave）不可用 → 已在评估工作区显式固定。

## 运行记录（全部命令退出码 0，无重跑）

| 阶段 | 耗时 | LLM 调用 | tokens (in/out) | 备注 |
|---|---|---|---|---|
| grill 访谈（10 维） | 16.8s | ~11 | 计入 new 汇总 | citation_style 维未被触发 |
| outline + scaffolding | 30s | 计入 16 | （见下） | job-3e3ffc1e |
| expand 01-01 | 97s | 3 | 16189/12172 | 1461 字 · 8 引用 |
| expand 01-02 | 75s | 3 | 14005/9473 | 1236 字 · 4 引用 |
| expand 02-01 | 103s | 3 | 14674/12618 | 1531 字 · 6 引用 |
| expand 03-01 | 116s | 3 | 14811/12864 | 1333 字 · 7 引用 |
| expand 04-01 | 124s | 3 | 13264/21622 | 1457 字 · 5 引用 · **28 条未核验论断** |
| factcheck 01-01 | 68s | 8 | 18845/3450 | 混合结论（含 ✓ 与 ✗） |
| factcheck 01-02 | 35s | **0** | 0 | 来源全部读取失败 → 全部保守未通过 |
| factcheck 02-01 | 40s | **0** | 0 | 同上 |
| factcheck 03-01 | 42s | 1 | 1349/651 | 大部分来源读取失败 |
| factcheck 04-01 | 23s | 6 | 10570/2018 | 部分来源可读 |

**书级累计（status）**：210,714 tokens（in 122,637 / out 88,077），46 次调用，0 次缓存，0 次缺失用量报告。5 章 4 部共 7,018 字、30 条脚注定义。实际账单**未测量**（以 Provider 账单为准；搜索/阅读器费用未含）。

## 运行中问题（如实记录，不静默重跑）

1. **jina 免费额度中途耗尽**：expand 阶段约 30 次 URL 读取全部成功；进入 factcheck 后读取开始失败（"source could not be read"，含 Wikipedia 等来源）。02-01/01-02 两章 0 次 LLM 调用、全部论断保守未通过——数学恒真式（乘法公式）也被标 ✗，这是"来源不可读 ≠ 论断错误"的保守设计，**不能**按字面理解为内容错误。修正评估环境的动作：配置 `JINA_API_KEY` 后可显式重跑 factcheck 轮（重跑会记录在案）。
2. **04-01 未核验论断密度异常**（28 条 vs 其他章 4–6 条）：实战复盘章包含大量具体数字/案例论断但引用仅 5 条。这是"初步内容检查"的重点章。

## 未完成（人工部分，按 EVALUATION.md）

- 每章 ≥30 条论断抽查（或全查）、来源原文比对：**未开始**
- 术语冲突/重复段落/无依据数字检查：未开始
- revise 决策与再次 factcheck：未开始（当前 verdicts 因 reader 问题不可靠，建议先补 JINA key）
- review / finalize / publish：未开始
- 3 位真实读者前后测：未开始；实际账单记录：未测量

## 工件

- 书：`~/Code/zhurong/jianwu-eval/books/b-e70ef579ef3bf737/`（slug 由长主题哈希生成）
- grill 会话：`s-19cad58d-2af0-45a2-9d75-92bee5b14ce8`；生成任务：`job-3e3ffc1e`
- 阶段日志：`/tmp/eval-*.log`（临时，可能被清理）
