# 第一轮样书评估

状态：等待云端凭据。用户选择配置云端模型与搜索密钥后再跑完整评估；不启动 Ollama 或执行本地试写。没有已生成样书，也没有读者结果。

## 运行前配置

将 secrets.example.yaml 复制到 ~/.config/jianwu/secrets.yaml，填入至少一个受支持的云端模型密钥（Gemini 或 GLM）和 Brave Search 密钥；有 Jina Reader 密钥时一并填写。目标文件权限设为 0600。完整凭据只保留在本地，不提交到仓库，也不发到对话中。

配置完成后，评估将根据实际可用 Provider 固定各阶段模型，并先做连通性检查；不会默认要求同时拥有 Gemini 和 GLM 两套凭据。

## 环境预检

- 仓库：0.3.6-dev，存在本轮未提交修改。
- 云端：GEMINI_API_KEY / GLM_API_KEY / BRAVE_API_KEY / JINA_API_KEY 均未配置；secrets.yaml 不存在。
- 本地：Ollama 可执行文件已安装；11434 端口未运行服务。
- 本地模型 manifest：qwen3:14b、qwen3.8:27b-mlx、bge-m3:latest；manifest 存在不等于模型加载成功。
- 当前 CLI 的 expand 会要求搜索 Provider 密钥。仅配置 Ollama 无法完成默认联网流程；不得把无来源的本地试写记为“来源复核通过”。

## 固定样书任务

每本目标为中文短书、3–5 章。保留首次生成结果与错误，不通过静默重跑挑选最好的一次。

| ID | 固定核心问题 | 受众与目标 | 原型 |
|---|---|---|---|
| probability | 初学者如何区分条件概率与逆概率，避免忽略基准率？ | beginner / understanding / intro | foundations-application-practice |
| code-review | 一个小型软件团队如何执行可复现的代码审查，从准备、发现问题到验证修复？ | advanced-practitioner / operational / intermediate | diagnosis-decoding-breakthrough |
| retrieval-practice | 检索练习与重读相比，对长期学习保留有哪些证据、限制与适用条件？ | educated-general / understanding / intermediate | theory-dynamics-history-present |

## 运行规则

先用 probability 做单本试运行，确认真实模型请求、JSON 输出、章节数和失败恢复，再运行另外两本。每阶段记录：版本、模型、配置、开始/结束时间、退出码、调用量、缺失用量、文件路径。模型参数未确定前不填写生成结果。

本地试写与完整联网评估分开记账。无搜索凭据时，只能报告能够实际运行的生成/结构项目；证据准确性、真实账单、人工修订时间、读者学习效果均保留“未测量”。

## 初步内容检查

- 概率书：是否明确 P(A|B) 与 P(B|A)，是否给出基准率、联合概率和分母；例题计算能否复核。
- 审查指南：是否能按清单完成一次变更检查；是否覆盖测试、边界、失败处理及意见复验；是否区分阻塞项和偏好。
- 证据综述：来源是否对应具体研究；是否区分短期表现与长期保留；是否说明对象、任务、对照条件和局限。

读者测试题见 reader-tasks.md。运行与结论遵循 ../../EVALUATION.md。
