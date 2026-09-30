# imprint memory

This project's long-term memory is **imprint**, not chat history. **Nothing syncs automatically** — you must call imprint tools in the same turn when the user states something durable.

## MCP + shelves (required for full recall)

Mount **imprint MCP** for full shelves (document BM25 + rule↔doc links). CLI `imprint --json` reads/writes the vault; recall capabilities differ as below.

| Capability | MCP (shelves on) | CLI |
| --- | --- | --- |
| Write rules + link docs | `add` / `supersede` + `sources` | same (vault write) |
| Pre-coding recall | `find(scope, query[, query_local][, paths])` → rules + **documents** + **links** | `find --json` → vault rules; with shelves + query → same enriched shape; optional `--paths` |
| Rule provenance | compact `get r-…` → source pointers; `full:true` → excerpts | `get r-…` → vault only |
| Doc inbound refs | `get <chunk-id>` → **referenced_rules** | chunk `get` not supported |
| Rule graph | desk `/` | host `GET /graph` |
| Human review rule↔doc | `imprint desk open` → `/unified` | — |

Mount: `docs/mcp.md`, `docs/examples/cursor-mcp.json`. Write loop: `docs/correction.md`.

- **Transport:** Prefer **imprint MCP** when connected. CLI fallback = vault read/write (see table). Same vault; pick one transport per operation.
- **Users never maintain the vault.** They speak normally; you run `find` / `add` / `reinforce` / `supersede` / `forget` — never ask for imprint commands or rule ids.
- **You maintain the vault, not the chat.** A durable preference stays unwritten until you call a write tool in the same turn.
- **Review and prune is fine.** `show` / `get` (or `imprint desk open`); explain in plain language; then `supersede` / `forget` / `sweep` after they agree.
- Vault: default `~/.imprint/projects/<hash>/vault.db` per repo (`--vault`, `IMPRINT_VAULT`, or `vault:` in yaml). `path` on add/get = `vault.db`.

## Link model (all types)

### Rule ↔ rule (vault, persistent)

| Link | Direction | Write | Read |
| --- | --- | --- | --- |
| `supersedes` | new rule → old rule | `supersede` | `get`, `/graph`, desk `/` |
| `related` | rule → rule | `link` / vault maintenance | same |
| `conflicts_with` | rule → rule | `add --conflicts` / `link` | same |
| `referenced_by` | reverse (who points at me) | automatic | `get r-…` |

### Rule ↔ document (vault + shelves, persistent)

| Link | Direction | Write | Read (MCP or host/desk) |
| --- | --- | --- | --- |
| **`sources`** | rule → doc | **`add` / `supersede` with `[{path, heading?, chunk?}]`** — vault only, **default: no project markdown edits** | `get r-…` → **`resolved_sources`**; MCP `find`+`query` on rules also includes |
| **`referenced_rules`** | doc → rule | **automatic** — reverse of vault `sources` | **`get <chunk-id>`** (MCP) |

Default path: user speaks → MCP **`find(scope, query[, query_local])`** → document hit → same-turn **`add`/`supersede` + `sources`** (persist distilled **`query_local`** when local-language terms differ from `query`). `sources` / `query_local` live in vault.

### `find.links` (session recall, not persisted)

| `kind` | Meaning |
| --- | --- |
| `sources` | vault `sources` already point at a chunk hit in this find |
| `co_search` | rules and documents co-occur in the same query BM25 pass — **assist only, not written back** |

### Human review (desk, not agent recall)

`imprint desk open` → **`/`** rule graph (`supersedes` / `related` / `conflicts_with`) · **`/docs`** shelves · **`/unified`** rules + docs + **`sources`** cross-edges.

## Writes & filters

