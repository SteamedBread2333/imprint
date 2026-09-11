#!/usr/bin/env bash
# Cross-compile imprint and pack platform archives into dist/.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-$(sed -n 's/^[[:space:]]*Version[[:space:]]*=[[:space:]]*"\(.*\)".*/\1/p' pkg/imprint/record.go | head -1)}"
VERSION="${VERSION:-0.0.0}"
NAME="imprint"
OUT="${ROOT}/dist"
LDFLAGS="${LDFLAGS:--s -w}"

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
  cp LICENSE README.md PROMPT.md "$stage/"

  archive_base="${NAME}-${VERSION}-${os}-${arch}"
  if [[ "$os" == windows ]]; then
    (
      cd "$stage"
      zip -q "${OUT}/${archive_base}.zip" "$bin" LICENSE README.md PROMPT.md
    )
  else
    tar -czf "${OUT}/${archive_base}.tar.gz" -C "$stage" "$bin" LICENSE README.md PROMPT.md
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
