#!/usr/bin/env python3
"""Run imprint add() on all calibration pairs against an isolated vault + live sidecar."""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import tempfile
import textwrap
import urllib.request

# Import labeled pairs from sibling sidecar clone.
SIDECAR = os.path.normpath(
    os.path.join(os.path.dirname(__file__), "..", "..", "imprint-embed-sidecar")
)
sys.path.insert(0, SIDECAR)
from samples import CONFLICT, DUPLICATE  # noqa: E402

IMPRINT = os.environ.get("IMPRINT_BIN", "imprint")
EMBED_URL = os.environ.get("IMPRINT_EMBED_URL", "http://127.0.0.1:4174")


def health_ready() -> bool:
    try:
        with urllib.request.urlopen(f"{EMBED_URL}/health", timeout=3) as r:
            return b'"ready": true' in r.read()
    except OSError:
        return False


def imprint_add(vault: str, workdir: str, claim: str, scope: str, text: str) -> dict:
    cmd = [
        IMPRINT,
        "--vault",
        vault,
        "--json",
        "add",
        claim,
        "--scope",
        scope,
        "--text",
        text,
    ]
    proc = subprocess.run(cmd, cwd=workdir, capture_output=True, text=True)
    raw = (proc.stdout or proc.stderr or "").strip()
    try:
        return json.loads(raw.splitlines()[-1])
    except (json.JSONDecodeError, IndexError):
        return {"_error": raw, "_code": proc.returncode}


def main() -> int:
    if not health_ready():
        print("sidecar not ready on 4174", file=sys.stderr)
        return 1

    workdir = tempfile.mkdtemp(prefix="imprint-embed-batch-")
    vault = os.path.join(workdir, "vault")
    sidecar_pkg = SIDECAR if os.path.isdir(SIDECAR) else "../imprint-embed-sidecar"
    with open(os.path.join(workdir, "imprint.yaml"), "w", encoding="utf-8") as f:
        f.write(
            textwrap.dedent(
                f"""\
                host:
                  listen: 127.0.0.1:9470
                plugins:
                  embed:
                    enabled: true
                    package: {sidecar_pkg}
                    config:
                      port: 4174
                      model: BAAI/bge-small-zh-v1.5
                      relate_model: cross-encoder/nli-deberta-v3-base
                      duplicate_threshold: 0.65
                      relate_contradiction_min: 0.40
                      relate_entailment_min: 0.35
                      timeout_seconds: 8
                """
            )
        )

    dup_ok = dup_miss = con_ok = con_bad = 0
    misses: list[str] = []

    print(f"vault={vault}\nimprint={IMPRINT}\n")

    for i, (a, b) in enumerate(DUPLICATE):
        scope = f"pair-dup-{i:02d}"
        r1 = imprint_add(vault, workdir, a, scope, "seed")
        if "id" not in r1:
            misses.append(f"dup#{i} seed failed: {r1}")
            dup_miss += 1
            continue
        r2 = imprint_add(vault, workdir, b, scope, "paraphrase")
        if r2.get("code") == "duplicate":
            dup_ok += 1
        else:
            dup_miss += 1
            misses.append(f"dup#{i} expected duplicate, got: {r2.get('id', r2)}")

    for i, (a, b) in enumerate(CONFLICT):
        scope = f"pair-con-{i:02d}"
        r1 = imprint_add(vault, workdir, a, scope, "seed")
        if "id" not in r1:
            misses.append(f"con#{i} seed failed: {r1}")
            con_bad += 1
            continue
        r2 = imprint_add(vault, workdir, b, scope, "opposite")
        if r2.get("code") == "duplicate":
            con_bad += 1
            misses.append(f"con#{i} false duplicate block")
        else:
            con_ok += 1
            sim = len(r2.get("similar") or [])
            if sim:
                misses.append(f"con#{i} note: add with similar={sim} (expected)")

    print("=== duplicate pairs (20): second add should duplicate ===")
    print(f"  block={dup_ok}/20  miss={dup_miss}/20  recall={dup_ok / 20:.0%}")
    print("=== conflict pairs (20): second add must NOT duplicate ===")
    print(f"  ok={con_ok}/20  false_block={con_bad}/20")

    if misses:
        print(f"\ndetails ({len(misses)} lines):")
        for line in misses:
            print(f"  {line}")

    shutil.rmtree(workdir, ignore_errors=True)
    return 0 if dup_miss == 0 and con_bad == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