- `find` scope tags = **AND**; optional BM25 **`query`** and **`query_local`** (dual pass, merge by rule id max score; not embeddings). Optional **`paths`** filters **shelves documents only** (indexed paths from grep; non-index paths are ignored silently).
- **`claim`**: English imperative (for agents to read and follow); user verbatim → **`text`**; local-language search terms → **`query_local`**.
- **`query_local`**: LLM local-language search terms (**not** verbatim); on write, vault **merges gse tokens from CJK evidence** so terms like `30秒` are not dropped. When distilling, keep compound proper terms (e.g. `灰度发布`, not shortened to `灰度` alone).
- **`confidence` on add**: omit → **0.6**; **0.85** when user corrects; **never 0.9 on add** (tier 0.9 via `reinforce` over time).
- **`sources`**: only when a document **excerpt substantively supports** the rule—not topical overlap (same section title without matching content).
- `add` after ADD only: `claim`, `scope`, `text` required; optional **`sources`** / **`query_local`** as above; **`conflicts`** (CSV of existing rule ids) when the new rule is the opposite of another rule that must stay active.
- `link ID --conflicts id,id`: post-hoc edges. When `add` returns advisory **`similar`** (embed sidecar: same topic, opposite polarity), **confirm with the user**, then record opposition via `link` (or keep both without linking); `conflicts_with` makes later `find` penalize the weaker side.
- `reinforce` requires non-empty user reaffirmation evidence; confidence uses diminishing gain `min(0.95, old + (0.95-old)*0.25)`. Find hits never change confidence. `supersede` decays successor confidence by `supersede.inheritance_alpha` of the gap above 0.6 (default 0.20; 0 copies old, 1 drops to baseline) and inherits **`sources`** / **`query_local`** unless overridden; `forget` strips inbound links and keeps a lifecycle event.
- Server-side guards reject likely secrets/personal data, denied source paths, and high-similarity active duplicates.
- `list`: `--scope`, `--query`, `--min-confidence`, `--since`.
- Language scope tags are canonicalized via GitHub Linguist aliases (go-enry) on add/find/list/supersede; unknown tags pass through. Telemetry writes privacy-safe daily JSONL under `<vault>/state/telemetry/` when enabled. `imprint report --days 30` is CLI audit-only.

## Retrieval (maintainers)

- **Tokenization:** vault and shelves share `internal/textseg` (go-ego/gse `CutSearch`, zh+en) plus camel/Pascal/snake/kebab identifier terms. Do not duplicate tokenizers.
- **Storage:** SQLite `vault.db` only; no `migrate-shards` / `imprint-*.md` shard import.
- `resolved_sources` headings: markdown inline to plain text; a miss is `heading_unresolved` (never another section’s excerpt).
- MCP `find`/rule `get` are compact by default. Document hits use `shelves.config.find_top_k` (default 2, one chunk per path), not rule `top_k`; snippets are a query-window of `snippet_runes` (default 80). A query may return one penalized dormant `wake_candidate`; only explicit user confirmation followed by `reinforce` wakes it.

```bash
# CLI — vault read/write from repo root (walk-up imprint.yaml); find/get see CLI column above
imprint --json find --scope go,naming --query PascalCase [--query-local LOCAL_TERMS] [--paths path1,path2]
imprint --json add "CLAIM" --scope tag,tag --text "user's original words" [--query-local TERMS]
imprint --json reinforce ID --evidence "..." [--query-local TERMS]
imprint --json supersede ID --claim "NEW" --scope tag,tag --reason "..." [--query-local TERMS]
imprint --json get ID
imprint --json forget ID
imprint --json list --status active --scope go --min-confidence 0.85
imprint --json show
imprint --json sweep
imprint --json report --days 30
imprint export --format jsonl
```

## Recall model

**k** = `shelves.config.find_top_k` (default **2**). **k caps `find` document hits** (BM25 snippets, one chunk per path) and **triggers** shelves `find(..., paths=…)` when indexed grep `-l` hits exceed **k**. **k does not cap how many files you Read in full**—Read breadth follows narrowing or the **host agent’s default** search→read behavior.

