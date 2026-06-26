# 萃取记录：从 zhurongshuo 萃取 jianwu v0.1 内容资产

> 本文档记录 v0.1 前置工作（DESIGN.md 第 13 节）的执行情况，
> 便于祝融审阅修改时知道每个文件的依据和信心程度。

执行日期：2026-06-21
执行者：AI 辅助（Claude Code + zhurongshuo 文本反推）
审阅者：祝融
状态：**v0.1.0 ship 时已通过 7 个切片的 SDD review 验证**。低信心（标记 medium）的 corpus abstract 仍可后续修订（v0.2 `corpus sync` 时）。

---

## 一、产出物清单

### 1. 原型库（3 个 YAML）

| 文件 | 原型 ID | 信心 | 主要依据 |
|---|---|---|---|
| `internal/archetypes/ontology-epistemology-practice.yaml` | 本体-认识-实践型 | 高 | reality-construction（4 部）+ advancement-of-reality（3 部） |
| `internal/archetypes/diagnosis-decoding-breakthrough.yaml` | 诊断-解码-破局型 | 高 | silent-games（3 部）+ forced-convergence（4 部） |
| `internal/archetypes/foundations-application-practice.yaml` | 基础-应用-实战型 | 高 | ai-engineer-in-action（3 部）+ intelligent-computing-center-construction-guide（4 部） |

每个原型含：完整 schema（id / 双语 name / description / when_to_use / parts / examples / metadata）；每部含 role / title_template / guidance / typical_chapters / chapter_role_hints。

### 2. 风格规约（1 个 markdown）

| 文件 | 内容 | 信心 |
|---|---|---|
| `internal/style/style-guide.md` | 硬规则 + 软偏好 + 原型差异化语调 + 反例集 + 自检清单 | 中-高 |

萃取自三本代表书的 introduction 和第一章正文。

### 3. Few-shot 样例段落（3 个 markdown）

| 文件 | 段落数 | 来源 | 信心 |
|---|---|---|---|
| `internal/style/samples/ontology-epistemology-practice.md` | 5 段 | reality-construction | 高 |
| `internal/style/samples/diagnosis-decoding-breakthrough.md` | 5 段 | silent-games | 高 |
| `internal/style/samples/foundations-application-practice.md` | 5 段 | ai-engineer-in-action | 高 |

每段标注：出处、体现的风格特征、段落正文。

### 4. 参考语料（6 个 JSON）

| 文件 | 来源书 | 信心 | 备注 |
|---|---|---|---|
| `internal/corpus/builtin/reality-construction.json` | reality-construction | 高 | 4 部 12 章，已读 introduction 和 chapter-01 |
| `internal/corpus/builtin/advancement-of-reality.json` | advancement-of-reality | 中 | 3 部 13 章，仅读章节标题，abstract 基于 part_titles + 同系列书推断 |
| `internal/corpus/builtin/silent-games.json` | silent-games | 高 | 3 部 8 章，已读 introduction 和 chapter-01；part-02/03 的章节 abstract 基于书名推断（zhurongshuo 实际章节标题未取） |
| `internal/corpus/builtin/forced-convergence.json` | forced-convergence | 中-高 | 4 部 12 章，章节标题已取，abstract 基于标题推断 |
| `internal/corpus/builtin/ai-engineer-in-action.json` | ai-engineer-in-action | 高（introduction 部分）/ 中（章节） | 3 部 13 章，已读 introduction；章节标题是常见 AI 工程书命名约定（zhurongshuo 实际未取） |
| `internal/corpus/builtin/intelligent-computing-center-construction-guide.json` | intelligent-computing-center-construction-guide | 中-高 | 4 部 13 章，章节标题已取 |

### 5. 目录结构

```
internal/
  archetypes/
    ontology-epistemology-practice.yaml
    diagnosis-decoding-breakthrough.yaml
    foundations-application-practice.yaml
  style/
    style-guide.md
    samples/
      ontology-epistemology-practice.md
      diagnosis-decoding-breakthrough.md
      foundations-application-practice.md
  corpus/
    builtin/
      reality-construction.json
      advancement-of-reality.json
      silent-games.json
      forced-convergence.json
      ai-engineer-in-action.json
      intelligent-computing-center-construction-guide.json
```

---

## 二、需要重点审阅的地方

### 2.1 原型 schema 字段（重要）

字段命名和结构是后续 jianwu 代码直接读的契约。重点审阅：

- `parts[].role`：每个原型的 role 名（如 `ontology`、`diagnosis`、`foundations`）是否合适？是否需要换名？
- `parts[].title_template`：模板字符串里的 `{n}` / `{topic}` / `{subtitle}` 占位符约定是否认可？
- `parts[].guidance`：每部的写作指引是否准确捕捉到了该部的本质？有没有该写没写的？
- `parts[].chapter_role_hints`：每章的角色提示是否合理？是否过多/过少？
- `parts[].conditional`（diagnosis-decoding-breakthrough 的 context 部有此字段）：标记可选部的设计是否认可？
- `when_to_use.goals / topic_types / audience_fit / not_recommended_for`：分类值是否合适？是否需要扩展？

### 2.2 风格规约的硬规则（重要）

`style-guide.md` 的"硬规则"部分会被 LLM 严格执行。重点审阅：

