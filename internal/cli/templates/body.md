# imprint memory

This project's long-term memory is **imprint**, not chat history.

- **Transport:** Prefer **imprint MCP** tools when the `imprint` server is connected in the host. If MCP is not mounted or a tool call fails, fall back to `imprint --json` in the shell. Same vault, same JSON shapes — do not use both for one operation.
- **Users never maintain the vault.** They speak normally while coding. You run `find` / `add` / `reinforce` / `supersede` / `forget` — never ask them to say imprint commands, rule ids, or "delete rule X". Confirm in plain language only.
- **Review and prune is fine.** When they ask to review what is recorded or trim obsolete branches, use `show` / `viz` / `get` (and the dashboard graph), explain in plain language, then `supersede` / `forget` / `sweep` what they reject — they do not pick ids or run CLI themselves.
- Vault: `./memory/` (override with `IMPRINT_VAULT` or `--vault` / `--global`)
- MCP mount: see `docs/mcp.md` and `docs/examples/cursor-mcp.json`. Correction loop + coding examples: `docs/correction.md` / `docs/correction.zh.md`.
- `path` on add/get is the shard file (`imprint-NNNN.md`), not a per-rule filename. `memory/notes/` is generated and read-only.
- Effects: `find` tags are **AND**, then `--query` is optional BM25 (not embeddings); `add` only after ADD (`claim`, `--scope`, `--text` required); `reinforce` +0.1 (cap 0.95); `supersede` archives the old rule and writes a new one; `get` includes `evidence_log` and `referenced_by`; `forget` strips inbound links; `list` accepts `--scope`, `--query`, `--min-confidence`, `--since`.

```bash
imprint --json --vault ./memory find --scope go,naming
imprint --json --vault ./memory find --scope go --query PascalCase
imprint --json --vault ./memory add "CLAIM" --scope tag,tag --text "user's original words"
imprint --json --vault ./memory reinforce ID --evidence "..."
imprint --json --vault ./memory supersede ID --claim "NEW" --scope tag,tag --reason "..."
imprint --json --vault ./memory get ID
imprint --json --vault ./memory forget ID
imprint --json --vault ./memory list --status active --scope go --min-confidence 0.85
imprint --json --vault ./memory show
imprint --json --vault ./memory sweep
imprint --json --vault ./memory viz
imprint --json --vault ./memory viz --format notes
```

## Must do

1. Before coding or answering style/convention questions, `find` (MCP tool or CLI) with a **narrow scope** (e.g. `go,naming`). Cite `[r-id]` when a hit shapes behavior.
2. Before modifying anything, analyze the requirement until it is complete.
3. Before every write, classify: ADD / REINFORCE / SUPERSEDE / IGNORE. Never add a duplicate.
4. Record only what the user **said**. Starting confidence 0.6; corrections 0.85; "from now on always" 0.9.
5. User negates in normal speech ("don't record that", "we follow STYLE.md now") → you `find` and `forget` or `supersede`; user does not maintain memory.
6. User asks what is recorded / to see the picture → `show`; if more than ~20 items, `viz` (HTML by default, or `--format mermaid` / `--format notes`) and tell them the path.
7. After development on this project, update README (including the flowchart) and all docs; self-test thoroughly.

## Must not

- Infer preferences. Don't store secrets. Don't archive by hand (`imprint sweep` only). Don't hand-edit `memory/notes/`.
- Call the system "memory store" in user-facing text — it is **imprint**.
- Ask the user to maintain imprint (commands, ids, "forget r-…"). Cite `[r-id]` only when shaping code or when they ask what is recorded.
