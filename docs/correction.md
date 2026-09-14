# Memory correction

In imprint, **correction** means: when the user fixes the agent, preferences land in the vault and **recall on the next coding task** steers behaviour back — not model training, not embeddings, but **markdown rules with an explicit lifecycle**.

Before every write the agent must **`find` → classify → pick an operation**. imprint stores honestly; it does not decide whether a write should happen.

## Who maintains the vault (no extra burden)

**Users** keep working and talking normally — corrections and confirmations in plain language. They do **not** need to know `imprint`, `forget`, rule ids, or maintenance phrases.

**Agents** run `find`, classify, and call `add` / `reinforce` / `supersede` / `forget`. When the user says “don’t record that” or “we follow STYLE.md now”, the agent queries the vault and acts — never ask the user to repeat a command or id.

**Humans (optional)** audit via `memory/dashboard.html` or `show` / `viz` when curious — that is read-only review, not a daily chore.

**Review + prune (allowed):** the user may say “review naming memories and cut obsolete branches”. The agent uses `show` / `viz` / `get`, explains in plain language, then `supersede` / `forget` / `sweep` after they agree. Forbidden is making them supply ids or run CLI — not tidying the vault.

CLI / MCP snippets in this doc are the **agent execution surface**, not a user manual to memorise.

## Correction loop

```mermaid
sequenceDiagram
  participant U as User
  participant A as Agent
  participant I as imprint

  U->>A: preference / correction / "don't do that"
  A->>I: find (narrow scope, optional query)
  I-->>A: ranked existing rules

  alt no match
    A->>A: ADD
    A->>I: add
  else user repeats the same preference
    A->>A: REINFORCE
    A->>I: reinforce
  else wording or scope changed / old rule wrong
    A->>A: SUPERSEDE
    A->>I: supersede
  else user says don't record / one-off
    A->>A: IGNORE or forget
  end

  Note over A,I: find again before coding; cite [r-id]
```

| Step | Who | What |
| --- | --- | --- |
| **Recall** | Agent | `find --scope tag,tag` (tags are **AND**), optional `--query` BM25 |
| **Classify** | Agent | ADD / REINFORCE / SUPERSEDE / IGNORE |
| **Write** | imprint | `add` / `reinforce` / `supersede` / `forget` |
| **Audit** | Human | `get`, `show`, `viz`; old rules in `archive/` |

Prefer **MCP tools**; fall back to **`imprint --json`** ([mcp.md](mcp.md)).

## Four write classifications

| Class | When | Command | Vault effect |
| --- | --- | --- | --- |
| **ADD** | `find` empty; **new** user preference | `add` | New `active` rule, default confidence **0.6** |
| **REINFORCE** | Rule exists; user **confirms again** | `reinforce` | confidence **+0.1** (cap **0.95**), `reinforcement_count++`, may wake `dormant` |
| **SUPERSEDE** | Preference **changed**, scope **widened/narrowed**, old claim **invalid** | `supersede` | Old → `superseded` in `archive/`; new `active` with `supersedes: [old_id]` |
| **IGNORE** | One-off task, chit-chat, agent-**inferred** preference | (no write) | — |

Also:

| Op | When |
| --- | --- |
| **forget** | User negates in **plain speech** (“don’t record that”); agent `find`s then `forget`s — user never mentions imprint |
| **sweep** | Periodic decay of untouched rules (default 90d −0.05; below 0.3 → `dormant`) |

### Confidence (agent convention)

On `add`, set `--confidence` / MCP `confidence`:

| User tone | Suggested confidence |
| --- | --- |
| First mention, casual | **0.6** (default) |
| Explicit correction ("no, use …") | **0.85** |
| "From now on always …" | **0.9** |

`reinforce` stacks on the current value; a changed policy should be **supersede** with a high-confidence new claim, not a duplicate `add`.

### Record only what the user said

- **`--text` / `text`**: user's words (evidence)
- **`claim`**: imperative statement for the agent
- No secrets; no inferred preferences

---

## Coding scenarios

Vault **`./memory`**. MCP and CLI are equivalent; one of each shown.

### 1. New preference — ADD

**User:** Go exported identifiers must be PascalCase, not snake_case exports.

**Agent**

1. `find --scope go,naming --query PascalCase` → no hits  
2. **ADD**

