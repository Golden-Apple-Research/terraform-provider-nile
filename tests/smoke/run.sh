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

START="$(date +%s)"
TMP="$(mktemp -d)"
CHECKS="$TMP/checks.log"
: >"$CHECKS"
MOCK_PID=""
cleanup() {
  if [ -n "$MOCK_PID" ]; then kill "$MOCK_PID" 2>/dev/null || true; fi
  rm -rf "$TMP"
}
trap cleanup EXIT

# dev_overrides resolve the canonical, unversioned binary name in a dedicated dir.
mkdir -p "$TMP/plugins" "$TMP/work"
echo "==> building provider"
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
echo "==> mock API ready on 127.0.0.1:$PORT (first request will fail with 503 to exercise retries)"

cp "$ROOT/tests/smoke/main.tf" "$TMP/work/"
export TF_CLI_CONFIG_FILE="$TMP/terraformrc"
export NILE_API_TOKEN="$TOKEN"

apply() {
  if ! terraform -chdir="$TMP/work" apply -auto-approve -input=false -no-color \
    -var="nile_api_url=http://127.0.0.1:$PORT" "$@" \
    | tee "$TMP/apply.log" >/dev/null; then
    echo "terraform apply $* failed; mock API log:" >&2
    cat "$TMP/mock.log" >&2
    exit 1
  fi
  grep -q "Apply complete" "$TMP/apply.log" || {
    echo "terraform apply $* produced no summary; mock API log:" >&2
    cat "$TMP/mock.log" >&2
    exit 1
  }
}

# apply_inplace <vars...>: like apply, but additionally insists that the run
# destroyed nothing (0 destroyed). In-place phases must never replace
# resources: a regression here once turned credential rotation into a silent
# destroy/recreate cycle that looked identical in the outputs.
apply_inplace() {
  apply "$@"
  destroyed=$(sed -n 's/.*Apply complete! Resources: \([0-9]*\) added, \([0-9]*\) changed, \([0-9]*\) destroyed.*/\3/p' "$TMP/apply.log")
  [ "$destroyed" = "0" ] || {
    echo "in-place apply destroyed $destroyed resource(s):" >&2
    grep -E 'destroy|Destroy' "$TMP/apply.log" >&2 || true
    exit 1
  }
}

dump_outputs() {
  terraform -chdir="$TMP/work" output -json > "$TMP/work/output.json"
}

# --- phase 1: create everything ----------------------------------------------
echo "==> phase 1/4: create resources, read data sources"
apply_inplace
dump_outputs
python3 - "$TMP/work/output.json" "$CHECKS" <<'PY'
import json, sys

with open(sys.argv[1]) as fh:
    out = json.load(fh)

passed = 0

def value(name):
    return out[name]["value"]

def check(label, got, want):
    global passed
    if got != want:
        print(f"  FAIL {label}\n       got:  {got!r}\n       want: {want!r}", file=sys.stderr)
        sys.exit(1)
    print(f"  ok   {label}: {got!r}")
    passed += 1
    with open(sys.argv[2], "a") as fh:
        fh.write(label + "\n")

print("  -- existing compute instances (paginated list, two pages)")
check("instance count", value("compute_instance_count"), 2)
check("instance ids", value("compute_instance_ids"), ["inst-abc123", "inst-def456"])
check("instance names", value("compute_instance_names"), ["primary-compute", "old-compute"])
check("instance statuses", value("compute_instance_statuses"), ["READY", "TERMINATED"])
check("instance sizes", value("compute_instance_sizes"), ["large", "standard-4"])
check("instance created_at", value("compute_instance_created_ats"), ["2025-06-01T12:00:00Z", "2025-07-15T08:30:00Z"])
check("raw payload passthrough", value("first_raw")["instanceId"], "inst-abc123")

print("  -- resources created")
check("database name", value("database_name"), "app_database")
check("database status", value("database_status"), "READY")
check("database region", value("database_region"), "AWS_US_WEST_2")
check("compute instance id", value("instance_id"), "inst-1-app_database")
check("compute instance status", value("instance_status"), "READY")
check("compute instance size", value("instance_size"), "large")
check("compute instance memory", value("instance_memory"), "8GB")
check("compute instance hourly cost", value("instance_hourly_cost"), 0.42)
check("credential id", value("credential_id"), "cred-1")
check("credential password (one-time secret)", value("credential_password"), "password-cred-1")
check("invite id", value("invite_id"), "inv-1")
check("invite code (programmatic)", value("invite_code"), "code-inv-1")
check("invite verification state", value("invite_state"), "EMAIL_PENDING")

print("  -- data sources read")
# The database list depends on the managed database, so it reflects the
# database created in this apply; the invite list is read before the invite
# resource exists.
check("databases count", value("databases_count"), 3)
check("databases names", sorted(value("databases_names")), sorted(["test_database", "app_database", "unauth_mock_1"]))
check("database lookup status", value("looked_up_database_status"), "READY")
check("credentials count", value("credentials_count"), 1)
check("regions (sorted)", value("regions"), ["AWS_EU_CENTRAL_1", "AWS_US_WEST_2", "AZURE_EASTUS"])
check("compute type sizes", value("compute_types"), ["large", "xlarge"])
check("workspace slug", value("workspace_slug"), "test-workspace")
check("workspaces count", value("workspaces_count"), 2)
check("workspace developers count", value("developers_count"), 1)
check("workspace invites count", value("invites_count"), 0)
check("subscription level", value("subscription_level"), "paid")
check("subscription history entries", value("subscription_history_count"), 1)
check("compute usage total vCPU hours", value("compute_usage_vcpu"), 12.5)
check("uptime insights percentage", value("uptime_percentage"), 99.9)
check("error insights count", value("error_count"), 3)
check("query performance p99 latency", value("p99_latency"), 10.0)
check("billing readiness status", value("billing_status"), "ready")
check("billing totals compute", value("billing_compute"), 1.5)
check("developer email", value("developer_email"), "dev@example.com")

