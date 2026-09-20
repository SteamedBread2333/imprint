# imprint memory

This project's long-term memory is **imprint**, not chat history. **Nothing syncs automatically** — you must call imprint tools in the same turn when the user states something durable.

## MCP + shelves (required for full recall)

Mount **imprint MCP** for full shelves (document BM25 + rule↔doc links). CLI `imprint --json` reads/writes the vault; recall capabilities differ as below.

| Capability | MCP (shelves on) | CLI |
| --- | --- | --- |
| Write rules + link docs | `add` / `supersede` + `sources` | same (vault write) |
| Pre-coding recall | `find(scope, query[, query_local])` → rules + **documents** + **links** | `find` → vault rules only (`--query-local` dual merge) |
| Rule provenance | `get r-…` → **resolved_sources** | `get r-…` → vault only |
| Doc inbound refs | `get <chunk-id>` → **referenced_rules** + **cited_rules** | chunk `get` not supported |
| Rule graph | desk `/` | host `GET /graph` |
| Human review rule↔doc | `imprint desk open` → `/unified` | — |

Mount: `docs/mcp.md`, `docs/examples/cursor-mcp.json`. Write loop: `docs/correction.md`.

- **Transport:** Prefer **imprint MCP** when connected. CLI fallback = vault read/write (see table). Same vault; pick one transport per operation.
- **Users never maintain the vault.** They speak normally; you run `find` / `add` / `reinforce` / `supersede` / `forget` — never ask for imprint commands or rule ids.
- **You maintain the vault, not the chat.** A durable preference stays unwritten until you call a write tool in the same turn.
- **Review and prune is fine.** `show` / `get` (or `imprint desk open`); explain in plain language; then `supersede` / `forget` / `sweep` after they agree.
- Vault: `.imprint/memory/vault.db` (`IMPRINT_VAULT`, `--vault`, `--global`). `path` on add/get = `vault.db`.

## Link model (all types)

### Rule ↔ rule (vault, persistent)

| Link | Direction | Write | Read |
| --- | --- | --- | --- |
| `supersedes` | new rule → old rule | `supersede` | `get`, `/graph`, desk `/` |
| `related` | rule → rule | `add` / vault maintenance | same |
| `conflicts_with` | rule → rule | `add` | same |
| `referenced_by` | reverse (who points at me) | automatic | `get r-…` |

### Rule ↔ document (vault + shelves, persistent)

| Link | Direction | Write | Read (MCP or host/desk) |
| --- | --- | --- | --- |
| **`sources`** | rule → doc | **`add` / `supersede` with `[{path, heading?, chunk?}]`** — vault only, **default: no project markdown edits** | `get r-…` → **`resolved_sources`**; MCP `find`+`query` on rules also includes |
| **`referenced_rules`** | doc → rule | **automatic** — reverse of vault `sources` | **`get <chunk-id>`** (MCP) |
| **`cited_rules`** | doc → rule | maintainer writes `[[r-…]]` / `[imprint:r-…]` in markdown; shelves rebuild | **`get <chunk-id>`** (MCP); desk unified **`cited_by`** edges |

Default path: user speaks → MCP **`find(scope, query[, query_local])`** → document hit → same-turn **`add`/`supersede` + `sources`** (persist distilled **`query_local`** when local-language terms differ from `query`). `sources` / `query_local` live in vault; optional `[[r-…]]` in markdown body.

### `find.links` (session recall, not persisted)

| `kind` | Meaning |
| --- | --- |
| `sources` | vault `sources` already point at a chunk hit in this find |
| `cited_by` | chunk body `[[r-…]]` points at a rule |
| `co_search` | rules and documents co-occur in the same query BM25 pass — **assist only, not written back** |

### Human review (desk, not agent recall)

`imprint desk open` → **`/`** rule graph (`supersedes` / `related` / `conflicts_with`) · **`/docs`** shelves · **`/unified`** rules + docs + **`sources` / `cited_by`** cross-edges.

## Writes & filters

