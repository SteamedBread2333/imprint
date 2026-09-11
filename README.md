<div align="center">
  <img src="docs/logo.jpg" alt="imprint" width="168" />
  <h1>imprint</h1>
  <p><em>AI agent long-term memory</em></p>
  <p>
    <strong>English</strong> ·
    <a href="README.zh.md">中文</a>
  </p>
  <p>
    <a href="https://github.com/SteamedBread2333/imprint/releases"><img src="https://img.shields.io/github/v/release/SteamedBread2333/imprint?include_prereleases&style=flat-square" alt="release" /></a>
    <img src="https://img.shields.io/badge/go-1.23+-00ADD8?style=flat-square" alt="Go 1.23+" />
    <img src="https://img.shields.io/badge/license-MIT-c4a574?style=flat-square" alt="MIT" />
  </p>
</div>

User corrections → portable markdown. Go static binary, zero runtime, MIT.

Agents forget. Users repeat themselves. imprint is the file-backed memory they share: preferences, corrections, and decisions, stored as ordinary markdown, recalled by scope, superseded instead of stacked.

The protocol an agent should follow lives in [PROMPT.md](PROMPT.md). This repository is the companion implementation.

## Install

```bash
go install github.com/SteamedBread2333/imprint/cmd/imprint@latest
```

Requires Go 1.23+. `CGO_ENABLED=0` — no libc, no database, no network at runtime.

Or download a platform archive from [GitHub Releases](https://github.com/SteamedBread2333/imprint/releases), or the container from GitHub Packages:

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

Default project vault is `./memory/` (or the nearest `memory/` walking up from cwd). `--global` uses `~/.imprint`. `--vault PATH` and `IMPRINT_VAULT` override both.

```
memory/
  imprint-0001.md
  archive/
    imprint-0001.md
  dashboard.html
```

Rules are packed into `imprint-NNNN.md` shards (legacy `r-YYYY-MM-DD-NNN.md` files are still read and compacted on open). A new shard starts at **32768 lines** or **1 MiB**, whichever comes first — large enough that a typical project stays in one file, small enough that reinforce still rewrites well under a millisecond on SSD.

Several frontmatter documents live in one shard. `forget` removes one document, not the file.

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
imprint reinforce r-2026-09-11-001 --evidence "user confirmed again"
imprint supersede r-2026-09-11-001 \
  --claim "All JS/Python functions must use snake_case" \
  --scope javascript,python,naming \
  --reason "extended to frontend"

imprint list --status active
imprint get r-2026-09-11-001
imprint show
imprint sweep
imprint viz                          # ./memory/dashboard.html
imprint viz --format mermaid
imprint export
imprint init                         # alwaysApply Cursor rule
imprint forget r-2026-09-11-001
imprint clear --confirm --yes        # irreversible
```

`--json` prints the §21 shapes so agents and scripts can parse stdout.

Global flags: `--vault PATH`, `--global`, `--json`.

## Cursor

Install the binary globally, then drop an `alwaysApply` rule in the project:

```bash
go install github.com/SteamedBread2333/imprint/cmd/imprint@latest
cd your-project
imprint init
```

That writes `.cursor/rules/imprint-memory.mdc`. Agents run `imprint --json` against `./memory/` (or `IMPRINT_VAULT` / `--global` for `~/.imprint`). There is no MCP server.

## Release

```bash
# CI path (recommended): tests, tag, push; Actions uploads Release + GHCR
make publish V=X.Y.Z
# same as: scripts/publish.sh X.Y.Z

# From this machine instead of waiting for Actions
scripts/publish.sh X.Y.Z --local
```

Pushing tag `vX.Y.Z` runs [`.github/workflows/release.yml`](.github/workflows/release.yml). First GHCR image is private until you set the package to Public (GitHub → Packages → imprint → Package settings).

## Go module

```go
import "github.com/SteamedBread2333/imprint/pkg/imprint"

v, err := imprint.Open("./memory")
res, err := v.Add("Use gofmt", []string{"go"}, "gofmt", 0.6)
hits, err := v.Find([]string{"go"}, "", 5)
```

## What this binary does not do

Gatekeeper classification (ADD / REINFORCE / SUPERSEDE / IGNORE), sensitive-data refusal, and “should I write this?” judgment are the agent’s job. See [PROMPT.md](PROMPT.md). imprint is the honest store: write, recall, reinforce, supersede, decay.

## License

MIT
