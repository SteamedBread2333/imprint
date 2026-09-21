<div align="center">

<img alt="imprint logo" width="200" src="assets/logo.png" />

# imprint

**A vault of project policy for AI agents**

English · [中文](README.zh.md)

[![release](https://img.shields.io/github/v/release/SteamedBread2333/imprint?include_prereleases&style=flat-square)](https://github.com/SteamedBread2333/imprint/releases)
![Go 1.25+](https://img.shields.io/badge/go-1.25+-00ADD8?style=flat-square)
![MIT](https://img.shields.io/badge/license-MIT-c4a574?style=flat-square)

</div>

Conversation becomes durable rules. The agent classifies each turn, links rules to project docs, and recalls before the next edit. **You talk normally; you do not maintain the vault.**

| | When | Example |
| --- | --- | --- |
| **ADD** | New long-term preference | Naming rule, workflow, architecture boundary |
| **REINFORCE** | Same policy again | “Yes, still PascalCase for exports” |
| **SUPERSEDE** | Policy changed | Narrow scope, widen scope, replace claim |
| **IGNORE** | Task-only turn | One-off refactor, chit-chat, secrets |

When the user rejects a stored rule (“don’t record that”), the agent `find`s and calls `forget` — not one of the four classifications above. See [Memory writes](docs/correction.md).

Vault stores claim, evidence, and optional doc pointers (`sources`). Shelves indexes markdown under `roots` for BM25 recall alongside rules.

---

## Quick start

```bash
go install github.com/SteamedBread2333/imprint/cmd/imprint@latest
go install github.com/SteamedBread2333/imprint/cmd/imprint-mcp@latest
cd your-project && imprint init
```

Merge [docs/examples/cursor-mcp.json](docs/examples/cursor-mcp.json) into `.cursor/mcp.json` and restart the editor. CLI fallback: `imprint --json`.

| | You | imprint |
| --- | --- | --- |
| **1** | Install + `init` | Writes `imprint.yaml` and editor rules (Cursor, Claude Code, Codex, Trae, Workbuddy) |
| **2** | Speak normally | Agent `find` (vault + shelves) → classify → `add`/`reinforce`/… (optional `sources`) |
| **3** | Browser audit (optional) | `imprint up` → `imprint desk open` |

```mermaid
flowchart TB
  U((User<br/>speaks normally))

  subgraph Agent["Agent · imprint-mcp"]
    direction TB
    F["find(scope, query)"]
    C{"Classify<br/>ADD · REINFORCE · SUPERSEDE · IGNORE"}
    W["add / supersede<br/>optional sources"]
    R["find again before coding · cite [r-id]"]
  end

  subgraph Store["imprint storage"]
    direction LR
    Vault[(".imprint/vault.db<br/>claim · evidence · sources")]
    Shelves[(".imprint/state/shelves.db<br/>roots doc BM25")]
  end

  subgraph FindOut["one find call"]
    direction LR
    FR["rules<br/>resolved_sources"]
    FD["documents<br/>snippet"]
    FL["links"]
  end

  subgraph Audit["Optional · human audit"]
    H((You)) --> Desk["desk · show"]
  end

  UP["imprint up"] -.->|host indexes| Shelves

  U -->|speak / task| Agent
  F --> Vault
  F --> Shelves
  Vault --> FR
  Shelves --> FD
  Vault --> FL
  Shelves --> FL
  FR & FD & FL --> C
  C -->|persist| W
  W --> Vault
  R --> F
  Desk --> Vault
  Desk --> Shelves
```

<details>
<summary>Before coding (sequence)</summary>

```mermaid
sequenceDiagram
  participant U as User
  participant A as Agent
  participant V as vault
  participant S as shelves

  U->>A: new task / continue coding
  A->>V: find(scope, query)
  V->>S: BM25 docs (when shelves on)
  V-->>A: rules · documents · links
  A->>A: code from imprints + excerpts
  Note over A: cite [r-id] and doc path
```

</details>

---

## CLI

Global flags: `--vault PATH` · `--global` · `--json` (machine-readable stdout for agents)

### Everyday — vault

Read and write the vault directly from the CLI.

| Command | What it does |
| --- | --- |
| `imprint init` | Write `imprint.yaml` (if missing) and editor agent rules |
| `imprint find [--scope a,b] [--query TEXT]` | Recall imprints (scope **AND**); with query + shelves → also `documents`, `links` |
| `imprint add CLAIM --scope a,b --text ORIG` | Create an imprint (MCP may include `sources` linking docs) |
| `imprint reinforce ID --evidence TEXT` | Strengthen a rule after explicit reaffirmation (diminishing gain, cap 0.95) |
| `imprint supersede OLD --claim NEW --scope a,b` | Replace a rule; old → `superseded` |
| `imprint forget ID` | Delete permanently (lifecycle event remains) |
| `imprint list` · `show` · `get ID` · `report` | Browse, inspect, and audit lifecycle / recall / telemetry |
| `imprint sweep` · `export` | Decay stale rules · write `.imprint/export/vault.json` or `vault.jsonl` (`--format jsonl`) |

```bash
imprint find --scope go,naming --query PascalCase
imprint add "Use snake_case" --scope python,naming --text "user said snake_case"
imprint get r-2026-09-11-001
```

### Local stack

| Command | What it does |
| --- | --- |
| `imprint up` | Start the local stack (vault API, shelves, enabled desk, …) |
| `imprint down` | Stop the local stack |
| `imprint desk open` | Open desk in the browser (requires `up` first) |
| `imprint status` | Snapshot of running services |

```bash
imprint up && imprint desk open
# when done:
imprint down
```

After editing `imprint.yaml`: run `down` then `up`.

<details>
<summary>Advanced: split host / plugin control (debug)</summary>

| Command | What it does |
| --- | --- |
| `imprint host start` / `host stop` | Host only (includes shelves) |
| `imprint plugin start` / `plugin stop` | External plugins only |
| `plugin list` · `enable` · `disable` | Toggle plugins in yaml |

</details>

Shelves is top-level host config (`shelves:` in `imprint.yaml`). See [docs/shelves-builtin.md](docs/shelves-builtin.md).

### Debug & advanced

| Command | What it does |
| --- | --- |
| `imprint host serve [--listen ADDR]` | Foreground host (Ctrl+C) |
| `imprint clear --confirm --yes` | Delete every rule — irreversible |
| `imprint version` | Print version |

Run `imprint --help` or `imprint help <cmd>` for full flags.

---

## Vault layout

Team convention: commit `imprint.yaml` (roots, plugin switches). All private runtime is gitignored under `.imprint/`.

Default vault directory: `./.imprint/` (walk up for `imprint.yaml` or `.imprint/`). `--global` → `~/.imprint`. Override with `--vault` or `IMPRINT_VAULT`.

```
imprint.yaml            # git: host + shelves.roots + plugins + supersede.inheritance_alpha
.imprint/               # gitignore: private runtime
  vault.db              # SQLite vault (rules, evidence, edges, sources)
  state/
    shelves.db          # rebuildable doc index
    plugins/            # plugin derived state
  export/               # optional md/json projections
docs/                   # typical shelves root
.cursor/rules/          # typical shelves root
```

Rules live in `vault.db`. IDs: `r-YYYY-MM-DD-NNN`. Status: `active` | `dormant` | `superseded`. `supersede` marks old rules superseded; `sweep` decays stale rules to dormant; `forget` deletes. Interactive rule graph: **`imprint desk open`** (host `GET /graph`).

---

## Shelves & desk

| | Where it runs | Config |
| --- | --- | --- |
| **Shelves** | **host** | `shelves.enabled`, `config.roots` in [imprint.yaml](docs/examples/imprint.yaml) |
| **Desk** | External plugin | `plugins.desk` + [imprint-desk-plugin](https://github.com/SteamedBread2333/imprint-desk-plugin) |

**Shelves** indexes markdown under `roots` in `imprint.yaml` (e.g. `docs/`, `.cursor/rules/`). Before coding, `find(scope, query)` returns vault rules, document snippets, and `links` in one call. Local BM25 index.

**`roots`** lists which directories enter the index — scoped recall and fast rebuilds. See [docs/shelves-builtin.md](docs/shelves-builtin.md).

**Compact recall:** MCP `find` returns claim/count metadata and bounded document snippets by default; `get r-…` folds evidence and source text. Use `include_evidence` or `full` only for audit. Dormant rules can contribute at most one penalized `wake_candidate`; only an explicit `reinforce` wakes one.

**Write safety:** `add`, `reinforce`, and `supersede` reject likely secrets and personal data. Source paths must remain inside the workspace and may not target credentials, `.env*`, `*.pem`, or `*.key`. `add` also rejects high-similarity active duplicates with candidate IDs. `reinforce` requires non-empty evidence and uses diminishing confidence gain; find hits only update recall stats. `supersede` decays successor confidence by `supersede.inheritance_alpha` of the gap above 0.6 (default 0.20 in `imprint.yaml`; 0 copies old, 1 drops to baseline). Language scope tags (`ts`, `tsx`, `golang`) are canonicalized with GitHub Linguist via go-enry; non-language tags pass through.

**Audit:** `imprint report --days 30` summarizes lifecycle events, duplicates, conflicts, zero-recall rules, and telemetry latency. Telemetry is a daily JSONL under `.imprint/state/telemetry/` and never stores query, claim, evidence, or path text.

**Linking:** Vault `sources` point rules at doc paths or headings; compact `get r-…` returns source pointers, while `full:true` resolves excerpts. Design: [docs/imprint-shelves-linking.md](docs/imprint-shelves-linking.md).

One MCP mount (`imprint-mcp`). With shelves on and `find` + query, the response includes `rules`, `documents`, and `links`.

---

## Install & release

```bash
docker pull ghcr.io/steamedbread2333/imprint:latest   # or GitHub Releases binaries
make install          # from clone
make publish V=X.Y.Z  # tag + CI → Release + GHCR
```

Local builds without a tag print `devel`.

---

## Documentation

| Topic | English | 中文 |
| --- | --- | --- |
| MCP mount | [docs/mcp.md](docs/mcp.md) | [docs/mcp.zh.md](docs/mcp.zh.md) |
| Editor `init` | [docs/editors.md](docs/editors.md) | [docs/editors.zh.md](docs/editors.zh.md) |
| Shelves & linking | [docs/shelves-builtin.md](docs/shelves-builtin.md) · [docs/imprint-shelves-linking.md](docs/imprint-shelves-linking.md) | [docs/shelves-builtin.zh.md](docs/shelves-builtin.zh.md) · [docs/imprint-shelves-linking.zh.md](docs/imprint-shelves-linking.zh.md) |
| Write loop & scenarios | [docs/correction.md](docs/correction.md) | [docs/correction.zh.md](docs/correction.zh.md) |
| Testing & acceptance | [docs/testing.md](docs/testing.md) | [docs/testing.zh.md](docs/testing.zh.md) |

MCP examples (merge manually): [cursor-mcp.json](docs/examples/cursor-mcp.json) · [cursor-mcp-global.json](docs/examples/cursor-mcp-global.json)

---

## Go module

```go
import "github.com/SteamedBread2333/imprint/pkg/imprint"

v, _ := imprint.Open("./.imprint")
v.Add("Use gofmt", []string{"go"}, "gofmt", 0.6)
v.Find([]string{"go"}, "", 5)
```

MIT
