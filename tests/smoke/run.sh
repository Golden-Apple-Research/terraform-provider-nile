#!/usr/bin/env bash
# End-to-end smoke test: builds the provider, starts the mock Nile API and
# runs `terraform apply` against it in a throwaway working directory.
#
# Phases:
#   1. create all resources and read all data sources
#   2. resize the compute instance in place
#   3. rename the database (replacing its dependent compute instance)
#   4. destroy everything
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

# Make the mock answer the very first request with a transient 503 so the
# smoke test also exercises the provider's retry logic.
export MOCK_FAIL_FIRST=1
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

apply() {
  if ! terraform -chdir="$TMP/work" apply -auto-approve -input=false -no-color \
    -var="nile_api_url=http://127.0.0.1:$PORT" "$@" >/dev/null; then
    echo "terraform apply $* failed; mock API log:" >&2
    cat "$TMP/mock.log" >&2
    exit 1
  fi
}

dump_outputs() {
  terraform -chdir="$TMP/work" output -json > "$TMP/work/output.json"
}

# --- phase 1: create everything ----------------------------------------------
apply
dump_outputs
python3 - "$TMP/work/output.json" <<'PY'
import json, sys

with open(sys.argv[1]) as fh:
    out = json.load(fh)

def value(name):
    return out[name]["value"]

# Existing paginated compute instance data source.
assert value("compute_instance_count") == 2, out
assert value("compute_instance_ids") == ["inst-abc123", "inst-def456"], out
assert value("compute_instance_names") == ["primary-compute", "old-compute"], out
assert value("compute_instance_statuses") == ["READY", "TERMINATED"], out
assert value("compute_instance_sizes") == ["large", "standard-4"], out
assert value("compute_instance_created_ats") == ["2025-06-01T12:00:00Z", "2025-07-15T08:30:00Z"], out
assert value("first_raw")["instanceId"] == "inst-abc123", out

# Resources.
assert value("database_name") == "app-database", out
assert value("database_status") == "READY", out
assert value("database_region") == "AWS_US_WEST_2", out
assert value("instance_id") == "inst-1-app-database", out
assert value("instance_status") == "READY", out
assert value("instance_size") == "large", out
assert value("instance_memory") == "8GB", out
assert value("instance_hourly_cost") == 0.42, out
assert value("credential_id") == "cred-1", out
assert value("credential_password") == "password-cred-1", out
assert value("invite_id") == "inv-1", out
assert value("invite_code") == "code-inv-1", out
assert value("invite_state") == "EMAIL_PENDING", out

# Data sources. The database list depends on the managed database, so it
# reflects the database created in this apply; the invite list is static and
# is read before the invite resource exists.
assert value("databases_count") == 2, out
assert value("databases_names") == ["test-database", "app-database"], out
assert value("looked_up_database_status") == "READY", out
assert value("credentials_count") == 1, out
assert value("regions") == ["AWS_EU_CENTRAL_1", "AWS_US_WEST_2", "AZURE_EASTUS"], out
assert value("compute_types") == ["large", "xlarge"], out
assert value("workspace_slug") == "test-workspace", out
assert value("workspaces_count") == 1, out
assert value("developers_count") == 1, out
assert value("invites_count") == 0, out
assert value("subscription_level") == "paid", out
assert value("subscription_history_count") == 1, out
assert value("compute_usage_vcpu") == 12.5, out
assert value("uptime_percentage") == 99.9, out
assert value("error_count") == 3, out
assert value("p99_latency") == 10.0, out
assert value("billing_status") == "ready", out
assert value("billing_compute") == 1.5, out
assert value("developer_email") == "dev@example.com", out

print("phase 1 passed: resources created, typed fields and data sources mapped correctly")
PY

# --- phase 2: resize the compute instance in place ---------------------------
apply -var="instance_size=xlarge"
dump_outputs
python3 - "$TMP/work/output.json" <<'PY'
import json, sys

with open(sys.argv[1]) as fh:
    out = json.load(fh)

def value(name):
    return out[name]["value"]

assert value("instance_size") == "xlarge", out
assert value("instance_id") == "inst-1-app-database", out
assert value("instance_status") == "READY", out

# The database resource from phase 1 now shows up in the list data source
# (the API returns the seeded database first).
assert value("databases_count") == 2, out
assert value("databases_names") == ["test-database", "app-database"], out
assert value("invites_count") == 1, out

print("phase 2 passed: compute instance resized in place")
PY

# --- phase 3: rename the database --------------------------------------------
apply -var="database_name=renamed-database" -var="instance_size=xlarge"
dump_outputs
python3 - "$TMP/work/output.json" <<'PY'
import json, sys

with open(sys.argv[1]) as fh:
    out = json.load(fh)

def value(name):
    return out[name]["value"]

assert value("database_name") == "renamed-database", out
assert value("database_status") == "READY", out
assert value("instance_id") == "inst-1-renamed-database", out
assert value("instance_size") == "xlarge", out
assert value("databases_names") == ["test-database", "renamed-database"], out

print("phase 3 passed: database renamed and dependent instance recreated")
PY

# --- phase 4: destroy everything ---------------------------------------------
if ! terraform -chdir="$TMP/work" destroy -auto-approve -input=false -no-color \
  -var="nile_api_url=http://127.0.0.1:$PORT" >/dev/null; then
  echo "terraform destroy failed; mock API log:" >&2
  cat "$TMP/mock.log" >&2
  exit 1
fi

echo "smoke test passed"
