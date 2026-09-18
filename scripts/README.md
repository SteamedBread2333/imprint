# scripts

Shell helpers and dev-only utilities. Release scripts are invoked from the Makefile.

| Path | Purpose |
| --- | --- |
| [`release/dist.sh`](release/dist.sh) | Cross-compile `imprint` + `imprint-mcp` into `dist/` |
| [`release/publish.sh`](release/publish.sh) | Tag, push, optional local GitHub Release + GHCR |
| [`release/version.sh`](release/version.sh) | Print version from git tags |
| [`dev/imprint-mcp.sh`](dev/imprint-mcp.sh) | Run MCP bound to this repo (`--project`) |
| [`demo/seed-vault/`](demo/seed-vault/) | Idempotent demo vault + doc `sources` for desk unified graph |

## Common commands

```bash
make dist
make publish V=1.2.0

# MCP from repo root regardless of Cursor cwd
scripts/dev/imprint-mcp.sh

# Demo data (safe to re-run; skips existing claims)
go run ./scripts/demo/seed-vault
make demo-seed
```
