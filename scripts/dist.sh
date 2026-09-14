#!/usr/bin/env bash
# Cross-compile imprint and pack platform archives into dist/.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-$(bash "${ROOT}/scripts/version.sh")}"
VERSION="${VERSION:-devel}"
NAME="imprint"
OUT="${ROOT}/dist"
LDFLAGS="${LDFLAGS:--s -w} -X github.com/SteamedBread2333/imprint/pkg/imprint.Version=${VERSION}"

PLATFORMS=(
  darwin/amd64
  darwin/arm64
  linux/amd64
  linux/arm64
  windows/amd64
  windows/arm64
)

rm -rf "$OUT"
mkdir -p "$OUT"

checksum() {
  local f="$1"
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$f"
  else
    sha256sum "$f"
  fi
}

for spec in "${PLATFORMS[@]}"; do
  os="${spec%/*}"
  arch="${spec#*/}"
  ext=""
  [[ "$os" == windows ]] && ext=".exe"
  bin="${NAME}${ext}"
  stage="$(mktemp -d "${TMPDIR:-/tmp}/imprint-dist.XXXXXX")"

  echo "→ ${os}/${arch}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags="$LDFLAGS" -o "${stage}/${bin}" ./cmd/imprint
  mcp_bin="imprint-mcp${ext}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags="$LDFLAGS" -o "${stage}/${mcp_bin}" ./cmd/imprint-mcp

  docs=()
  for f in LICENSE README.md README.zh.md PROMPT.md docs/mcp.md docs/mcp.zh.md docs/correction.md docs/correction.zh.md docs/editors.md docs/editors.zh.md; do
    if [[ -f "$f" ]]; then
      docs+=("$f")
      cp "$f" "$stage/"
    fi
  done

  archive_base="${NAME}-${VERSION}-${os}-${arch}"
  if [[ "$os" == windows ]]; then
    (
      cd "$stage"
      zip -q "${OUT}/${archive_base}.zip" "$bin" "$mcp_bin" "${docs[@]}"
    )
  else
    tar -czf "${OUT}/${archive_base}.tar.gz" -C "$stage" "$bin" "$mcp_bin" "${docs[@]}"
  fi
  rm -rf "$stage"
done

(
  cd "$OUT"
  : > SHA256SUMS
  shopt -s nullglob
  for f in *.tar.gz *.zip; do
    checksum "$f" >> SHA256SUMS
  done
)

echo
echo "packed ${VERSION} → ${OUT}"
ls -lh "$OUT"
