#!/usr/bin/env sh
# Bind MCP to this repo regardless of Cursor spawn cwd.
set -e
ROOT=$(CDPATH= cd "$(dirname "$0")/../.." && pwd)
exec imprint-mcp --project "$ROOT"
