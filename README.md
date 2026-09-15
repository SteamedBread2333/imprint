<div align="center">
  <img alt="imprint" width="256"  src="https://github.com/user-attachments/assets/5c546072-5913-446b-b092-d255fc4fca90" />
  <h1>imprint</h1>
  <p><em>AI agent long-term memory</em></p>
  <p>
    <strong>English</strong> ·
    <a href="README.zh.md">中文</a>
  </p>
  <p>
    <a href="https://github.com/SteamedBread2333/imprint/releases"><img src="https://img.shields.io/github/v/release/SteamedBread2333/imprint?include_prereleases&style=flat-square" alt="release" /></a>
    <img src="https://img.shields.io/badge/go-1.25+-00ADD8?style=flat-square" alt="Go 1.25+" />
    <img src="https://img.shields.io/badge/license-MIT-c4a574?style=flat-square" alt="MIT" />
  </p>
</div>

User corrections → portable markdown. Go static binary, zero runtime, MIT.

Agents forget. Users repeat themselves. imprint is shared, file-backed memory: preferences and corrections in `.imprint/memory/`, recalled before the next edit. **You talk normally; the agent maintains the vault.**

## Quick start

**Three steps** (about two minutes):

```bash
go install github.com/SteamedBread2333/imprint/cmd/imprint@latest
go install github.com/SteamedBread2333/imprint/cmd/imprint-mcp@latest
cd your-project
imprint init
```

Optional — mount MCP in `.cursor/mcp.json` (merge [docs/examples/cursor-mcp.json](docs/examples/cursor-mcp.json)). Restart the editor. Skip MCP if you prefer `imprint --json` only.

| Step | You do | imprint does |
| --- | --- | --- |
| 1 | Install + `imprint init` | Writes agent rules for Cursor, Claude Code, Codex, Trae, Workbuddy |
| 2 | Code and correct in plain language | Agent `find` → ADD / REINFORCE / SUPERSEDE / IGNORE → writes `.imprint/memory/` |
| 3 | `imprint host serve` + `imprint desk open` when curious | Live list / graph dashboard (desk plugin) |