print("  -- new lifecycle resources")
check("workspace resource slug", value("workspace_extra_slug"), "smoke-extra")
check("workspace resource id", value("workspace_extra_id"), "ws-2")
check("billing customer ensured", value("billing_customer_id"), "cus_test")
check("subscription started", value("subscription_resource_level"), "paid")
check("subscription resource id", value("subscription_resource_id"), "sub-2")
check("dedicated database provisioned", value("provisioned_claim_code"), "claim-1")
check("provisioned database name", value("provisioned_database_name"), "unauth_mock_1")
check("provisioned database claimed", value("claimed_database_name"), "unauth_mock_1")

print(f"phase 1 passed: {passed} checks")
PY

# --- phase 2: resize the compute instance in place ---------------------------
echo "==> phase 2/4: resize compute instance, rotate credential, change subscription"
apply_inplace -var="instance_size=xlarge" -var="credential_rotation=rotated" -var="subscription_level=enterprise"
dump_outputs
python3 - "$TMP/work/output.json" "$CHECKS" <<'PY'
import json, sys

with open(sys.argv[1]) as fh:
    out = json.load(fh)

passed = 0

def value(name):
    return out[name]["value"]

def check(label, got, want):
    global passed
    if got != want:
        print(f"  FAIL {label}\n       got:  {got!r}\n       want: {want!r}", file=sys.stderr)
        sys.exit(1)
    print(f"  ok   {label}: {got!r}")
    passed += 1
    with open(sys.argv[2], "a") as fh:
        fh.write(label + "\n")

check("instance resized", value("instance_size"), "xlarge")
check("instance kept its id (in-place update)", value("instance_id"), "inst-1-app_database")
check("instance still READY", value("instance_status"), "READY")

# The database resource from phase 1 now shows up in the list data source
# (the API returns the seeded database first).
check("databases count", value("databases_count"), 3)
check("databases names", sorted(value("databases_names")), sorted(["test_database", "app_database", "unauth_mock_1"]))
check("invites count (resource now visible)", value("invites_count"), 1)

print("  -- in-place lifecycle updates")
check("credential rotated", value("credential_id"), "cred-2")
check("password rotated", value("credential_password"), "password-cred-2")
check("subscription changed", value("subscription_resource_level"), "enterprise")
check("seeded subscription untouched", value("subscription_level"), "paid")

print(f"phase 2 passed: {passed} checks")
PY

# --- phase 3: rename the database --------------------------------------------
echo "==> phase 3/4: rename database, recreate dependent instance"
apply -var="database_name=renamed_database" -var="instance_size=xlarge" \
  -var="credential_rotation=rotated" -var="subscription_level=enterprise"
dump_outputs
python3 - "$TMP/work/output.json" "$CHECKS" <<'PY'
import json, sys

with open(sys.argv[1]) as fh:
    out = json.load(fh)

passed = 0

def value(name):
    return out[name]["value"]

def check(label, got, want):
    global passed
    if got != want:
        print(f"  FAIL {label}\n       got:  {got!r}\n       want: {want!r}", file=sys.stderr)
        sys.exit(1)
    print(f"  ok   {label}: {got!r}")
    passed += 1
    with open(sys.argv[2], "a") as fh:
        fh.write(label + "\n")

check("database renamed", value("database_name"), "renamed_database")
check("database still READY", value("database_status"), "READY")
check("dependent instance recreated", value("instance_id"), "inst-1-renamed_database")
check("instance size preserved", value("instance_size"), "xlarge")
check("databases names after rename", sorted(value("databases_names")), sorted(["test_database", "renamed_database", "unauth_mock_1"]))
check("claimed database unaffected by rename", value("claimed_database_name"), "unauth_mock_1")
# The rename replaces the credential (database_name forces replacement): the
# replacement is created fresh, so the id advances past the rotated one.
check("credential recreated after rename", value("credential_id"), "cred-3")
check("recreated credential password", value("credential_password"), "password-cred-3")

print(f"phase 3 passed: {passed} checks")
PY

# --- phase 4: destroy everything ---------------------------------------------
echo "==> phase 4/4: destroy everything"
if ! terraform -chdir="$TMP/work" destroy -auto-approve -input=false -no-color \
  -var="nile_api_url=http://127.0.0.1:$PORT" >/dev/null; then
  echo "terraform destroy failed; mock API log:" >&2
  cat "$TMP/mock.log" >&2
  exit 1
fi
state="$(terraform -chdir="$TMP/work" state list)"
if [ -n "$state" ]; then
  echo "  FAIL terraform state not empty after destroy:" >&2
  echo "$state" >&2
  exit 1
fi
echo "  ok   terraform state empty after destroy"
echo "state empty after destroy" >>"$CHECKS"
echo "phase 4 passed: destroy"

END="$(date +%s)"
TOTAL="$(grep -c . "$CHECKS")"
echo "smoke test passed: $TOTAL checks in 4 phases ($((END - START))s)"
