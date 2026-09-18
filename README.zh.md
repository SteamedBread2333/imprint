<div align="center">

<img alt="imprint logo" width="200" src="assets/logo.png" />

# imprint

**智能体长期记忆 · 可携带的 markdown**

[English](README.md) · 中文

[![release](https://img.shields.io/github/v/release/SteamedBread2333/imprint?include_prereleases&style=flat-square)](https://github.com/SteamedBread2333/imprint/releases)
![Go 1.25+](https://img.shields.io/badge/go-1.25+-00ADD8?style=flat-square)
![MIT](https://img.shields.io/badge/license-MIT-c4a574?style=flat-square)

</div>

用户纠正变成 `.imprint/memory/` 里的持久规则，下次写代码前召回。**你正常说话即可，vault 由智能体维护。**

---

## 快速开始

```bash
go install github.com/SteamedBread2333/imprint/cmd/imprint@latest
go install github.com/SteamedBread2333/imprint/cmd/imprint-mcp@latest
cd your-project && imprint init
```

可选：把 [docs/examples/cursor-mcp.json](docs/examples/cursor-mcp.json) 合并进 `.cursor/mcp.json` 并重启编辑器。不用 MCP 时，智能体可直接走 `imprint --json`。

| | 你 | imprint |
| --- | --- | --- |
| **1** | 安装 + `init` | 写入编辑器规则（Cursor、Claude Code、Codex、Trae、Workbuddy） |
| **2** | 正常写代码、正常纠正 | 智能体 `find`（imprint + shelves）→ 分类 → `add`/`reinforce`/…（可选 `sources`） |
| **3** | 浏览器审计（可选） | `imprint up` → `imprint desk open` |

```mermaid
flowchart TB
  U((用户<br/>正常说话))

  subgraph Agent["智能体 · imprint-mcp"]
    direction TB
    F["find(scope, query)"]
    C{"分类<br/>ADD · REINFORCE · SUPERSEDE · IGNORE"}
    W["add / supersede<br/>可选 sources"]
    R["写代码前再 find · cite [r-id]"]
  end

  subgraph Store["imprint 存储"]
    direction LR
    Vault[(".imprint/memory/<br/>claim · evidence · sources")]
    Shelves[(".shelves/.cache/<br/>roots 文档 BM25")]
  end

  subgraph FindOut["find 一次返回"]
    direction LR
    FR["rules<br/>resolved_sources"]
    FD["documents<br/>snippet"]
    FL["links"]
  end

  subgraph Audit["可选 · 人工审计"]
    H((你)) --> Desk["desk · show"]
  end

  UP["imprint up"] -.->|host 索引| Shelves

  U -->|纠正 / 任务| Agent
  F --> Vault
  F --> Shelves
  Vault --> FR
  Shelves --> FD
  Vault --> FL
  Shelves --> FL
  FR & FD & FL --> C
  C -->|持久写入| W
  W --> Vault
  R --> F
  Desk --> Vault
  Desk --> Shelves
```

<details>
<summary>写代码前召回（sequence）</summary>

```mermaid
sequenceDiagram
  participant U as 用户
  participant A as 智能体
  participant V as vault
  participant S as shelves

  U->>A: 新任务 / 继续开发
  A->>V: find(scope, query)
  V->>S: BM25 文档（shelves 开）
  V-->>A: rules · documents · links
  A->>A: 按 imprint + excerpt 写代码
  Note over A: cite [r-id] 与 doc path
```

</details>

---

## CLI

全局参数：`--vault PATH` · `--global` · `--json`（stdout 输出 JSON，供智能体与脚本）

### 常用 — vault

直接读写。**不需要 host。**

| 命令 | 作用 |
| --- | --- |
| `imprint init` | 写入智能体规则；不写 MCP 配置 |
| `imprint find [--scope a,b] [--query TEXT]` | 召回 imprint（scope **AND**）；shelves 开且带 query → 含 documents、links |
| `imprint add CLAIM --scope a,b --text ORIG` | 新建 imprint（MCP 可带 `sources` 关联文档） |
| `imprint reinforce ID [--evidence TEXT]` | 加强规则（置信度 +0.1） |
| `imprint supersede OLD --claim NEW --scope a,b` | 替换规则；旧规则标记 `superseded` |
| `imprint forget ID` | 永久删除 |
| `imprint list` · `show` · `get ID` | 浏览与查看 |
| `imprint sweep` · `export` | 衰减陈旧规则 · 导出 JSON |

```bash
imprint find --scope go,naming --query PascalCase
imprint add "函数名用 snake_case" --scope python,naming --text "用户要求 snake_case"
imprint get r-2026-09-11-001
```

### 常用 — 本地服务

开 desk、看 vault 图谱时，**记这两条就够了**：

| 命令 | 作用 |
| --- | --- |
| `imprint up` | 启动本地服务（vault API、shelves、已启用的 desk 等） |
| `imprint down` | 停止本地服务 |
| `imprint desk open` | 浏览器打开 desk（**不启动**服务；需先 `up`） |
| `imprint status` | 本地服务快照 |

```bash
imprint up && imprint desk open
# 用完：
imprint down
```

改 `imprint.yaml` 后：`down` 再 `up` 即可生效。

<details>
<summary>进阶：拆开控制 host / 插件（调试）</summary>

| 命令 | 作用 |
| --- | --- |
| `imprint host start` / `host stop` | 仅 host（含 shelves） |
| `imprint plugin start` / `plugin stop` | 仅外部插件 |
| `plugin list` · `enable` · `disable` | 改 yaml 里的插件开关 |

日常用 `up` / `down` 即可，不必记这些子命令。

</details>

shelves 是 **host 配置**（顶层 `shelves:`），不是插件。见 [docs/shelves-builtin.zh.md](docs/shelves-builtin.zh.md)。

### 调试与进阶

| 命令 | 作用 |
| --- | --- |
| `imprint host serve [--listen ADDR]` | 前台 host（Ctrl+C）— 调试 API |
| `imprint migrate-shards [--dry-run]` | 将旧 `imprint-*.md` 导入 `vault.db`（一次性） |
| `imprint clear --confirm --yes` | 删除全部规则 — 不可逆 |
| `imprint version` | 打印版本 |

完整参数：`imprint --help` 或 `imprint help <cmd>`。

---

## Vault 布局

默认 `./.imprint/memory/`（向上找 `.imprint/`）。`--global` → `~/.imprint`。可用 `--vault` 或 `IMPRINT_VAULT` 覆盖。

```
.imprint/
  imprint.yaml          # host + shelves.roots + plugins
  memory/vault.db       # SQLite vault（规则、证据、边、sources）
  .shelves/.cache/      # roots 下文档索引（SQLite，gitignore）
docs/                   # 常见 roots 之一
.cursor/rules/          # 常见 roots 之一
```

规则存于 `memory/vault.db`。ID：`r-YYYY-MM-DD-NNN`。状态：`active` | `dormant` | `superseded`。`supersede` 将旧规则标为 superseded；`sweep` 衰减至 dormant；`forget` 删除。交互式规则图：**`imprint desk open`**（host `GET /graph`）。

---

## Shelves 与 desk

| | 运行位置 | 配置 |
| --- | --- | --- |
| **Shelves** | **host** | `shelves` · [imprint.yaml](docs/examples/imprint.yaml) |
| **Desk** | 外部插件 | `plugins.desk` · [imprint-desk-plugin](https://github.com/SteamedBread2333/imprint-desk-plugin) |

**Shelves 做什么：** 在 `imprint.yaml` 的 `roots` 下建本地文档索引（默认如 `docs/`、`.cursor/rules/`），不是「LLM 已读过整个仓库」。Agent 写代码前 `find(scope, query)` 可**一次**拿到 vault 里的 imprint、相关文档段落（snippet）、以及 `links`。BM25 本地索引，无 embedding API。

**为何需要 `roots`：** 只有列进 `roots` 的目录会被索引和搜索；用来划定 Agent 应对照的项目文档范围，并控制 rebuild 成本。详见 [docs/shelves-builtin.zh.md](docs/shelves-builtin.zh.md)。

**与 vault 关联：** 用户纠正后，Agent 可在 ADD 时写 vault 的 `sources`（规则→文档），**不必改项目 markdown**。`get r-…` 返回 `resolved_sources`。关联设计：[docs/imprint-shelves-linking.zh.md](docs/imprint-shelves-linking.zh.md)。

只挂一个 MCP（`imprint-mcp`）。shelves 开启且 `find` 带 query 时，响应含 `rules`、`documents`、`links`（规则带 `resolved_sources` 时含 excerpt）。

---

## 安装与发布

```bash
docker pull ghcr.io/steamedbread2333/imprint:latest   # 或 GitHub Releases 二进制
make install          # 从源码
make publish V=X.Y.Z  # 打 tag，CI 上传 Release + GHCR
```

未打 release tag 的本地构建显示 `devel`。

---

## 文档

| 主题 | 中文 | English |
| --- | --- | --- |
| MCP 挂载 | [docs/mcp.zh.md](docs/mcp.zh.md) | [docs/mcp.md](docs/mcp.md) |
| 编辑器 `init` | [docs/editors.zh.md](docs/editors.zh.md) | [docs/editors.md](docs/editors.md) |
| Shelves 与关联 | [docs/shelves-builtin.zh.md](docs/shelves-builtin.zh.md) · [docs/imprint-shelves-linking.zh.md](docs/imprint-shelves-linking.zh.md) | [docs/shelves-builtin.md](docs/shelves-builtin.md) · [docs/imprint-shelves-linking.md](docs/imprint-shelves-linking.md) |
| 纠偏与编码场景 | [docs/correction.zh.md](docs/correction.zh.md) | [docs/correction.md](docs/correction.md) |

MCP 示例（需手动合并）：[cursor-mcp.json](docs/examples/cursor-mcp.json) · [cursor-mcp-global.json](docs/examples/cursor-mcp-global.json)

---

## Go 模块

```go
import "github.com/SteamedBread2333/imprint/pkg/imprint"

v, _ := imprint.Open("./.imprint/memory")
v.Add("Use gofmt", []string{"go"}, "gofmt", 0.6)
v.Find([]string{"go"}, "", 5)
```

分类（ADD / REINFORCE / SUPERSEDE / IGNORE）和「该不该写」由**智能体**判断。imprint 只负责存、召回、强化、替换、衰减。

MIT
