# 测试与验收

> **上手：** [README.zh.md](../README.zh.md#快速开始)。本文说明如何复现真实使用场景验收。

## 覆盖范围

`cmd/imprint-acceptance` 在临时项目里调用已安装的 `imprint` / `imprint-mcp`，不碰开发者 vault。场景包括：

1. `imprint init` 目录
2. 经 Linguist 语言别名（`ts` / `tsx`）用 `Camel Case` 召回 `camelCase`
3. 重复写入与隐私拒绝
4. 空 evidence 与明确重申
5. find 只更新召回统计
6. sweep → dormant 候选 → reinforce
7. supersede / forget 审计
8. 并发 CLI 写入
9. MCP 紧凑 find/get
10. telemetry 隐私与保留
11. `imprint report --days 30`
12. 关闭后再打开

## 运行

```bash
go install ./cmd/imprint ./cmd/imprint-mcp
go run ./cmd/imprint-acceptance --report .imprint/export/acceptance-report.md
```

报告是给人看的中文 Markdown，不会回显 claim、evidence 或 secret 原文。失败时仍写报告并以非零状态退出。

单元、race 与文档契约：

```bash
go test ./...
go test -race ./...
go test ./internal/cli -run TestDocsContract
```
