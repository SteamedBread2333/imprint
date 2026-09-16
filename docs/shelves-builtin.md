# Shelves

Workspace documentation indexing runs **inside the host** (`imprint up`, or `imprint host serve` for foreground debug) and in **imprint-mcp** — not as an external plugin.

## Configuration

In `.imprint/imprint.yaml`:

```yaml
host:
  listen: 127.0.0.1:9470

shelves:
  enabled: true
  config:
    roots:
      - docs
      - .cursor/skills
    # stateDir: .imprint/.shelves/.cache   # optional
```

| Field | Meaning |
| --- | --- |
| `enabled` | When `true`, scan `roots` and serve search. When `false`, stop indexing and search; existing cache stays readable. |
| `config.roots` | Directories under the workspace to index (`.md`, `.mdc`, `.txt`). Default: `docs`. |
| `config.stateDir` | SQLite cache directory. Default: `.imprint/.shelves/.cache`. |

After changes, run `imprint up` (or restart a foreground `host serve` for debug).

## When disabled

`shelves.enabled: false`:

- No filesystem scan or rebuild
- `POST /docs/search` → **503**
- MCP `find` with a query returns **rules only** (no `documents` field)
- `GET /docs/graph`, `/docs/chunks/:id`, `/docs/stats` still read the on-disk cache

## Host API

Default base: `http://127.0.0.1:9470`

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/health` | Includes `shelves: { enabled, indexed, chunk_count, … }` |
| POST | `/docs/search` | `{ query, top_k? }` → `{ hits: [...] }` |
| GET | `/docs/chunks/{id}` | Full chunk text |
| GET | `/docs/graph` | Document relationship graph |
| GET | `/docs/stats` | Index metadata |
| POST | `/docs/rebuild` | Force rebuild when enabled |

## MCP

Mount **imprint-mcp** only. Document hits are merged into `find` when shelves is enabled and `query` is set:

```json
{
  "rules": [ ... ],
  "documents": [ { "id", "path", "heading", "score", "snippet" } ]
}
```

With no query, or shelves disabled, the response is a **rules array** only.

`get` returns a document chunk when `id` matches the shelves index; otherwise loads a vault rule.

## Desk UI

Desk proxies `/api/docs/*` to the host. The header shows shelves on/off from `GET /health` → `shelves.enabled`.
