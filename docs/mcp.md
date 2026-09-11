# 安装 imprint MCP（发布之后）

GitHub **没有 Go 的 Packages 仓库**。这个仓库发布后你会得到三样东西，MCP 用其中的**二进制**或 **GHCR 容器**：

| 产物 | 在哪 | MCP 怎么用 |
|---|---|---|
| `imprint` 命令 | `go install …@vX.Y.Z` 或 GitHub Release 压缩包 | Cursor `command`: `imprint`（推荐） |
| 容器 | GitHub Packages / `ghcr.io/<owner>/imprint` | Cursor `command`: `docker` |
| Go module | git tag | `import` / `go install`，不是 MCP 本身 |

## 1. 装命令

```bash
# 需要 Go 1.23+
go install github.com/SteamedBread2333/imprint/cmd/imprint@latest
```

确认 `imprint` 在 PATH 里（通常是 `$(go env GOPATH)/bin`）：

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
imprint version
```

或不装 Go，从 [GitHub Releases](https://github.com/SteamedBread2333/imprint/releases) 下载对应平台的 `.tar.gz` / `.zip`，把 `imprint` 放到 PATH。

容器（GitHub Packages）：

```bash
docker pull ghcr.io/steamedbread2333/imprint:latest
```

若 Packages 是 private，先 `docker login ghcr.io`；公开仓库请在 GitHub → Packages → 该镜像 → Package settings 设为 **Public**。

## 2. 在项目里接入 Cursor MCP

在**你要记住偏好的那个项目根目录**执行：

```bash
imprint init
```

会写入（已存在则加 `--force`）：

- `.cursor/mcp.json` — stdio 调用 `imprint --vault ./memory mcp`
- `.cursor/rules/imprint-memory.mdc` — `alwaysApply`，让 Agent 按协议 find / 分类后再写

改用 GHCR 容器而不是本地二进制：

```bash
imprint init --docker --force
```

然后：

1. Cursor **Settings → MCP** 重载（或重启 Cursor）
2. 工具列表里应出现 `imprint_find`、`imprint_add` 等
3. 不要在终端前台空跑 `imprint mcp` 来「接上」Cursor，那只会占用 stdin

Vault 默认是项目下的 `./memory/`。全局偏好用 `--global`（`~/.imprint`）或改 `mcp.json` 里的 `--vault` / `IMPRINT_VAULT`。

## 3. 手写配置（不用 init 时）

`.cursor/mcp.json`：

```json
{
  "mcpServers": {
    "imprint": {
      "command": "imprint",
      "args": ["--vault", "./memory", "mcp"]
    }
  }
}
```

若 Cursor 找不到命令，把 `command` 换成 `which imprint` 的绝对路径。

规则文件复制自仓库 [`.cursor/rules/imprint-memory.mdc`](../.cursor/rules/imprint-memory.mdc)，必须 `alwaysApply: true`。只接 MCP、不放规则时，Agent 有工具但不会稳定地先 find 再写。

## 4. 检查

```bash
imprint --vault ./memory list
```

Agent 会话里应能调用 `imprint_find`。若没有工具：看 MCP 面板日志、PATH、以及 `mcp.json` 的 `command` 是否指向已发布的二进制。