Further reading → [Documentation](#documentation) (all files under `docs/`). Desk screenshots → [imprint-desk-plugin](https://github.com/SteamedBread2333/imprint-desk-plugin).

```mermaid
flowchart LR
  Setup["install · init"] -.-> CLI

  User((User)) --> Agent[Agent]
  Human((Human))

  Agent <-->|find · write · get| CLI["imprint --json"]
  Human <-->|list · viz · export| CLI

  CLI <-->|read / write| Vault[(.imprint/memory/)]

  Vault --- Disk

  subgraph Disk["on disk"]
    direction TB
    Shards[imprint-NNNN.md]
    Archive[archive/]
    Notes[notes/]
  end
```

```mermaid
sequenceDiagram
  autonumber
  participant U as User
  participant A as Agent
  participant C as imprint CLI
  participant V as Vault

  U->>A: preference / correction / decision
  A->>C: find — scope AND, then BM25
  C->>V: read shards
  V-->>C: hits
  C-->>A: ranked rules

  alt ADD · REINFORCE · SUPERSEDE
    A->>C: add / reinforce / supersede
    C->>V: update markdown
  else IGNORE
    A->>A: continue without write
  end

  Note over A,C: get — evidence_log & referenced_by when one rule matters
```

| Phase | What happens |
| --- | --- |
| **Bootstrap** | [Quick start](#quick-start) — `imprint init`; optional MCP ([docs/mcp.md](docs/mcp.md)). Vault: `.imprint/memory/`, or `~/.imprint` with `--global`, or `--vault` / `IMPRINT_VAULT`. |
| **Agent loop** | `find` recalls; the agent classifies **ADD / REINFORCE / SUPERSEDE / IGNORE**; writes go through `add`, `reinforce`, or `supersede` (old rule → `archive/`). `get` loads one rule with evidence and backlinks. |
| **Vault** | Rules pack into `imprint-NNNN.md` shards; `sweep` and `supersede` move copies to `archive/`; `forget` deletes and strips inbound links. |
| **Human views** | Same CLI: `list`, `show`, `get`, `export`. `viz` prints mermaid or writes read-only `notes/` cards. Interactive list / graph UI is the **desk** plugin. |
| **Not here** | Whether to write at all is the agent's judgment call (ADD / REINFORCE / SUPERSEDE / IGNORE). |

Agents use MCP tools or `imprint --json`. `notes/` is generated; do not hand-edit it.

## Install

See [Quick start](#quick-start). Also: [GitHub Releases](https://github.com/SteamedBread2333/imprint/releases) binaries, or:

```bash
docker pull ghcr.io/steamedbread2333/imprint:latest
```

From a clone:

```bash
make install
# or
make build   # writes ./imprint
make dist    # cross-compile darwin/linux/windows → dist/*.tar.gz|zip
make publish V=X.Y.Z   # tag vX.Y.Z and push (CI uploads Release + GHCR)
```

GitHub has no Go package registry. Publishing puts **binaries on the GitHub Release** and the **image on GHCR (Packages)**.

The version you type at publish time is the only one that matters: it becomes the git tag, the module version, `imprint version`, and the GHCR label. Local builds without a tag print `devel`.

## Vault

Default project vault is `./.imprint/memory/` (walk up from cwd for `.imprint/`). `--global` uses `~/.imprint`. `--vault PATH` and `IMPRINT_VAULT` override both.

```
.imprint/                 # imprint project home (config + vault + caches)
  imprint.yaml            # plugins + host
  memory/                 # vault (rules)
  .shelves/.cache/        # doc index (SQLite); gitignore
docs/                     # your project docs (shelves indexes this)
```

Rules are packed into `imprint-NNNN.md` shards (legacy `r-YYYY-MM-DD-NNN.md` files are still read and compacted on open). A new shard starts at **32768 lines** or **1 MiB**, whichever comes first — large enough that a typical project stays in one file, small enough that reinforce still rewrites well under a millisecond on SSD.

Several frontmatter documents live in one shard. `forget` removes one document, not the file, and strips that id from other rules’ `related` / `supersedes` / `conflicts_with`. Marshal may append a derived `See also: [[r-…]]` footer; YAML remains the source of truth.

```markdown
<!-- imprint pack (2 rules) -->
---
id: r-2026-09-11-001
claim: Python function names must always be snake_case
scope:
    - python
    - naming
confidence: 0.6
status: active
reinforcement_count: 0
created_at: 2026-09-11T12:00:00Z
updated_at: 2026-09-11T12:00:00Z
last_touched_at: 2026-09-11T12:00:00Z
supersedes: []
related: []
conflicts_with: []
evidence_log:
    - at: 2026-09-11T12:00:00Z
      kind: original
      text: use snake_case
---
---
id: r-2026-09-11-002
claim: Use 4-space indents
scope:
    - python
    - style
confidence: 0.85
status: active
...
---
```

IDs are `r-YYYY-MM-DD-NNN`. `forget` deletes the rule. `supersede` and `sweep` only archive.

## CLI

```bash
imprint add "Python function names must always be snake_case" \
  --scope python,naming --text "use snake_case"

imprint find --scope python,naming
imprint find --scope go --query PascalCase
imprint reinforce r-2026-09-11-001 --evidence "user confirmed again"
imprint supersede r-2026-09-11-001 \
  --claim "All JS/Python functions must use snake_case" \
  --scope javascript,python,naming \
  --reason "extended to frontend"

imprint list --status active --scope python,naming --min-confidence 0.85
imprint get r-2026-09-11-001          # includes evidence_log and referenced_by
imprint show
imprint sweep
imprint viz                          # mermaid on stdout
imprint viz --format mermaid --out graph.mmd
imprint viz --format notes           # .imprint/memory/notes/*.md, overwrite, do not edit
imprint export
imprint init                         # rules for Cursor, Claude, Codex, Trae, Workbuddy
imprint init --cursor --force        # one editor only
imprint host serve                   # vault read-only HTTP API (plugins)
imprint plugin list|enable|disable|reload
imprint desk open                    # open desk-plugin UI
imprint forget r-2026-09-11-001
imprint clear --confirm --yes        # irreversible
```

`--json` prints machine-readable JSON on stdout so agents and scripts can parse it.

Global flags: `--vault PATH`, `--global`, `--json`.

## Plugins

Optional UI and doc search ship as **separate repos**, enabled via [`.imprint/imprint.yaml`](docs/examples/imprint.yaml):

| Plugin | Repo | Role |
| --- | --- | --- |
| **desk** | [imprint-desk-plugin](https://github.com/SteamedBread2333/imprint-desk-plugin) | Live dashboard SPA (list / graph, IndexedDB cache, shelves doc search, EN/中文) |
| **shelves** | [imprint-shelves-plugin](https://github.com/SteamedBread2333/imprint-shelves-plugin) | Workspace doc index (SQLite + BM25); `doc_search` proxied through **imprint-mcp** |

```bash
imprint host serve          # 127.0.0.1:9470 — GET /graph, /rules, /find, …
imprint plugin reload       # start enabled plugins from manifest
imprint desk open
```

Agent still mounts **one** MCP (`imprint-mcp`). Plugin tools come from each plugin's `imprint.plugin.json`; the host does not hardcode shelves APIs.

### Integration test fixture

Sibling repo **`memory-test`** simulates the **Mercury** hotel-booking team — 3000 realistic vault rules and 20 engineering docs:

```bash
cd ../memory-test && ./scripts/seed.sh && ./scripts/selftest.sh
```

## Documentation

Setup lives in [Quick start](#quick-start) above. Everything under [`docs/`](docs/) is **reference** — linked here so nothing is orphaned.

| Topic | English | 中文 |
| --- | --- | --- |
| AI editor `init` paths & flags | [docs/editors.md](docs/editors.md) | [docs/editors.zh.md](docs/editors.zh.md) |
| MCP server & mount | [docs/mcp.md](docs/mcp.md) | [docs/mcp.zh.md](docs/mcp.zh.md) |
| Memory correction & coding scenarios | [docs/correction.md](docs/correction.md) | [docs/correction.zh.md](docs/correction.zh.md) |

**MCP examples** (merge manually; `imprint init` does not write these):

| File | Use |
| --- | --- |
| [docs/examples/cursor-mcp.json](docs/examples/cursor-mcp.json) | Project vault `./.imprint/memory` |
| [docs/examples/cursor-mcp-global.json](docs/examples/cursor-mcp-global.json) | Global vault `~/.imprint` |

## Release

```bash
# CI path (recommended): tests, tag, push; Actions uploads Release + GHCR
make publish V=X.Y.Z
# same as: scripts/publish.sh X.Y.Z

# From this machine instead of waiting for Actions
scripts/publish.sh X.Y.Z --local
```

Pushing tag `vX.Y.Z` runs [`.github/workflows/release.yml`](.github/workflows/release.yml) (binaries + GHCR). First GHCR image is private until you set the package to Public (GitHub → Packages → imprint → Package settings).

## Go module

```go
import "github.com/SteamedBread2333/imprint/pkg/imprint"

v, err := imprint.Open("./.imprint/memory")
res, err := v.Add("Use gofmt", []string{"go"}, "gofmt", 0.6)
hits, err := v.Find([]string{"go"}, "", 5)
```

## What this binary does not do

Gatekeeper classification (ADD / REINFORCE / SUPERSEDE / IGNORE), sensitive-data refusal, and “should I write this?” judgment are the agent’s job. imprint is the honest store: write, recall, reinforce, supersede, decay.

## License

MIT