```bash
imprint --json --vault ./memory add \
  "Go exported identifiers must use PascalCase" \
  --scope go,naming \
  --text "Go exported identifiers must be PascalCase, not snake_case exports"
```

MCP `add`: same `claim`, `scope`, `text`; omit `confidence` (0.6).

---

### 2. User confirms again — REINFORCE

**User:** Yes, exports stay PascalCase — what I said before still holds.

**Agent**

1. `find --scope go,naming` → `r-2026-09-14-001`  
2. **REINFORCE**

```bash
imprint --json --vault ./memory reinforce r-2026-09-14-001 \
  --evidence "user confirmed PascalCase for exports again"
```

---

### 3. User narrows a wrong rule — SUPERSEDE

**User:** Not quite — internal packages can use unexported camelCase; only **cross-package exports** need PascalCase.

**Agent**

1. `find --scope go,naming` → old rule too broad  
2. **SUPERSEDE**

```bash
imprint --json --vault ./memory supersede r-2026-09-14-001 \
  --claim "Go identifiers exported across packages must use PascalCase; internal unexported names use camelCase" \
  --scope go,naming \
  --reason "narrowed to cross-package exports only" \
  --text "internal packages can use unexported camelCase; only cross-package exports need PascalCase"
```

Old id → `superseded` + `archive/`; new id is `active` and links back.

---

### 4. Widen scope — SUPERSEDE

**User:** Same for frontend TS — exported components/functions follow PascalCase, aligned with Go.

```bash
imprint --json --vault ./memory supersede r-2026-09-14-002 \
  --claim "Exported Go and TypeScript symbols follow PascalCase (TS aligned with Go exports)" \
  --scope go,typescript,naming \
  --reason "extended naming rule to frontend TS" \
  --text "Same for frontend TS — exported components/functions follow PascalCase, aligned with Go"
```

---

### 5. Agent almost infers — IGNORE

**User:** Refactor this handler and split it into two files.

Task instruction, not a long-term preference → **IGNORE**. No `add`.

---

### 6. Preference voided — user talks, agent cleans up

**User:** Use `STYLE.md` for naming from now on — the PascalCase export rule we used before is gone.

**Agent**

1. `find --scope go,naming` → old rule `r-2026-09-14-003`  
2. **SUPERSEDE** (policy moved to `STYLE.md`, keep audit trail) or **forget** (user rejects imprint-stored naming prefs entirely)  
3. User never says “imprint”; commands below are agent-only

**Prefer supersede:**

```bash
imprint --json --vault ./memory supersede r-2026-09-14-003 \
  --claim "Follow STYLE.md for naming; imprint does not override the style guide" \
  --scope go,naming,docs \
  --reason "naming policy moved to STYLE.md" \
  --text "Use STYLE.md for naming from now on — the PascalCase export rule we used before is gone"
```

**If the user rejects imprint-held naming prefs** (“stop recording these”) → `forget r-2026-09-14-003`.

Reply in normal language — do not ask the user to confirm a rule id or pick a command.

---

### 7. Recall before coding — find

**User:** Add an exported method on `UserService`.

**Agent (before editing)**

```bash
imprint --json --vault ./memory find --scope go,naming --query export
```

Hit `[r-2026-09-14-002]` → name the method `GetProfile`, not `get_profile`; cite `[r-…]` when it shapes the change.

---

## Passive fade vs active correction

| Mechanism | Trigger | Use when |
| --- | --- | --- |
| **supersede / forget** | User speaks in plain language | Rule **wrong** or **void** (agent acts; user does not maintain the vault) |
| **reinforce** | User repeats | Rule still **valid**, strengthen |
| **sweep** | Ops / cron | Stale preferences **fade** (default 90d untouched −0.05; &lt; 0.3 → dormant) |

`sweep` does not erase history; `dormant` rules live under `archive/` and `reinforce` can wake them.

---

## Quick reference

```bash
imprint --json --vault ./memory find --scope go,error-handling --query wrap
imprint --json --vault ./memory get r-2026-09-14-002
imprint --json --vault ./memory list --status active --scope go --min-confidence 0.85
imprint --json --vault ./memory show
imprint --json --vault ./memory viz
```

---

## See also

- [README.md](../README.md) — vault layout and full CLI  
- [mcp.md](mcp.md) — MCP mount and tools  
- [correction.zh.md](correction.zh.md) — 中文版  
- `.cursor/rules/imprint-memory.mdc` — alwaysApply agent rule
