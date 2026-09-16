# Shelves

工作区文档索引跑在 **host 进程**（`imprint up` 或调试时 `imprint host serve`）和 **imprint-mcp** 里 — 不是外部插件。

## 配置

在 `.imprint/imprint.yaml`：

```yaml
host:
  listen: 127.0.0.1:9470

shelves:
  enabled: true
  config:
    roots:
      - docs
      - .cursor/skills
    # stateDir: .imprint/.shelves/.cache   # 可选
```

| 字段 | 含义 |
| --- | --- |
| `enabled` | `true` 时扫描 `roots` 并提供搜索；`false` 时停止索引与搜索，缓存只读保留 |
| `config.roots` | 工作区下要索引的目录（`.md`、`.mdc`、`.txt`）。默认 `docs` |
| `config.stateDir` | SQLite 缓存目录。默认 `.imprint/.shelves/.cache` |

改完后执行 `imprint up`（或调试时重启前台 `host serve`）。

## 禁用时

`shelves.enabled: false`：

- 不扫描、不 rebuild
- `POST /docs/search` → **503**
- MCP `find` 带 query 时只返回规则（无 `documents` 字段）
- `GET /docs/graph`、`/docs/chunks/:id`、`/docs/stats` 仍可读磁盘缓存

## Host API

默认：`http://127.0.0.1:9470`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/health` | 含 `shelves: { enabled, indexed, chunk_count, … }` |
| POST | `/docs/search` | `{ query, top_k? }` → `{ hits: [...] }` |
| GET | `/docs/chunks/{id}` | 完整 chunk |
| GET | `/docs/graph` | 文档关联图 |
| GET | `/docs/stats` | 索引元数据 |
| POST | `/docs/rebuild` | enabled 时强制 rebuild |

## MCP

只挂 **imprint-mcp**。shelves 启用且 `find` 带 query 时，响应可含：

```json
{
  "rules": [ ... ],
  "documents": [ { "id", "path", "heading", "score", "snippet" } ]
}
```

无 query 或 shelves 禁用时，响应为**规则数组**。

`get` 在 `id` 命中 shelves 索引时返回 chunk；否则按 vault 规则加载。

## Desk UI

desk 将 `/api/docs/*` 代理到 host。页眉显示 shelves 开/关（来自 `GET /health` → `shelves.enabled`）。