- `find` scope tags = **AND**; optional BM25 **`query`** and **`query_local`** (dual pass, merge by rule id max score; not embeddings).
- **`claim`**: English imperative (for agents to read and follow); user verbatim → **`text`**; local-language search terms → **`query_local`**.
- **`query_local`**: LLM local-language search terms (**not** verbatim); on write, vault **merges gse tokens from CJK evidence** so terms like `30秒` are not dropped.
- **`confidence` on add**: omit → **0.6**; **0.85** when user corrects; **never 0.9 on add** (tier 0.9 via `reinforce` over time).
- **`sources`**: only when a document **excerpt substantively supports** the rule—not topical overlap (same section title without matching content).
- `add` after ADD only: `claim`, `scope`, `text` required; optional **`sources`** / **`query_local`** as above.
- `reinforce` +0.1 (cap 0.95); optional **`query_local`** update; `supersede` inherits **`sources`** and **`query_local`** unless overridden; `forget` strips inbound links.
- `list`: `--scope`, `--query`, `--min-confidence`, `--since`.

## Retrieval (maintainers)

- **Tokenization:** vault and shelves share `internal/textseg` ([go-ego/gse](https://github.com/go-ego/gse) `CutSearch`, zh+en); do not reintroduce CJK bigram or duplicate tokenizers. Optional `imprint.yaml` → `glossary.path` for domain terms.
- **Storage:** SQLite `vault.db` only; no `migrate-shards` / `imprint-*.md` shard import.
- Design detail: `docs/storage-retrieval.md` §5.2 (`query_local` dual find).

```bash
# CLI — vault read/write; find/get see CLI column above
imprint --json --vault ./.imprint/memory find --scope go,naming --query PascalCase [--query-local LOCAL_TERMS]
imprint --json --vault ./.imprint/memory add "CLAIM" --scope tag,tag --text "user's original words" [--query-local TERMS]
imprint --json --vault ./.imprint/memory reinforce ID --evidence "..." [--query-local TERMS]
imprint --json --vault ./.imprint/memory supersede ID --claim "NEW" --scope tag,tag --reason "..." [--query-local TERMS]
imprint --json --vault ./.imprint/memory get ID
imprint --json --vault ./.imprint/memory forget ID
imprint --json --vault ./.imprint/memory list --status active --scope go --min-confidence 0.85
imprint --json --vault ./.imprint/memory show
imprint --json --vault ./.imprint/memory sweep
```

## Must do

1. Before coding or style answers → **MCP `find`** with **narrow scope** + **`query`** (shelves on); when local-language terms differ from `query`, also pass **`query_local`**. **Answer from rule claims first**; documents are supplemental (weak hits filtered). Use **`resolved_sources`**, **`documents`**, **`links`**. Cite `[r-id]` when a rule shapes behavior. CLI fallback = vault rules only.
2. **Same-turn write:** durable preference → MCP `find` → classify ADD / REINFORCE / SUPERSEDE / IGNORE → **write in this turn**. Document hit → pass **`sources`** on `add` / `supersede`; distilled local search terms → **`query_local`** on `add` / `supersede` / `reinforce`.
3. **IGNORE** one-off tasks and session-only steps. **ADD / REINFORCE / SUPERSEDE** only for cross-session policy in the user's words.
4. Analyse the requirement before modifying code.
5. Before every write, classify again; no duplicates. Confidence: default 0.6; corrections 0.85; "always" 0.9.
6. User negates in plain speech → `find` then `forget` or `supersede`.
7. User asks what's recorded → `show` or **`imprint desk open`** (`/`, `/docs`, `/unified`).
8. After imprint feature work here → update README, docs, **vault rules** (same turn), **`internal/cli/templates/body.md`** (`imprint init` embed source) and **`.cursor/rules/imprint-memory.mdc`**, and self-test.

## Must not

- Pre-coding recall with documents/links — use **MCP** `find`/`get` (see table).
- Assume vault updates from chat. Don't backfill from history unless asked.
- Infer preferences. Don't store secrets. Don't hand-edit `vault.db` (`sweep` only).
- Call it "memory store" — it is **imprint**.
- Ask the user to maintain imprint (commands, ids). Cite `[r-id]` only when shaping code or when they ask what's recorded.
