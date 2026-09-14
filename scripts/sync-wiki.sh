#!/usr/bin/env bash
# Sync docs/ (+ README pointers) to the GitHub Wiki git repository.
# Requires: GITHUB_REPOSITORY, GITHUB_TOKEN
# Optional: GITHUB_REF_NAME, GITHUB_SHA
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REPO="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY required}"
TOKEN="${GITHUB_TOKEN:?GITHUB_TOKEN required}"
REF="${GITHUB_REF_NAME:-main}"
SHA="${GITHUB_SHA:-local}"
WIKI_URL="https://x-access-token:${TOKEN}@github.com/${REPO}.wiki.git"

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

clone_or_init_wiki() {
  if git clone --depth=1 "$WIKI_URL" "$WORKDIR/wiki" 2>/dev/null; then
    return 0
  fi
  echo "→ wiki repo empty or missing; initializing"
  mkdir -p "$WORKDIR/wiki"
  git -C "$WORKDIR/wiki" init -b master
  git -C "$WORKDIR/wiki" remote add origin "$WIKI_URL"
}

write_home() {
  local w="$WORKDIR/wiki/Home.md"
  cat >"$w" <<EOF
# imprint

Documentation synced from [${REPO}](https://github.com/${REPO}) (\`${REF}\`, \`${SHA:0:7}\`).

**Setup (install, \`imprint init\`, MCP)** lives in the repo [README](https://github.com/${REPO}#quick-start) — not duplicated here.

## Index

| Topic | English | 中文 |
| --- | --- | --- |
| AI editors | [[Editors]] | [[Editors-zh]] |
| MCP | [[MCP]] | [[MCP-zh]] |
| Memory correction | [[Correction]] | [[Correction-zh]] |

## MCP examples (repo files)

- [cursor-mcp.json](https://github.com/${REPO}/blob/main/docs/examples/cursor-mcp.json) — project vault
- [cursor-mcp-global.json](https://github.com/${REPO}/blob/main/docs/examples/cursor-mcp-global.json) — global vault
EOF
}

copy_docs() {
  cp "$ROOT/docs/editors.md" "$WORKDIR/wiki/Editors.md"
  cp "$ROOT/docs/editors.zh.md" "$WORKDIR/wiki/Editors-zh.md"
  cp "$ROOT/docs/mcp.md" "$WORKDIR/wiki/MCP.md"
  cp "$ROOT/docs/mcp.zh.md" "$WORKDIR/wiki/MCP-zh.md"
  cp "$ROOT/docs/correction.md" "$WORKDIR/wiki/Correction.md"
  cp "$ROOT/docs/correction.zh.md" "$WORKDIR/wiki/Correction-zh.md"
}

push_wiki() {
  local w="$WORKDIR/wiki"
  git -C "$w" config user.name "github-actions[bot]"
  git -C "$w" config user.email "41898282+github-actions[bot]@users.noreply.github.com"
  write_home
  copy_docs
  git -C "$w" add -A
  if git -C "$w" diff --staged --quiet; then
    echo "wiki unchanged"
    return 0
  fi
  git -C "$w" commit -m "docs: sync from ${REF} (${SHA:0:7})"
  local branch
  branch="$(git -C "$w" branch --show-current 2>/dev/null || true)"
  if [[ -z "$branch" ]]; then
    branch=master
  fi
  git -C "$w" push origin "HEAD:${branch}"
  echo "→ pushed wiki (${branch})"
}

clone_or_init_wiki
push_wiki
