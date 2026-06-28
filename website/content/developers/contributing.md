---
title: "参与贡献"
description: "代码规范、测试约定、PR 流程"
---

## 构建和测试

```bash
# 构建
go build -o ./bin/jianwu ./cmd/jianwu

# 运行全部测试
go test ./internal/...

# 单包测试
go test ./internal/cli/...

# 静态检查
go vet ./...
```

## 代码规范

- **命名：** Go 标准 camelCase，导出结构体加 JSON 标签（`json:"snake_case"`）
- **测试：** 表格驱动测试，命名 `TestXxx`，不用 testify/assert
- **错误处理：** `fmt.Errorf("context: %w", err)` 保持可解包
- **导入顺序：** 标准库 → 第三方 → 内部包，组间空行
- **没有全局状态：** 库包中不使用 `init()`（embed.go 除外）

## 提交 PR

1. Fork 仓库并创建分支
2. 确保全部测试通过
3. 确保 `go vet` 无误
4. 提交 PR 时附上变更说明

## 设计文档

关键的架构决策记录在 `docs/decisions/` 目录下，共 26 项核心决策。如有重大变更，建议先阅读相关决策记录。
