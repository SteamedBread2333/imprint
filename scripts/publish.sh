#!/usr/bin/env bash
# Publish imprint:
#   - git tag vX.Y.Z (Go module consumers: go install ...@vX.Y.Z)
#   - GitHub Release with dist/*.tar.gz|zip (binaries)
#   - GitHub Packages / GHCR image ghcr.io/<owner>/imprint
#
# GitHub has no Go package registry. The Packages artifact is the container.
# Default path: tag + push, then GitHub Actions builds Release + GHCR.
# --local does Release + GHCR from this machine instead of waiting for Actions.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION=""
DRY_RUN=0
LOCAL=0
SKIP_DOCKER=0
ALLOW_DIRTY=0

usage() {
  cat <<'EOF'
Usage: scripts/publish.sh [v]X.Y.Z [flags]

Flags:
  --dry-run       Print actions; do not tag, push, or upload
  --local         Build dist + create GitHub Release + push GHCR here
  --skip-docker   Do not push ghcr.io (with --local)
  --allow-dirty   Allow a dirty working tree
  -h, --help      Show this help

Examples:
  scripts/publish.sh 1.0.0              # tag v1.0.0 and push (CI publishes)
  scripts/publish.sh v1.0.0 --local     # tag, push, and upload from this machine
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=1 ;;
    --local) LOCAL=1 ;;
    --skip-docker) SKIP_DOCKER=1 ;;
    --allow-dirty) ALLOW_DIRTY=1 ;;
    -h|--help) usage; exit 0 ;;
    -*)
      echo "unknown flag: $1" >&2
      usage >&2
      exit 2
      ;;
    *)
      VERSION="${1#v}"
      ;;
  esac
  shift
done

if [[ -z "$VERSION" ]]; then
  VERSION="$(sed -n 's/^[[:space:]]*Version[[:space:]]*=[[:space:]]*"\(.*\)".*/\1/p' pkg/imprint/record.go | head -1)"
fi
if [[ -z "$VERSION" ]]; then
  echo "version is empty; pass X.Y.Z" >&2
  exit 1
fi
TAG="v${VERSION}"

run() {
  echo "+" "$@"
  if [[ "$DRY_RUN" -eq 0 ]]; then
    "$@"
  fi
}

if [[ "$ALLOW_DIRTY" -eq 0 ]]; then
  if [[ -n "$(git status --porcelain)" ]]; then
    echo "working tree is dirty; commit or pass --allow-dirty" >&2
    exit 1
  fi
fi

echo "→ test"
if [[ "$DRY_RUN" -eq 0 ]]; then
  go test ./...
else
  echo "+ go test ./..."
fi

if git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "tag $TAG already exists" >&2
  exit 1
fi

run git tag -a "$TAG" -m "imprint $TAG"
run git push origin "$TAG"

if [[ "$LOCAL" -eq 0 ]]; then
  echo
  echo "tagged $TAG and pushed. GitHub Actions will attach Release assets and push GHCR."
  echo "Watch: gh run watch"
  echo "Then: go install …@${TAG} && imprint init"
  exit 0
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "gh is required for --local (https://cli.github.com/)" >&2
  exit 1
fi

echo "→ dist $VERSION"
if [[ "$DRY_RUN" -eq 0 ]]; then
  VERSION="$VERSION" bash scripts/dist.sh
else
  echo "+ VERSION=$VERSION bash scripts/dist.sh"
fi

notes="imprint ${TAG}

- Binaries: GitHub Release assets
- Container (GitHub Packages): ghcr.io/OWNER/imprint:${VERSION}
- Go: go install github.com/SteamedBread2333/imprint/cmd/imprint@${TAG}
"

if [[ "$DRY_RUN" -eq 0 ]]; then
  gh release create "$TAG" dist/* --title "imprint $TAG" --notes "$notes"
else
  echo "+ gh release create $TAG dist/*"
fi

if [[ "$SKIP_DOCKER" -eq 1 ]]; then
  echo "skip docker"
  exit 0
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found; skip GHCR (pass --skip-docker to silence)" >&2
  exit 0
fi

repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
owner_lc="$(printf '%s' "${repo%%/*}" | tr '[:upper:]' '[:lower:]')"
name_lc="$(printf '%s' "${repo##*/}" | tr '[:upper:]' '[:lower:]')"
image="ghcr.io/${owner_lc}/${name_lc}"
user="$(gh api user --jq .login)"
token="$(gh auth token)"

echo "→ ghcr ${image}:${VERSION}"
run docker login ghcr.io -u "$user" --password-stdin <<<"$token"
run docker buildx inspect --bootstrap >/dev/null
run docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --tag "${image}:${VERSION}" \
  --tag "${image}:latest" \
  --label "org.opencontainers.image.source=https://github.com/${repo}" \
  --push \
  "$ROOT"

echo
echo "published $TAG"
echo "  go install github.com/${repo}/cmd/imprint@${TAG}"
echo "  docker pull ${image}:${VERSION}"
echo "  imprint init   # Cursor alwaysApply rule"
