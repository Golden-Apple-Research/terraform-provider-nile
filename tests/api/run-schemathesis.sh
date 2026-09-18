#!/usr/bin/env bash
# Run Schemathesis (https://schemathesis.readthedocs.io) against the stateful
# Nile mock API: property-based fuzz tests derived from the pinned OpenAPI
# spec, so deviations between the mock's behavior and the documented Nile
# contract fail loudly with a minimal reproducing case (curl command + diff).
#
# Usage: tests/api/run-schemathesis.sh [path-to-st]   (or: make api-schemathesis)
#
# The optional argument is the schemathesis CLI binary; the default "st" is
# resolved via PATH. CI installs a pinned version into a venv and passes the
# venv path; see .github/workflows/api-drift.yml.
#
# Environment:
#   NILE_MOCK_PORT   mock API port (default 18080)
#   ST_MAX_EXAMPLES  generated examples per operation (default 20)
#   ST_REPORT_DIR    if set, the JUnit report is copied here for inspection
#
# Deviations that are known and accepted are recorded in
# tests/api/schemathesis-baseline.txt; see tests/api/README.md. Only NEW
# deviations (or harness errors) fail this script.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PORT="${NILE_MOCK_PORT:-18080}"
ST_BIN="${1:-st}"
SPEC="${ROOT}/tests/spec/nile-openapi.json"
BASE_URL="http://127.0.0.1:${PORT}"

command -v python3 >/dev/null || { echo "missing required command: python3" >&2; exit 1; }
command -v "$ST_BIN" >/dev/null || {
  echo "schemathesis CLI '$ST_BIN' not found; install a pinned version, e.g.:" >&2
  echo "  python3 -m venv .venv && .venv/bin/pip install 'schemathesis==3.38.*' 'hypothesis<6.113'" >&2
  echo "  tests/api/run-schemathesis.sh .venv/bin/st" >&2
  exit 1
}
[ -s "$SPEC" ] || { echo "missing pinned spec: $SPEC (run: make api-spec-fetch)" >&2; exit 1; }

TMP="$(mktemp -d)"
MOCK_PID=""
cleanup() {
  if [ -n "$MOCK_PID" ]; then kill "$MOCK_PID" 2>/dev/null || true; fi
  rm -rf "$TMP"
}
trap cleanup EXIT INT TERM

echo "==> starting mock API on 127.0.0.1:$PORT"
python3 "$ROOT/tests/mockserver.py" "$PORT" >"$TMP/mock.log" 2>&1 &
MOCK_PID=$!
mock_ready=0
for _ in $(seq 1 100); do
  if grep -q "listening" "$TMP/mock.log" 2>/dev/null; then
    mock_ready=1
    break
  fi
  if ! kill -0 "$MOCK_PID" 2>/dev/null; then
    break
  fi
  sleep 0.1
done
if [ "$mock_ready" -ne 1 ]; then
  echo "mock API did not start on port $PORT:" >&2
  cat "$TMP/mock.log" >&2
  exit 1
fi

echo "==> running schemathesis ($("$ST_BIN" --version)) against $BASE_URL"
# The bearer token matches tests/mockserver.py (TOKEN); without it every
# request would only exercise the documented 401 path.
set +e
"$ST_BIN" run "$SPEC" \
  --base-url "$BASE_URL" \
  --set-header "Authorization=Bearer test-token-123" \
  --hypothesis-max-examples "${ST_MAX_EXAMPLES:-20}" \
  --checks all \
  --junit-xml "$TMP/st-report.xml"
status=$?
set -e

if [ -n "${ST_REPORT_DIR:-}" ]; then
  mkdir -p "$ST_REPORT_DIR"
  cp "$TMP/st-report.xml" "$ST_REPORT_DIR/st-report.xml"
  echo "JUnit report copied to: $ST_REPORT_DIR/st-report.xml"
fi

if [ "$status" -eq 0 ]; then
  echo "schemathesis: all contract checks passed"
  exit 0
fi

# Non-zero: genuine failures or harness errors. Classify via the baseline so
# known, accepted deviations keep the run green while new ones fail loudly.
if ! python3 "$ROOT/tests/api/check-schemathesis-baseline.py" \
  "$TMP/st-report.xml" "$ROOT/tests/api/schemathesis-baseline.txt"; then
  echo "" >&2
  echo "schemathesis found NEW contract deviations (not in tests/api/schemathesis-baseline.txt)." >&2
  echo "Either the mock deviates from the documented Nile contract (fix" >&2
  echo "tests/mockserver.py), or Nile changed its API (refresh the pinned spec" >&2
  echo "via: make api-spec-fetch), or a change is intentional: review and update" >&2
  echo "the baseline consciously (see tests/api/README.md)." >&2
  exit 1
fi
exit 0
