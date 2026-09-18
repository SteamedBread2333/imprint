# 智能编辑器初始化

> **上手：** [README.zh.md](../README.zh.md#快速开始) — 安装与 `imprint init`。  
> 本文是**参考**（各编辑器路径、参数、Codex 合并规则）。

`imprint init` 写入各编辑器规则；**不写** MCP 配置。

```bash
imprint init              # 全部编辑器
imprint init --cursor     # 仅某一编辑器
imprint init --force      # 覆盖 / 刷新
```

## 写入路径

| 编辑器 | 标志 | 项目内路径 | 说明 |
| --- | --- | --- | --- |
| **Cursor** | `--cursor` | `.cursor/rules/imprint-memory.mdc` | `alwaysApply: true` |
| **Claude Code** | `--claude` | `.claude/rules/imprint-memory.md` | 无 `paths` → 每会话加载 |
| **Codex** | `--codex` | 见下节 | 官方 `AGENTS.md` 机制 |
| **Trae** | `--trae` | `.trae/rules/imprint-memory.md` | `alwaysApply: true` |
| **Workbuddy / CodeBuddy** | `--workbuddy` | `.codebuddy/rules/imprint-memory/RULE.mdc` | 模块化 RULE.mdc |

共享正文来自 `internal/cli/templates/body.md`；各编辑器仅 frontmatter / 路径不同。

## Codex（官方 AGENTS.md 机制）

Codex **没有**类似 `.cursor/rules/` 的独立规则目录。它在每个目录层只加载**一个**指令文件（[官方文档](https://developers.openai.com/codex/guides/agents-md)）：

1. 若存在且非空的 **`AGENTS.override.md`** → 只用 override（同目录的 `AGENTS.md` **不会**再加载）
2. 否则用 **`AGENTS.md`**
3. 从仓库根到当前工作目录逐层拼接，**整文件**进入上下文

因此 `imprint init --codex` **不会**写 HTML 注释或单独 sidecar 文件，而是：

| 情况 | 行为 |
| --- | --- |
| 目标文件不存在 | 创建 `AGENTS.md`（或写入已存在的非空 `AGENTS.override.md` 路径），内容为 `## imprint memory` 章节 |
| 已有 `## imprint memory` | 只替换该章节（`imprint init --codex --force` 可刷新） |
| 已有文件但无该章节 | 默认拒绝；`--force` 在末尾追加章节 |

**目标文件选择：**

- 根目录有非空 `AGENTS.override.md` → 写入 **override**（Codex 实际读的那份）
- 否则 → 写入 **`AGENTS.md`**

这与 Codex 文档里「Code Review Rules 等用 `##` 章节叠在同一份 AGENTS 文件里」的做法一致。章节内的 Must do / Must not 用 `###`，以便与仓库里其他 `##` 章节区分。

**注意：** 不要在已有团队 `AGENTS.md` 时随意新建 `AGENTS.override.md`——override 会**顶替**同目录的 `AGENTS.md`，导致团队原文不被 Codex 加载。

## MCP 挂载（手动）

`init` **不写** `.cursor/mcp.json`、`.mcp.json` 等。各编辑器 MCP 示例见 [mcp.zh.md](mcp.zh.md)。

| 编辑器 | 常见 MCP 配置位置 |
| --- | --- |
| Cursor | `.cursor/mcp.json` |
| Claude Code | `~/.claude.json` 或项目 MCP 设置 |
| Codex | `~/.codex/` 或 ChatGPT 开发者设置 |
| Trae | 智能体 / MCP 面板 |
| Workbuddy | `.codebuddy/.mcp.json` 或 `.mcp.json` |

## 示例：新项目一次到位

```bash
go install github.com/SteamedBread2333/imprint/cmd/imprint@latest
cd your-project
imprint init
# 可选：合并 docs/examples/cursor-mcp.json 到 .cursor/mcp.json
```

## 参见

- [correction.zh.md](correction.zh.md) — 写入循环与场景示例  
- [mcp.zh.md](mcp.zh.md) — MCP 工具与挂载  
- [editors.md](editors.md) — English
