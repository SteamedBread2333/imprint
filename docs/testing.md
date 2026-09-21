# Testing and acceptance

> **Setup:** [README.md](../README.md#quick-start). This page explains how to reproduce the real-user acceptance run.

## What it covers

`cmd/imprint-acceptance` drives the installed `imprint` and `imprint-mcp` binaries in a temporary project. It does not talk to the developer vault. Scenarios include:

1. `imprint init` layout
2. camelCase / Camel Case recall through Linguist language aliases (`ts` / `tsx`)
3. duplicate and privacy write rejects
4. empty vs explicit reinforce
5. find updates recall stats only
6. sweep → dormant wake candidate → reinforce
7. supersede / forget audit
8. concurrent CLI writers
9. compact MCP find/get
10. telemetry privacy and retention
11. `imprint report --days 30`
12. reopen after close

## Run

```bash
go install ./cmd/imprint ./cmd/imprint-mcp
go run ./cmd/imprint-acceptance --report .imprint/export/acceptance-report.md
```

The report is Chinese Markdown for humans. It never reprints claim, evidence, or secret text. A failing run still writes the report and exits non-zero.

Unit, race, and contract checks:

```bash
go test ./...
go test -race ./...
go test ./internal/cli -run TestDocsContract
```