- 第一节"开头禁忌"列的几个反例模式，是否覆盖了 zhurongshuo 不写的所有开头方式？
- "词汇禁忌"里的禁词列表是否准确？有没有遗漏或误伤？
- "标点偏好"——「」用法的描述是否符合你的实际写法？
- "结构禁忌"——"不重复同一观点三次以上"这种约束是否过于严格？

### 2.3 风格规约的软偏好（中等）

软偏好是倾向性指引，不会强制重写。审阅时看大方向是否对：

- 长句优于短句堆叠的倾向——是否符合你的写作直觉？
- 术语处理的「」+ 定义模式——是否符合你的实际操作？
- "比喻承担解释职责而非装饰"——这条软偏好的边界是否清楚？

### 2.4 Few-shot 样例段落（重要）

样例段落会被注入到 LLM 的 prompt 里，直接影响生成质量。重点审阅：

- 每段是否准确代表了该原型的写作风格？
- 有没有不希望被 LLM 模仿的段落特征（如过长、过理论化等）被收进来了？
- 段落顺序是否合理？通常把最具代表性的放在前面。
- 是否需要补充某些类型的段落（如"如何结尾"、"如何过渡"、"如何用引文"）？

### 2.5 Corpus 的章节 abstract（中等）

低信心（标记为 `medium`）的几本参考书：

- `advancement-of-reality.json`：第 5-13 章 abstract 基于 part_titles + 同系列书推断，可能偏离实际内容。
- `silent-games.json`：part-02/03 章节标题是我推测的（"稳定型机器/增长型机器/颠覆型机器"），zhurongshuo 实际章节标题未取。
- `forced-convergence.json`：章节标题已取但 abstract 基于标题推断。
- `ai-engineer-in-action.json`：章节标题是我基于常见 AI 工程书命名约定推测的（"第1章 Python 工程化基础"等），zhurongshuo 实际章节标题未取。

建议：
- 如果这些 abstract 不准，可以删除（保留 slug + title + part 结构即可，abstract 留空）
- 或者补充读取实际章节后再修订

### 2.6 Extraction 系统提示词（DESIGN.md 第 13.1 节）

按 DESIGN.md 计划，应该写一个 `scripts/extract-archetypes.go` 脚本，读 zhurongshuo 数据喂 LLM 萃取。实际我没写这个脚本，而是直接用 Claude Code 读 zhurongshuo 文本萃取（`scripts/` 目录已于 v0.1.1-post 删除）。

理由：
- 一次性脚本对单次萃取是过度工程
- 直接读 + 萃取更快、质量更高（我能深度理解上下文）
- 脚本的核心价值在于"未来 corpus sync 时复用"，那时再写不迟

如果你希望补上脚本（作为 `jianwu corpus sync` 的实现基础），告诉我，我可以补一份。

---

## 三、已知缺口（v0.1 不阻塞，v0.1.x 可补）

### 3.1 后 3 个原型

DESIGN.md 计划 v0.1 做 3 个，v0.1.x 补 3 个：

- `micro-meso-macro`（微观-中观-宏观型）
- `theory-dynamics-history-present`（理论-动力-历史-当下型）
- `mindset-method-practice`（心法-方法-实战型）

参考书候选：
- micro-meso-macro → data-as-the-boundary
- theory-dynamics-history-present → revisiting-history
- mindset-method-practice → open-map / barbaric-order

### 3.2 Few-shot 样例的多样性

目前每原型只有 1 本来源书的样例。理论上应该 2 本（每原型有 2 本代表书）。可以补：

- ontology-epistemology-practice 补 advancement-of-reality 的样例
- diagnosis-decoding-breakthrough 补 forced-convergence 的样例
- foundations-application-practice 补 intelligent-computing-center-construction-guide 的样例

### 3.3 Embedding 索引

DESIGN.md 第 7.3 节提到 `~/.local/share/jianwu/corpus/index/` 存 embedding 索引。这要等 jianwu 代码写好后用 `jianwu corpus reindex` 命令生成。当前 6 个 JSON 已经准备好作为索引源。

---

## 四、审阅建议的工作流

1. **先快速浏览所有文件**（10 分钟），找出明显不对的地方
2. **重点审阅 2.1（原型 schema）和 2.4（few-shot 样例）**——这两个直接影响 v0.1 的生成质量
3. **修订时直接编辑文件**——所有文件都是普通文本，结构清晰
4. **修订完成后**，告知我（或 commit 一份"v0.1 资产定稿"），可以进入下一阶段（C：搭 jianwu Go 项目骨架）

---

## 五、文件路径速查

```
/Users/rong.zhu/Code/jianwu/
├── docs/
│   ├── EXTRACTION_NOTES.md                           # 本文件
│   ├── archive/DESIGN.md                             # 设计文档（v0.1 锁定）
│   └── ...
└── internal/
    ├── archetypes/
    │   ├── ontology-epistemology-practice.yaml
    │   ├── diagnosis-decoding-breakthrough.yaml
    │   └── foundations-application-practice.yaml
    ├── style/
    │   ├── style-guide.md
    │   └── samples/
    │       ├── ontology-epistemology-practice.md
    │       ├── diagnosis-decoding-breakthrough.md
    │       └── foundations-application-practice.md
    └── corpus/
        └── builtin/
            ├── reality-construction.json
            ├── advancement-of-reality.json
            ├── silent-games.json
            ├── forced-convergence.json
            ├── ai-engineer-in-action.json
            └── intelligent-computing-center-construction-guide.json
```