- **Source code** (outside shelves `roots`): **Grep → Read**; imprint `find` does **not** filter source reads.
- **Shelves markdown** (indexed under `roots`): **Grep first** (full `-l` list; grep is not truncated by k) → at most **one** imprint find for docs/rules:
  - **Indexed hits ≤ k:** no `paths` on find. **Documents:** **host agent default** grep/search then read matched files (full-file Read per that environment’s normal rules). **Vault rules:** if policy is in scope, the same turn’s find omits `paths`.
  - **Indexed hits > k:** **one** `find(scope, query, query_local, paths=<indexed grep paths>)`. **Narrowing holds** when `grep ∩ find.documents` is non-empty: **Read every distinct path in that intersection** (full file each; count follows the intersection, not k). Find snippets locate sections; do not treat snippet + full Read of the same file as duplicate primary evidence.
  - **Narrowing does not apply** (no `paths` find, empty intersection, or find unusable): **documents** use **host agent default** grep/search→read on the grep hit list—imprint does **not** impose a Read file cap or imprint-specific pick heuristic. Do **not** chain find → grep → find in one message.
- **Vault / policy only** (no shelves doc search): one find without `paths`.
- **Never** open with find, then grep shelves docs, then find again with `paths` — that is two finds.

Unindexed grep-only paths: follow host agent default read policy; imprint does not assign a numeric Read cap.

## Must do

1. **At most one `find` per user message**. Prepare **scope + query + query_local + paths (when needed)** in one call. **Never** chain finds (`find` → grep → `find`).
2. **Reuse that same find** for write classification (ADD / REINFORCE / SUPERSEDE / IGNORE) — no second find before `add` / `supersede`.
3. When shelves is on: **answer from rule claims first**; documents are supplemental (weak hits filtered). Cite `[r-id]` only when a rule shapes behavior.
4. **Same-turn write:** durable preference → classify from the **single find above** → **write in this turn**. Document hit → pass **`sources`** on `add` / `supersede`; distilled local search terms → **`query_local`** on write tools.
5. **IGNORE** one-off tasks and session-only steps. **ADD / REINFORCE / SUPERSEDE** only for cross-session policy in the user's words.
6. **Implementation:** read/write code on the critical path; grep/read source without waiting on find. **Wide indexed doc grep (> k):** complete grep, then the single find with `paths`; if intersection non-empty, Read all intersected paths; otherwise document reads follow host agent default search→read.
7. Before every write, classify again from existing find results; no duplicates. Add confidence: default 0.6, corrections 0.85, never 0.9.
8. User negates in plain speech → use the **one** find (or `list` if no find yet) then `forget` or `supersede`.
9. User asks what's recorded → `show` or **`imprint desk open`** (`/`, `/docs`, `/unified`).
10. After imprint feature work here → update README, docs, **vault rules** (same turn), **`internal/cli/templates/body.md`** (`imprint init` embed source) and **`.cursor/rules/imprint-memory.mdc`**, and self-test.

## Must not

- **Multiple `find` calls in one turn** to refine scope/query/paths — merge parameters once.
- **Turn-start find** before grep when shelves doc search will need `paths` — grep indexed hits first, then the only find.
- **Pre-find codebase archaeology** whose only goal is tuning find args — parse the message, grep/read code, call find once when policy or wide doc narrowing needs it.
- Pre-coding recall with documents/links — use **MCP** `find`/`get` (see table).
- Assume vault updates from chat. Don't backfill from history unless asked.
- Infer preferences. Don't store secrets. Don't hand-edit `vault.db` (`sweep` only).
- Call it "memory store" — it is **imprint**.
- Ask the user to maintain imprint (commands, ids). Cite `[r-id]` only when shaping code or when they ask what's recorded.
- Prompt templates `internal/cli/templates/body.md` and `internal/cli/templates/imprint-memory.mdc` must not point at other documents (no markdown hyperlinks and no see-this-file design pointers).
