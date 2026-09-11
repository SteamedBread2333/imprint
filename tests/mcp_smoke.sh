#!/usr/bin/env bash
# End-to-end MCP stdio smoke against ./imprint mcp.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="${ROOT}/imprint"
VAULT="${ROOT}/tests/mcpvault"
REQ="${ROOT}/tests/mcp_requests.jsonl"
OUT="${ROOT}/tests/mcp_responses.jsonl"

cd "$ROOT"
[[ -x "$BIN" ]] || make build

rm -rf "$VAULT"
mkdir -p "$VAULT/archive"

echo "vault=${VAULT}"
echo "bin=${BIN}"
echo "→ imprint mcp < $(basename "$REQ")"

"$BIN" --vault "$VAULT" mcp < "$REQ" > "$OUT"

echo "responses: $(grep -c '^{' "$OUT" || true) lines → ${OUT}"
echo "--- CLI cross-check ---"
"$BIN" --vault "$VAULT" list
echo "--- mermaid ---"
"$BIN" --vault "$VAULT" viz --format mermaid --include-archived | head -40
