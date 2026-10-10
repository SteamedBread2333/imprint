#!/usr/bin/env bash
# End-to-end add() gate on isolated vault + live sidecar (duplicate/conflict samples).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SIDECAR="$(cd "$ROOT/../imprint-embed-sidecar" 2>/dev/null && pwd || true)"
EVAL_DIR="${TMPDIR:-/tmp}/imprint-embed-eval-$$"
VAULT="$EVAL_DIR/vault"
IMPRINT="${IMPRINT_BIN:-imprint}"

mkdir -p "$EVAL_DIR"
cat >"$EVAL_DIR/imprint.yaml" <<EOF
host:
  listen: 127.0.0.1:9470
plugins:
  embed:
    enabled: true
    package: ${SIDECAR:-../imprint-embed-sidecar}
    config:
      port: 4174
      model: BAAI/bge-small-zh-v1.5
      relate_model: cross-encoder/nli-deberta-v3-base
      duplicate_threshold: 0.65
      relate_contradiction_min: 0.40
      relate_entailment_min: 0.35
      relate_bidirectional: true
      duplicate_high_cosine_block: 0.80
      timeout_seconds: 8
EOF

cd "$EVAL_DIR"
IMPRINT_FLAGS=(--vault "$VAULT")

if ! curl -sf --noproxy '*' http://127.0.0.1:4174/health | grep -q '"ready": true'; then
  echo "embed sidecar not ready on 4174; run: imprint plugin start embed" >&2
  exit 1
fi

pass=0
fail=0
note() { echo "  $*"; }

try_add() {
  local expect="$1" claim="$2" scope="$3" text="$4"
  local out
  out=$("$IMPRINT" "${IMPRINT_FLAGS[@]}" --json add "$claim" --scope "$scope" --text "$text" 2>&1) || true
  if echo "$out" | grep -q '"code":"duplicate"'; then
    [[ "$expect" == duplicate ]] && { note "OK duplicate: $claim"; pass=$((pass + 1)); return; }
    note "FAIL wanted $expect got duplicate: $claim"
    echo "$out" | head -3
    fail=$((fail + 1))
    return
  fi
  if echo "$out" | grep -q '"id":"r-'; then
    if [[ "$expect" == add ]]; then
      if echo "$out" | grep -q '"similar"'; then
        [[ "$expect" == advisory ]] || { note "OK add (similar?): $claim"; pass=$((pass + 1)); return; }
      fi
      note "OK add: $claim"
      pass=$((pass + 1))
      return
    fi
    if [[ "$expect" == advisory ]]; then
      if echo "$out" | grep -q '"similar"'; then
        note "OK advisory/similar: $claim"
        pass=$((pass + 1))
        return
      fi
    fi
    note "FAIL wanted $expect got add: $claim"
    echo "$out" | head -3
    fail=$((fail + 1))
    return
  fi
  note "FAIL unexpected: $claim"
  echo "$out"
  fail=$((fail + 1))
}

echo "=== vault E2E (isolated $VAULT) ==="

SCOPE="eval,embed"
try_add add "Go eval rule uses PascalCase for exports" "$SCOPE" "baseline"
try_add duplicate "Exported eval identifiers must use PascalCase naming" "$SCOPE" "paraphrase"

SCOPE2="eval,style"
try_add add "Use tabs for indentation in eval tests" "$SCOPE2" "tabs"
try_add advisory "Use spaces for indentation in eval tests" "$SCOPE2" "spaces"

SCOPE3="eval,errors"
try_add add "Wrap eval errors with fmt.Errorf and %w" "$SCOPE3" "wrap"
try_add duplicate "Always wrap eval errors using %w verb" "$SCOPE3" "wrap2"

SCOPE4="eval,fmt"
try_add add "Run gofmt before eval commits" "$SCOPE4" "fmt"
try_add duplicate "Always run gofmt before you commit eval code" "$SCOPE4" "fmt2"

SCOPE5="eval,neutral"
try_add add "Keep eval functions under fifty lines" "$SCOPE5" "len"
try_add add "Deploy eval staging before production release" "$SCOPE5" "unrelated topic"

echo ""
echo "vault E2E: pass=$pass fail=$fail"
rm -rf "$EVAL_DIR"
[[ "$fail" -eq 0 ]]
