# imprint memory

This project's long-term memory is **imprint**, not chat history. **Nothing syncs automatically** — you must call imprint tools in the same turn when the user states something durable.

## MCP + shelves (required for full recall)

Mount **imprint MCP** for full shelves (document BM25 + rule↔doc links). CLI `imprint --json` reads/writes the vault; recall capabilities differ as below.

| Capability | MCP (shelves on) | CLI |
| --- | --- | --- |
| Write rules + link docs | `add` / `supersede` + `sources` | same (vault write) |
| Pre-coding recall | `find(scope, query[, query_local])` → rules + **documents** + **links** | `find` → vault rules only (`--query-local` dual merge) |
| Rule provenance | compact `get r-…` → source pointers; `full:true` → excerpts | `get r-…` → vault only |
| Doc inbound refs | `get <chunk-id>` → **referenced_rules** + **cited_rules** | chunk `get` not supported |
| Rule graph | desk `/` | host `GET /graph` |
| Human review rule↔doc | `imprint desk open` → `/unified` | — |

Mount: `docs/mcp.md`, `docs/examples/cursor-mcp.json`. Write loop: `docs/correction.md`.

- **Transport:** Prefer **imprint MCP** when connected. CLI fallback = vault read/write (see table). Same vault; pick one transport per operation.
- **Users never maintain the vault.** They speak normally; you run `find` / `add` / `reinforce` / `supersede` / `forget` — never ask for imprint commands or rule ids.
- **You maintain the vault, not the chat.** A durable preference stays unwritten until you call a write tool in the same turn.
- **Review and prune is fine.** `show` / `get` (or `imprint desk open`); explain in plain language; then `supersede` / `forget` / `sweep` after they agree.
- Vault: `.imprint/vault.db` (`IMPRINT_VAULT`, `--vault`, `--global`). `path` on add/get = `vault.db`.

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
- `reinforce` requires non-empty user reaffirmation evidence; confidence uses diminishing gain `min(0.95, old + (0.95-old)*0.25)`. Find hits never change confidence. `supersede` decays successor confidence by `supersede.inheritance_alpha` of the gap above 0.6 (default 0.20; 0 copies old, 1 drops to baseline) and inherits **`sources`** / **`query_local`** unless overridden; `forget` strips inbound links and keeps a lifecycle event.
- Server-side guards reject likely secrets/personal data, denied source paths, and high-similarity active duplicates.
- `list`: `--scope`, `--query`, `--min-confidence`, `--since`.
- Language scope tags are canonicalized via GitHub Linguist aliases (go-enry) on add/find/list/supersede; unknown tags pass through. Telemetry writes privacy-safe daily JSONL under `.imprint/state/telemetry/` when enabled. `imprint report --days 30` is CLI audit-only.

## Retrieval (maintainers)

- **Tokenization:** vault and shelves share `internal/textseg` (go-ego/gse `CutSearch`, zh+en) plus camel/Pascal/snake/kebab identifier terms. Do not duplicate tokenizers.
- **Storage:** SQLite `vault.db` only; no `migrate-shards` / `imprint-*.md` shard import.
- `resolved_sources` headings: markdown inline to plain text; a miss is `heading_unresolved` (never another section’s excerpt).
- MCP `find`/rule `get` are compact by default. A query may return one penalized dormant `wake_candidate`; only explicit user confirmation followed by `reinforce` wakes it.

```bash
# CLI — vault read/write; find/get see CLI column above
imprint --json --vault ./.imprint find --scope go,naming --query PascalCase [--query-local LOCAL_TERMS]
imprint --json --vault ./.imprint add "CLAIM" --scope tag,tag --text "user's original words" [--query-local TERMS]
imprint --json --vault ./.imprint reinforce ID --evidence "..." [--query-local TERMS]
imprint --json --vault ./.imprint supersede ID --claim "NEW" --scope tag,tag --reason "..." [--query-local TERMS]
imprint --json --vault ./.imprint get ID
imprint --json --vault ./.imprint forget ID
imprint --json --vault ./.imprint list --status active --scope go --min-confidence 0.85
imprint --json --vault ./.imprint show
imprint --json --vault ./.imprint sweep
imprint --json --vault ./.imprint report --days 30
imprint --vault ./.imprint export --format jsonl
```

## Recall model (async, reference-only)

- **Imprint is核对和参考**, not the primary research path. **Do not block** coding, grep, or file reads waiting on find results.
- **Critical path:** user task → read/write code or docs → answer. **Side path:** one find to check stored policy; adjust only if a rule claim conflicts.
- Find hits **confirm or constrain** — they do not replace reading `server.go`, tests, or project docs for implementation.

## Must do

1. **At most one `find` per user message** (per agent turn). Before calling, prepare **all** parameters in one pass: `scope` + `query` + `query_local` (when CJK/local terms differ). **Never** chain finds (`find` → grep → `find` → `find`); merge keywords up front (e.g. `query`: `host debug route timeout`, `query_local`: `调试 慢接口 debug 路由`).
2. **Reuse that same find** for write classification (ADD / REINFORCE / SUPERSEDE / IGNORE) — do **not** run a second find before `add` / `supersede`.
3. When shelves is on: **answer from rule claims first**; documents are supplemental (weak hits filtered). Cite `[r-id]` only when a rule shapes behavior. CLI fallback = vault rules only.
4. **Same-turn write:** durable preference → classify from the **single find above** → **write in this turn**. Document hit → pass **`sources`** on `add` / `supersede`; distilled local search terms → **`query_local`** on `add` / `supersede` / `reinforce`.
5. **IGNORE** one-off tasks and session-only steps. **ADD / REINFORCE / SUPERSEDE** only for cross-session policy in the user's words.
6. Analyse the requirement before modifying code. **Implementation requests:** code/docs on the critical path; imprint find is optional核对 unless policy is unclear.
7. Before every write, classify again from existing find results; no duplicates. Add confidence: default 0.6, corrections 0.85, never 0.9.
8. User negates in plain speech → use the **one** find (or `list` if no find yet) then `forget` or `supersede`.
9. User asks what's recorded → `show` or **`imprint desk open`** (`/`, `/docs`, `/unified`).
10. After imprint feature work here → update README, docs, **vault rules** (same turn), **`internal/cli/templates/body.md`** (`imprint init` embed source) and **`.cursor/rules/imprint-memory.mdc`**, and self-test.

## Must not

- **Multiple `find` calls in one turn** to “refine” scope/query — widen parameters once instead.
- **Pre-find codebase archaeology** (many greps/files) whose only goal is tuning find args — parse the user message, call find once, then read code if implementing.
- Pre-coding recall with documents/links — use **MCP** `find`/`get` (see table).
- Assume vault updates from chat. Don't backfill from history unless asked.
- Infer preferences. Don't store secrets. Don't hand-edit `vault.db` (`sweep` only).
- Call it "memory store" — it is **imprint**.
- Ask the user to maintain imprint (commands, ids). Cite `[r-id]` only when shaping code or when they ask what's recorded.
- Prompt templates `internal/cli/templates/body.md` and `internal/cli/templates/imprint-memory.mdc` must not point at other documents (no markdown hyperlinks and no see-this-file design pointers).
