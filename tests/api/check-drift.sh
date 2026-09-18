#!/usr/bin/env bash
# Compare the pinned Nile API spec against the live spec served by the API.
# Usage: tests/api/check-drift.sh [live-spec-copy]   (or: make api-drift-check)
#
# Exits 0 when in sync, 1 on drift. Drift means the spec version changed OR
# the canonical JSON content changed under the same version. On drift the
# script prints a refresh hint; the optional first argument is a path where
# the downloaded live spec is kept (useful for CI artifacts / local diffing).
#
# Environment:
#   NILE_SPEC_URL  spec source (default https://global.thenile.dev/openapi.json)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LIVE_URL="${NILE_SPEC_URL:-https://global.thenile.dev/openapi.json}"
PINNED="${ROOT}/tests/spec/nile-openapi.json"
KEEP="${1:-}"

command -v curl >/dev/null || { echo "missing required command: curl" >&2; exit 1; }
command -v python3 >/dev/null || { echo "missing required command: python3" >&2; exit 1; }
[ -s "$PINNED" ] || { echo "missing pinned spec: $PINNED (run: make api-spec-fetch)" >&2; exit 1; }

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
curl -sfL "$LIVE_URL" -o "$tmp/live.json"
[ -s "$tmp/live.json" ] || { echo "downloaded live spec from $LIVE_URL is empty" >&2; exit 1; }

comparison="$(python3 - "$PINNED" "$tmp/live.json" <<'PY'
import hashlib, json, sys

def load(path):
    with open(path) as fh:
        return json.load(fh)

def digest(spec):
    # Canonical form (sorted keys, compact separators) so a purely cosmetic
    # re-serialization by the server does not count as drift.
    canon = json.dumps(spec, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(canon.encode()).hexdigest()

pinned, live = load(sys.argv[1]), load(sys.argv[2])
print(pinned["info"]["version"])
print(live["info"]["version"])
print(digest(pinned))
print(digest(live))
PY
)"

pinned_version="$(sed -n 1p <<<"$comparison")"
live_version="$(sed -n 2p <<<"$comparison")"
pinned_digest="$(sed -n 3p <<<"$comparison")"
live_digest="$(sed -n 4p <<<"$comparison")"

if [ -n "$KEEP" ]; then
  cp "$tmp/live.json" "$KEEP"
  echo "live spec saved to: $KEEP"
fi

if [ "$pinned_version" != "$live_version" ] || [ "$pinned_digest" != "$live_digest" ]; then
  echo "" >&2
  echo "NILE API SPEC DRIFT DETECTED" >&2
  echo "  pinned version: $pinned_version ($pinned_digest)" >&2
  echo "   live version: $live_version ($live_digest)" >&2
  echo "" >&2
  echo "The Nile API contract changed. Refresh the pin and review the impact:" >&2
  echo "  make api-spec-fetch   # update tests/spec/{nile-openapi.json,VERSION}" >&2
  echo "  git diff tests/spec/  # review what changed in the contract" >&2
  echo "then fix provider/mock/deviations if any and commit." >&2
  exit 1
fi

echo "Nile API spec in sync (version $pinned_version)"
