#!/usr/bin/env bash
# Download the live Nile control-plane OpenAPI spec and pin it in-repo.
# Usage: tests/api/fetch-spec.sh   (or: make api-spec-fetch)
#
# The spec is committed under tests/spec/ so tests are deterministic (no
# network) and the drift check in CI has a fixed reference. After an
# intentional refresh, review the spec diff for contract changes that affect
# the provider or the mock, then commit both files together:
#   tests/spec/nile-openapi.json
#   tests/spec/VERSION
#
# Environment:
#   NILE_SPEC_URL  spec source (default https://global.thenile.dev/openapi.json)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SPEC_URL="${NILE_SPEC_URL:-https://global.thenile.dev/openapi.json}"
OUT="${ROOT}/tests/spec/nile-openapi.json"

command -v curl >/dev/null || { echo "missing required command: curl" >&2; exit 1; }
command -v python3 >/dev/null || { echo "missing required command: python3" >&2; exit 1; }

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
curl -sfL "$SPEC_URL" -o "$tmp"
[ -s "$tmp" ] || { echo "downloaded spec from $SPEC_URL is empty" >&2; exit 1; }

version="$(python3 - "$tmp" <<'PY'
import json, sys
try:
    spec = json.load(open(sys.argv[1]))
    print(spec["info"]["version"])
except Exception as exc:  # not a parseable OpenAPI document
    print(f"downloaded spec is not valid JSON with info.version: {exc}", file=sys.stderr)
    sys.exit(1)
PY
)"

printf '%s\n' "$version" > "${ROOT}/tests/spec/VERSION"
mv "$tmp" "$OUT"
trap - EXIT
echo "pinned Nile API spec: tests/spec/nile-openapi.json (version $version)"
