#!/usr/bin/env bash
# Print the current version with no leading v.
# Source of truth: git tags (Go module versioning). Untagged trees: devel.
set -euo pipefail
if ! command -v git >/dev/null 2>&1 || ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo devel
  exit 0
fi
if ver=$(git describe --tags --match 'v*' --exact-match HEAD 2>/dev/null); then
  echo "${ver#v}"
  exit 0
fi
if ver=$(git describe --tags --match 'v*' --always --dirty 2>/dev/null); then
  echo "${ver#v}"
  exit 0
fi
echo devel
