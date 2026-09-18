# imprint memory

This project's long-term memory is **imprint**, not chat history. **Nothing syncs automatically** — you must call imprint tools in the same turn when the user states something durable.

## MCP + shelves（必读）

**完整 shelves（文档 BM25 + 规则↔文档关联）请挂载 imprint MCP。** CLI `imprint --json` 可读写 vault；召回能力见下表。

| 能力 | MCP（shelves 开） | CLI |
| --- | --- | --- |
| 写规则 + 挂文档 | `add` / `supersede` + `sources` | 同左（写 vault） |
| 写代码前召回 | `find(scope, query)` → rules + **documents** + **links** | `find` → vault rules only |
| 查规则依据 | `get r-…` → **resolved_sources** | `get r-…` → vault only |
| 查文档被谁引用 | `get <chunk-id>` → **referenced_rules** + **cited_rules** | 不支持 chunk `get` |
| 规则关系图 | desk `/` | host `GET /graph` |
| 人审 rule↔doc | 建议 `imprint desk open` → `/unified` | — |

Mount: `docs/mcp.md`, `docs/examples/cursor-mcp.json`. Write loop: `docs/correction.md` / `docs/correction.zh.md`.

- **Transport:** Prefer **imprint MCP** when connected. CLI fallback = vault read/write (see table). Same vault; pick one transport per operation.
- **Users never maintain the vault.** They speak normally; you run `find` / `add` / `reinforce` / `supersede` / `forget` — never ask for imprint commands or rule ids.
- **You maintain the vault, not the chat.** A durable preference stays unwritten until you call a write tool in the same turn.
- **Review and prune is fine.** `show` / `get` (or `imprint desk open`); explain in plain language; then `supersede` / `forget` / `sweep` after they agree.
- Vault: `.imprint/memory/vault.db` (`IMPRINT_VAULT`, `--vault`, `--global`). `path` on add/get = `vault.db`.

## 关联方式（全部）

### 规则 ↔ 规则（vault，持久）

| 关联 | 方向 | 写入 | 读取 |
| --- | --- | --- | --- |
| `supersedes` | 新 rule → 旧 rule | `supersede` | `get`, `/graph`, desk `/` |
| `related` | rule → rule | `add` / vault 维护 | 同上 |
| `conflicts_with` | rule → rule | `add` | 同上 |
| `referenced_by` | 反向（谁指向我） | 自动 | `get r-…` |

### 规则 ↔ 文档（vault + shelves，持久）

| 关联 | 方向 | 写入 | 读取（需 MCP 或 host/desk） |
| --- | --- | --- | --- |
| **`sources`** | rule → doc | **`add` / `supersede` 传 `[{path, heading?, chunk?}]`** — 只写 vault，**默认不改项目 markdown** | `get r-…` → **`resolved_sources`**；MCP `find`+`query` 规则命中也带 |
| **`referenced_rules`** | doc → rule | **自动** — vault `sources` 反查 | **`get <chunk-id>`**（MCP） |
| **`cited_rules`** | doc → rule | 维护者在正文写 `[[r-…]]` / `[imprint:r-…]`；shelves rebuild | **`get <chunk-id>`**（MCP）；desk unified **`cited_by`** 边 |

默认路径：用户说话 → MCP **`find(scope, query)`** → document 命中 → 同轮 **`add`/`supersede` + `sources`**。`sources` 写在 vault；markdown 正文可选 `[[r-…]]`。

### `find` 的 `links`（当次召回，不持久）

| `kind` | 含义 |
| --- | --- |
| `sources` | vault 里已有 `sources` 指向本次命中的 chunk |
| `cited_by` | chunk 正文 `[[r-…]]` 指向规则 |
| `co_search` | 同一次 query 下 rules 与 documents BM25 共现 — **仅辅助判断，不写回 vault** |

### 人审（desk，非 agent 召回）

`imprint desk open` → **`/`** 规则图（`supersedes` / `related` / `conflicts_with`）· **`/docs`** shelves · **`/unified`** 规则+文档+**`sources` / `cited_by`** 跨边。

## Writes & filters

- `find` scope tags = **AND**; optional BM25 `query` (not embeddings).
- `add` after ADD only: `claim`, `scope`, `text` required; optional **`sources`** when user points at docs or MCP find hits a matching document.
- `reinforce` +0.1 (cap 0.95); `supersede` inherits `sources` unless overridden; `forget` strips inbound links.
- `list`: `--scope`, `--query`, `--min-confidence`, `--since`.

```bash
# CLI — vault read/write; find/get 见上表 CLI 列
imprint --json --vault ./.imprint/memory find --scope go,naming --query PascalCase
imprint --json --vault ./.imprint/memory add "CLAIM" --scope tag,tag --text "user's original words"
imprint --json --vault ./.imprint/memory reinforce ID --evidence "..."
imprint --json --vault ./.imprint/memory supersede ID --claim "NEW" --scope tag,tag --reason "..."
imprint --json --vault ./.imprint/memory get ID
imprint --json --vault ./.imprint/memory forget ID
imprint --json --vault ./.imprint/memory list --status active --scope go --min-confidence 0.85
imprint --json --vault ./.imprint/memory show
imprint --json --vault ./.imprint/memory sweep
```

## Must do

1. Before coding or style answers → **MCP `find`** with **narrow scope** + **query** (shelves on). Use **`resolved_sources`**, **`documents`**, **`links`**. Cite `[r-id]` when a rule shapes behavior; cite doc paths when excerpts apply. CLI fallback = vault rules only.
2. **Same-turn write:** durable preference → MCP `find` → classify ADD / REINFORCE / SUPERSEDE / IGNORE → **write in this turn**. Document hit → pass **`sources`** on `add` / `supersede`.
3. **IGNORE** one-off tasks and session-only steps. **ADD / REINFORCE / SUPERSEDE** only for cross-session policy in the user's words.
4. Analyse the requirement before modifying code.
5. Before every write, classify again; no duplicates. Confidence: default 0.6; corrections 0.85; "always" 0.9.
6. User negates in plain speech → `find` then `forget` or `supersede`.
7. User asks what's recorded → `show` or **`imprint desk open`** (`/`, `/docs`, `/unified`).
8. After imprint feature work here → update README flowchart and docs; self-test.

## Must not

- Pre-coding recall with documents/links — use **MCP** `find`/`get` (see table).
- Assume vault updates from chat. Don't backfill from history unless asked.
- Infer preferences. Don't store secrets. Don't hand-edit `vault.db` (`sweep` only).
- Call it "memory store" — it is **imprint**.
- Ask the user to maintain imprint (commands, ids). Cite `[r-id]` only when shaping code or when they ask what's recorded.
