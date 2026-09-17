# Shelves

Workspace documentation indexing runs **inside the host** (`imprint up`, or `imprint host serve` for debug) and in **imprint-mcp** — not as an external plugin.

Shelves does **not** mean “the LLM reads every file in the repo.” It builds a **local index over configured directories** so agents can **`find` vault imprints and document excerpts in one call**, with optional `sources` links. See [imprint ↔ shelves linking](imprint-shelves-linking.md).

[中文](shelves-builtin.zh.md)

---

## Why shelves (agent perspective)

An LLM does **not** load all project markdown each turn. Ad-hoc grep/read lacks unified ranking, paragraph excerpts, one-shot recall with the vault, and durable links.

| | LLM greps / reads files | shelves |
| --- | --- | --- |
| **With imprint** | Multiple tool calls | One `find` → rules + documents + links |
| **Shape** | Whole files or raw lines | **Chunks** with path, heading, lines, snippet |
| **Ranking** | None unified | BM25 on the same query as vault rules |
| **Durable links** | None | vault `sources` (rule→doc); optional `[[r-…]]` in docs |
| **Cost** | More tokens | Local SQLite index, no embedding API |

Shelves uses **BM25**, not vectors. The win is **integration, structure, locality, and workflow** — not “always more accurate than a human search.”

```mermaid
flowchart LR
  subgraph recall [Before coding · one find]
    F["find(scope, query)"]
    F --> R[rules + resolved_sources]
    F --> D[documents + snippet]
    F --> L[links runtime]
  end

  subgraph persist [On correction · vault only]
    A[add / supersede] --> S[sources path/heading]
    S --> V[(.imprint/memory/)]
  end

  subgraph reverse [get chunk · no doc edits]
    G["get chunk"] --> RR[referenced_rules]
    G --> CR[cited_rules optional]
  end

  V --> RR
```

---

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
      - .cursor/rules
    # stateDir: .imprint/.shelves/.cache   # optional
```

| Field | Meaning |
| --- | --- |
| `enabled` | When `true`, scan `roots` and serve search. When `false`, stop indexing; cache stays readable. |
| `config.roots` | **Directories to index** (`.md`, `.mdc`, `.txt`), relative to repo root. Defines what shelves **searches**, not “everything the LLM can open.” |
| `config.stateDir` | SQLite cache. Default: `.imprint/.shelves/.cache`. |

### `roots` is not optional noise

- Files **outside** `roots`: not indexed; **`find` / `/docs/search` never return them** (agents can still read files directly, but not via shelves recall).
- Files **inside** `roots`: indexed; **`find` with query** can return them alongside vault hits.
- **Why configure**: scope the corpus (less noise, faster rebuild), declare which docs agents should recall before coding.

After changing `roots` or `enabled`, run `imprint up` (or restart `host serve`).

---

## Linking vault imprints without polluting docs

| Direction | Persisted? | Stored in | Edit project markdown? |
| --- | --- | --- | --- |
| imprint → doc | **Yes** | vault `sources` | **No** |
| doc → imprint (reverse) | **Yes** | **Same** — `get chunk` scans vault `sources` → `referenced_rules` | **No** |
| Explicit @ in doc body | Optional | `[[r-…]]` → `cited_rules` | Yes (**optional**) |
| Same-turn `find` | **No** | Response `links` | **No** |

**Default durable bidirectional link: write vault `sources` once.**

- ADD with `sources: [{ path, heading? }]`
- `get r-…` → `resolved_sources`
- `get <chunk>` → `referenced_rules` (reverse from vault, **no doc edits**)

---

## When disabled

`shelves.enabled: false`:

- No scan or rebuild
- `POST /docs/search` → **503**
- MCP `find` with query → **rules only** (no `documents` / `links`)
- `GET /docs/graph`, `/docs/chunks/:id`, `/docs/stats` still read cache

---

## Host API

Default: `http://127.0.0.1:9470`

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/health` | Includes `shelves: { enabled, indexed, chunk_count, … }` |
| GET | `/find` | With query + shelves on → rules + documents + links |
| POST | `/docs/search` | `{ query, top_k? }` → `{ hits: [...] }` |
| GET | `/docs/chunks/{id}` | Full chunk + `referenced_rules` + optional `cited_rules` |
| GET | `/docs/graph` | Document structure graph |
| GET | `/docs/stats` | Index metadata |
| POST | `/docs/rebuild` | Force rebuild when enabled |

---

## MCP

Mount **imprint-mcp** only. With shelves enabled and **`find` + query**:

```json
{
  "rules": [{ "id", "resolved_sources": [...], ... }],
  "documents": [{ "id", "path", "heading", "score", "snippet" }],
  "links": [{ "rule_id", "chunk_id", "kind", "score" }]
}
```

Without query, or shelves off: **rules array** only (enriched with `resolved_sources` when rules have `sources`).

`get`: `r-…` → vault record + `resolved_sources`; chunk id → chunk + **`referenced_rules`** (+ `cited_rules` if markdown cites rules).

---

## Desk UI

Desk proxies `/api/docs/*` to the host. Header shows shelves on/off from `GET /health`. **Agents use MCP `find` / `get`**, not desk, for recall.

---

## See also

| Doc | Topic |
| --- | --- |
| [imprint-shelves-linking.md](imprint-shelves-linking.md) | Link model, storage, agent workflow |
| [mcp.md](mcp.md) | MCP tools and mount |
| [correction.md](correction.md) | When to pass `sources` on add |
