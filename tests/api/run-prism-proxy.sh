#!/usr/bin/env bash
# Start the stateful Nile mock API and front it with the Prism OpenAPI
# validation proxy (https://github.com/stoplightio/prism). Every request and
# response passing through the proxy is validated against the pinned spec
# (tests/spec/nile-openapi.json), so contract drift between the mock and the
# real Nile API shows up as Prism validation output while the mock keeps its
# full stateful lifecycle behavior (READY polling, 409 conflicts, retries).
#
# Usage:
#   tests/api/run-prism-proxy.sh                  # run proxy until Ctrl-C
#   tests/api/run-prism-proxy.sh --smoke [args]   # run tests/smoke/run.sh through it
#
# Send provider traffic to the proxy port; watch the prism log for
# validation warnings. Requests/responses that deviate from the spec are
# still forwarded (warn-only), so the mock's intended behavior is preserved.
#
# Environment:
#   NILE_MOCK_PORT  mock API port           (default 18080)
#   PRISM_PORT      validation proxy port   (default 18081)
#   PRISM_CLI       prism-cli npm specifier (default @stoplight/prism-cli@5)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MOCK_PORT="${NILE_MOCK_PORT:-18080}"
PRISM_PORT="${PRISM_PORT:-18081}"
PRISM_CLI="${PRISM_CLI:-@stoplight/prism-cli@5}"
SPEC="${ROOT}/tests/spec/nile-openapi.json"
MOCK_URL="http://127.0.0.1:${MOCK_PORT}"
PROXY_URL="http://127.0.0.1:${PRISM_PORT}"

for cmd in node npx python3 curl; do
  command -v "$cmd" >/dev/null || { echo "missing required command: $cmd" >&2; exit 1; }
done
[ -s "$SPEC" ] || { echo "missing pinned spec: $SPEC (run: make api-spec-fetch)" >&2; exit 1; }

TMP="$(mktemp -d)"
MOCK_PID=""
PRISM_PID=""
cleanup() {
  if [ -n "$PRISM_PID" ]; then kill "$PRISM_PID" 2>/dev/null || true; fi
  if [ -n "$MOCK_PID" ]; then kill "$MOCK_PID" 2>/dev/null || true; fi
  rm -rf "$TMP"
}
trap cleanup EXIT INT TERM

echo "==> starting mock API on 127.0.0.1:$MOCK_PORT"
python3 "$ROOT/tests/mockserver.py" "$MOCK_PORT" >"$TMP/mock.log" 2>&1 &
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
  echo "mock API did not start on port $MOCK_PORT:" >&2
  cat "$TMP/mock.log" >&2
  exit 1
fi

echo "==> starting Prism validation proxy ($PRISM_CLI)"
npx --yes "$PRISM_CLI" proxy "$SPEC" "$MOCK_URL" -p "$PRISM_PORT" >"$TMP/prism.log" 2>&1 &
PRISM_PID=$!
prism_ready=0
for _ in $(seq 1 150); do
  # Any HTTP answer (even 404 on /) proves the proxy is listening.
  code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 2 "$PROXY_URL/" || true)"
  if [ -n "$code" ] && [ "$code" != "000" ]; then
    prism_ready=1
    break
  fi
  if ! kill -0 "$PRISM_PID" 2>/dev/null; then
    break
  fi
  sleep 0.2
done
if [ "$prism_ready" -ne 1 ]; then
  echo "Prism proxy did not start on port $PRISM_PORT:" >&2
  cat "$TMP/prism.log" >&2
  exit 1
fi

echo "==> Prism proxy ready: $PROXY_URL (validates against $SPEC, forwards to $MOCK_URL)"
echo "    Prism log: $TMP/prism.log"

if [ "${1:-}" = "--smoke" ]; then
  shift
  # Exercise the provider's retry logic through the proxy as well: the mock
  # fails the very first request with a transient 503 (as tests/smoke/run.sh
  # would when it starts its own mock).
  export MOCK_FAIL_FIRST=1
  NILE_MOCK_URL="$PROXY_URL" "$ROOT/tests/smoke/run.sh" "$@"
else
  # Keep the proxy up until interrupted; stream validation output live.
  tail -f "$TMP/prism.log" &
  TAIL_PID=$!
  trap 'kill "$TAIL_PID" 2>/dev/null || true; cleanup' EXIT INT TERM
  wait "$PRISM_PID"
fi
