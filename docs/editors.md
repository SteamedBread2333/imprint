# AI editor initialization

> **Setup:** [README.md](../README.md#quick-start) — install and `imprint init`.  
> This page is **reference** (paths, flags, Codex merge rules).

`imprint init` writes the same imprint rules per editor. It does **not** write MCP config.

```bash
imprint init              # all editors
imprint init --cursor     # one editor
imprint init --force      # overwrite / refresh
```

## Output paths

| Editor | Flag | Project path |
| --- | --- | --- |
| Cursor | `--cursor` | `.cursor/rules/imprint-memory.mdc` |
| Claude Code | `--claude` | `.claude/rules/imprint-memory.md` |
| Codex | `--codex` | see below |
| Trae | `--trae` | `.trae/rules/imprint-memory.md` |
| Workbuddy | `--workbuddy` | `.codebuddy/rules/imprint-memory/RULE.mdc` |

Shared body: `internal/cli/templates/body.md`.

## Codex (official AGENTS.md)

Codex has **no** separate rules directory like Cursor. Per [OpenAI’s AGENTS.md guide](https://developers.openai.com/codex/guides/agents-md), each directory contributes **at most one** instruction file:

1. Non-empty **`AGENTS.override.md`** wins ( **`AGENTS.md` in the same directory is skipped** )
2. Otherwise **`AGENTS.md`**
3. Files from repo root → cwd are **concatenated whole** into context

So `imprint init --codex` does **not** use HTML comment markers or a sidecar file. It writes a markdown section:

| Situation | Behavior |
| --- | --- |
| Target file missing | Create `AGENTS.md` (or write the active override path) with a `## imprint memory` section |
| `## imprint memory` already present | Replace that section only |
| File exists, no section | Refuse unless `--force` (appends section) |

**Target file:**

- Non-empty root `AGENTS.override.md` → merge into **override** (what Codex actually loads)
- Else → merge into **`AGENTS.md`**

This matches Codex’s pattern of adding sections such as `## Code Review Rules` inside the same AGENTS file. Internal Must do / Must not headings use `###` so the section ends at the next peer `##`.

**Warning:** Do not create `AGENTS.override.md` casually when the team already relies on `AGENTS.md` — override **replaces** `AGENTS.md` for Codex in that directory.

## MCP mount (manual)

See [mcp.md](mcp.md). `init` never writes MCP JSON.

## See also

- [correction.md](correction.md)  
- [editors.zh.md](editors.zh.md)
