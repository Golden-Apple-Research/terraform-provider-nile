#!/usr/bin/env bash
# End-to-end smoke test: builds the provider, starts the mock Nile API and
# runs `terraform apply` against it in a throwaway working directory.
#
# Usage: tests/smoke/run.sh   (or: make smoke)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PORT="${NILE_MOCK_PORT:-18080}"
TOKEN="test-token-123"

for cmd in go terraform python3; do
  command -v "$cmd" >/dev/null || { echo "missing required command: $cmd" >&2; exit 1; }
done

TMP="$(mktemp -d)"
MOCK_PID=""
cleanup() {
  if [ -n "$MOCK_PID" ]; then kill "$MOCK_PID" 2>/dev/null || true; fi
  rm -rf "$TMP"
}
trap cleanup EXIT

# dev_overrides resolve the canonical, unversioned binary name in a dedicated dir.
mkdir -p "$TMP/plugins" "$TMP/work"
(cd "$ROOT" && go build -o "$TMP/plugins/terraform-provider-nile" .)

cat > "$TMP/terraformrc" <<EOF
provider_installation {
  dev_overrides {
    "golden-apple-research/nile" = "$TMP/plugins"
  }
  direct {}
}
EOF

python3 "$ROOT/tests/mockserver.py" "$PORT" >"$TMP/mock.log" 2>&1 &
MOCK_PID=$!

# Wait for the mock to signal readiness instead of sleeping for a fixed
# amount of time. This also fails fast, with the log attached, when the
# process dies (e.g. the port is already taken by a stale run).
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

cp "$ROOT/tests/smoke/main.tf" "$TMP/work/"
export TF_CLI_CONFIG_FILE="$TMP/terraformrc"
export NILE_API_TOKEN="$TOKEN"

if ! terraform -chdir="$TMP/work" apply -auto-approve -input=false -no-color \
  -var="nile_api_url=http://127.0.0.1:$PORT" >/dev/null; then
  echo "terraform apply failed; mock API log:" >&2
  cat "$TMP/mock.log" >&2
  exit 1
fi
terraform -chdir="$TMP/work" output -json > "$TMP/work/output.json"

python3 - "$TMP/work/output.json" <<'PY'
import json
import sys

with open(sys.argv[1]) as fh:
    out = json.load(fh)


def value(name):
    return out[name]["value"]


assert value("count") == 2, out
assert value("ids") == ["inst-abc123", "inst-def456"], out
assert value("names") == ["primary-compute", "old-compute"], out
assert value("statuses") == ["READY", "TERMINATED"], out
assert value("sizes") == ["large", "standard-4"], out
assert value("created_ats") == ["2025-06-01T12:00:00Z", "2025-07-15T08:30:00Z"], out
assert value("first_raw")["instanceId"] == "inst-abc123", out

print("smoke test passed: 2 instances, all typed fields mapped correctly")
PY
