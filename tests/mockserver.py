#!/usr/bin/env python3
"""Stateful mock of the Nile control plane API.

It implements every endpoint the provider uses, so `tests/smoke/run.sh` can
exercise the full lifecycle (create, read, rename, resize, delete) and all
data sources with `terraform apply`/`destroy`.

The payload shapes mirror https://www.thenile.dev/docs/api-reference.

Beyond the documented shape, the mock exercises two provider behaviours end
to end:

- pagination: it splits the compute instances of the seeded database into two
  pages served as wrapped objects with a nextPageToken (the provider must
  concatenate them), and
- retries: with MOCK_FAIL_FIRST=1 the very first request answers 503 with
  Retry-After: 0 (the provider must retry and succeed).
"""
import json
import os
import sys
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import parse_qs, urlparse, unquote

TOKEN = "test-token-123"
WORKSPACE = {
    "id": "ws-1",
    "name": "Test Workspace",
    "slug": "test-workspace",
    "stripe_customer_id": "cus_test",
    "created": "2025-01-01T00:00:00Z",
}

# Seed data for the paginated compute list data source.
SEEDED_INSTANCES = [
    {
        "instanceId": "inst-abc123",
        "instanceName": "primary-compute",
        "instanceType": {
            "id": "tier-standard-2",
            "computeSize": "large",
            "memory": "8GB",
            "hourlyCost": 0.42,
        },
        "desiredInstanceType": {
            "id": "tier-standard-2",
            "computeSize": "large",
            "memory": "8GB",
            "hourlyCost": 0.42,
        },
        "status": "READY",
        "workspace": WORKSPACE,
        "database": {"id": "db-seed", "name": "test-database", "status": "READY", "region": "AWS_US_WEST_2"},
        "region": "AWS_US_WEST_2",
        "created": "2025-06-01T12:00:00Z",
        "updated": "2025-06-02T12:00:00Z",
    },
    {
        "instanceId": "inst-def456",
        "instanceName": "old-compute",
        "instanceType": {
            "id": "tier-standard-4",
            "computeSize": "standard-4",
            "memory": "16GB",
            "hourlyCost": 0.84,
        },
        "status": "TERMINATED",
        "region": "AWS_EU_CENTRAL_1",
        "created": "2025-07-15T08:30:00Z",
    },
]


def seeded_database():
    return {
        "id": "db-seed",
        "name": "test-database",
        "status": "READY",
        "region": "AWS_US_WEST_2",
        "expandable": True,
        "created": "2025-01-02T00:00:00Z",
        "apiHost": "test-database.api.thenile.dev",
        "dbHost": "test-database.db.thenile.dev",
        "workspace": WORKSPACE,
    }


# Mutable server state.
STATE = {
    "databases": {"test-database": seeded_database()},
    "next_database": 1,
    "computes": {"test-database": {i["instanceId"]: i for i in SEEDED_INSTANCES}},
    "credentials": {},
    "next_credential": 1,
    "invites": {},
    "next_invite": 1,
}


def error_payload(code, message, status):
    return {"errorCode": code, "message": message, "statusCode": status}


class Handler(BaseHTTPRequestHandler):
    failed_once = False

    def _send(self, status, payload, extra_headers=None):
        body = json.dumps(payload).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        for key, value in (extra_headers or {}).items():
            self.send_header(key, value)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _read_body(self):
        length = int(self.headers.get("Content-Length") or 0)
        if not length:
            return {}
        try:
            return json.loads(self.rfile.read(length))
        except json.JSONDecodeError:
            return {}

    def _segments(self):
        return [unquote(part) for part in urlparse(self.path).path.strip("/").split("/")]

    def do_GET(self):
        self._handle("GET")

    def do_POST(self):
        self._handle("POST")

    def do_PUT(self):
        self._handle("PUT")

    def do_DELETE(self):
        self._handle("DELETE")

    def _handle(self, method):
        if self.headers.get("Authorization") != "Bearer " + TOKEN:
            self._send(401, error_payload("invalid_credentials", "bad token", 401))
            return
        if os.environ.get("MOCK_FAIL_FIRST") and not Handler.failed_once:
            Handler.failed_once = True
            self._send(
                503,
                error_payload("internal_error", "transient failure (simulated)", 503),
                {"Retry-After": "0"},
            )
            return

        segments = self._segments()
        query = parse_qs(urlparse(self.path).query)
        try:
            handled = self._route(method, segments, query)
        except KeyError as exc:
            self._send(404, error_payload("entity_not_found", f"unknown route {exc}", 404))
            return
        if not handled:
            self._send(404, error_payload("entity_not_found", f"no handler for {method} {'/'.join(segments)}", 404))

    def _route(self, method, segments, query):
        # /developers/me
        if segments == ["developers", "me"] and method == "GET":
            self._send(200, {
                "id": "dev-1",
                "email": "dev@example.com",
                "kind": "HUMAN",
                "workspaces": [WORKSPACE],
                "databases": [seeded_database()],
            })
            return True

        # /workspaces
        if segments == ["workspaces"] and method == "GET":
            self._send(200, [WORKSPACE])
            return True

        if len(segments) >= 2 and segments[0] == "workspaces":
            workspace_slug = segments[1]
            rest = segments[2:]
            return self._route_workspace(method, workspace_slug, rest, query)
        return False

    def _route_workspace(self, method, workspace_slug, rest, query):
        # /workspaces/{slug}
        if not rest and method == "GET":
            if workspace_slug != WORKSPACE["slug"]:
                self._send(404, error_payload("entity_not_found", "no such workspace", 404))
                return True
            self._send(200, [WORKSPACE])
            return True

        if not rest:
            return False

        head, tail = rest[0], rest[1:]

        if head == "databases":
            return self._route_databases(method, workspace_slug, tail, query)
        if head == "regions" and not tail and method == "GET":
            self._send(200, ["AWS_US_WEST_2", "AWS_EU_CENTRAL_1", "AZURE_EASTUS"])
            return True
        if head == "compute-types" and not tail and method == "GET":
            self._send(200, [
                {"id": "tier-standard-2", "computeSize": "large", "memory": "8GB", "hourlyCost": 0.42},
                {"id": "tier-standard-4", "computeSize": "xlarge", "memory": "16GB", "hourlyCost": 0.84},
            ])
            return True
        if head == "developers" and method == "GET":
            self._send(200, [{"id": "dev-1", "email": "dev@example.com", "kind": "HUMAN", "workspaces": [WORKSPACE]}])
            return True
        if head == "developers" and method == "DELETE":
            self._send(204, {})
            return True
        if head == "invites":
            return self._route_invites(method, tail)
        if head == "metrics" and tail == ["compute"] and method == "GET":
            self._send(200, [{
                "totalVCPUHours": 12.5,
                "start": "2025-06-01T00:00:00Z",
                "end": "2025-06-02T00:00:00Z",
                "chartData": {"points": [{"x": 1, "y": 2}], "maxCPUCount": 4},
                "usageByDatabase": {
                    "test-database": {
                        "totalVCPUHours": 12.5,
                        "start": "2025-06-01T00:00:00Z",
                        "end": "2025-06-02T00:00:00Z",
                        "usageByInstance": {
                            "inst-abc123": {
                                "totalVCPUHours": 12.5,
                                "size": "large",
                                "start": "2025-06-01T00:00:00Z",
                                "end": "2025-06-02T00:00:00Z",
                            }
                        },
                    }
                },
            }])
            return True
        if head == "subscription":
            return self._route_subscription(method, tail)
        if head == "billing":
            return self._route_billing(method, tail)
        return False

    def _route_databases(self, method, workspace_slug, rest, query):
        if not rest and method == "GET":
            self._send(200, list(STATE["databases"].values()))
            return True
        if not rest and method == "POST":
            body = self._read_body()
            name = body.get("databaseName", "")
            if not name or name in STATE["databases"]:
                self._send(409, error_payload("duplicate_entity", "database exists", 409))
                return True
            database = {
                "id": f"db-{STATE['next_database']}",
                "name": name,
                "status": "READY",
                "region": body.get("region", "AWS_US_WEST_2"),
                "expandable": True,
                "created": "2025-08-01T00:00:00Z",
                "apiHost": f"{name}.api.thenile.dev",
                "dbHost": f"{name}.db.thenile.dev",
                "workspace": WORKSPACE,
            }
            STATE["next_database"] += 1
            STATE["databases"][name] = database
            self._send(201, database)
            return True

        name = rest[0]

        if name not in STATE["databases"]:
            self._send(404, error_payload("entity_not_found", "no such database", 404))
            return True

        if len(rest) == 1 and method == "GET":
            self._send(200, STATE["databases"][name])
            return True
        if len(rest) == 1 and method == "PUT":
            new_name = self._read_body().get("name", "")
            if not new_name:
                self._send(400, error_payload("bad_request", "name is required", 400))
                return True
            database = STATE["databases"].pop(name)
            database["name"] = new_name
            STATE["databases"][new_name] = database
            if name in STATE["computes"]:
                STATE["computes"][new_name] = STATE["computes"].pop(name)
            self._send(200, database)
            return True
        if len(rest) == 1 and method == "DELETE":
            database = STATE["databases"].pop(name)
            STATE["computes"].pop(name, None)
            self._send(200, database)
            return True

        if rest[1] == "compute":
            return self._route_compute(method, name, rest[2:], query)
        if rest[1] == "credentials":
            return self._route_credentials(method, name, rest[2:], query)
        if rest[1] == "insights":
            return self._route_insights(method, rest[2:], name)
        return False

    def _route_compute(self, method, database_name, rest, query):
        instances = STATE["computes"].setdefault(database_name, {})

        if not rest and method == "GET":
            values = list(instances.values())
            if database_name == "test-database":
                # Exercise pagination: first page is a wrapped object with a
                # continuation token, the second page a bare array.
                token = (query.get("pageToken") or [""])[0]
                if token == "":
                    self._send(200, {"instances": values[:1], "nextPageToken": "page-2"})
                elif token == "page-2":
                    self._send(200, values[1:])
                else:
                    self._send(400, error_payload("bad_request", "unknown page token", 400))
                return True
            self._send(200, values)
            return True
        if not rest and method == "POST":
            body = self._read_body()
            instance_id = f"inst-{len(instances) + 1}-{database_name}"
            instance = {
                "instanceId": instance_id,
                "instanceName": body.get("instanceName", ""),
                "instanceType": {
                    "id": "tier-standard-2",
                    "computeSize": body.get("instanceSize", "large"),
                    "memory": "8GB",
                    "hourlyCost": 0.42,
                },
                "status": "READY",
                "region": "AWS_US_WEST_2",
                "workspace": WORKSPACE,
                "database": STATE["databases"].get(database_name, {}),
                "created": "2025-08-01T00:00:00Z",
                "updated": "2025-08-01T00:00:00Z",
            }
            instances[instance_id] = instance
            self._send(202, instance)
            return True

        instance_id = rest[0]
        if instance_id not in instances:
            self._send(404, error_payload("entity_not_found", "no such instance", 404))
            return True
        if method == "GET":
            self._send(200, [instances[instance_id]])
            return True
        if method == "PUT":
            body = self._read_body()
            instance = instances[instance_id]
            if body.get("instanceName"):
                instance["instanceName"] = body["instanceName"]
            if body.get("instanceSize"):
                instance["instanceType"]["computeSize"] = body["instanceSize"]
            instance["updated"] = "2025-08-02T00:00:00Z"
            self._send(202, STATE["databases"].get(database_name, {}))
            return True
        if method == "DELETE":
            del instances[instance_id]
            self._send(202, STATE["databases"].get(database_name, {}))
            return True
        return False

    def _route_credentials(self, method, database_name, rest, query):
        credentials = STATE["credentials"].setdefault(database_name, {})
        tenant = (query.get("tenantId") or [""])[0]

        if not rest and method == "GET":
            values = [c for c in credentials.values() if not tenant or c.get("tenant") == tenant]
            self._send(200, values)
            return True
        if not rest and method == "POST":
            credential_id = f"cred-{STATE['next_credential']}"
            STATE["next_credential"] += 1
            credential = {
                "id": credential_id,
                "tenant": tenant,
                "internal": query.get("internal", ["false"])[0] == "true",
                "created": "2025-08-01T00:00:00Z",
                "password": f"password-{credential_id}",
                "database": STATE["databases"].get(database_name, {}),
            }
            credentials[credential_id] = credential
            self._send(200, credential)
            return True

        credential_id = rest[0]
        if credential_id not in credentials:
            self._send(404, error_payload("entity_not_found", "no such credential", 404))
            return True
        if method == "DELETE":
            del credentials[credential_id]
            self._send(204, {})
            return True
        return False

    def _route_insights(self, method, rest, database_name):
        if method != "GET" or len(rest) != 1:
            return False
        kind = rest[0]
        if kind == "uptime":
            self._send(200, {
                "source": "probe",
                "scope": "database",
                "granularity": "1h",
                "calculation": "time-weighted",
                "summary": {"uptimePercentage": 99.9, "uptimeSeconds": 3599, "observedSeconds": 3600},
                "points": [{
                    "timestamp": "2025-06-01T00:00:00Z",
                    "uptimePercentage": 99.5,
                    "uptimeSeconds": 3590,
                    "observedSeconds": 3600,
                }],
            })
            return True
        if kind == "errors":
            self._send(200, {
                "granularity": "5m",
                "points": [{"timestamp": "2025-06-01T00:00:00Z", "source": "proxy", "errorCount": 3}],
            })
            return True
        if kind == "query-performance":
            self._send(200, {
                "source": "proxy",
                "granularity": "1m",
                "points": [{
                    "timestamp": "2025-06-01T00:00:00Z",
                    "proxyQueriesPerSecond": 1.5,
                    "thothQueriesPerSecond": 1.2,
                    "thothP99LatencyMs": 10.0,
                    "thothCpuMilliseconds": 50.0,
                }],
            })
            return True
        return False

    def _route_invites(self, method, rest):
        invites = STATE["invites"].setdefault(WORKSPACE["slug"], {})

        if not rest and method == "GET":
            self._send(200, list(invites.values()))
            return True
        if not rest and method == "POST":
            body = self._read_body()
            email = body.get("email", "")
            invite_id = f"inv-{STATE['next_invite']}"
            STATE["next_invite"] += 1
            invite = {
                "id": invite_id,
                "email": email,
                "verificationState": "EMAIL_PENDING",
                "sender": {"id": "dev-1", "email": "dev@example.com", "kind": "HUMAN"},
                "workspace": WORKSPACE,
                "created": "2025-08-01T00:00:00Z",
                "updated": "2025-08-01T00:00:00Z",
            }
            if body.get("programmatic"):
                invite["code"] = f"code-{invite_id}"
            invites[invite_id] = invite
            self._send(201, invite)
            return True

        invite_id = rest[0]
        if invite_id not in invites:
            self._send(404, error_payload("entity_not_found", "no such invite", 404))
            return True
        if method == "DELETE":
            del invites[invite_id]
            self._send(204, {})
            return True
        return False

    def _route_subscription(self, method, rest):
        subscription = {
            "workspace": WORKSPACE["slug"],
            "level": "paid",
            "validFrom": "2025-01-01T00:00:00Z",
            "validTo": "2026-01-01T00:00:00Z",
            "subscriptionId": "sub-1",
            "defaultPaymentMethod": "pm-1",
        }
        if not rest and method == "GET":
            self._send(200, subscription)
            return True
        if rest == ["history"] and method == "GET":
            self._send(200, [subscription])
            return True
        if not rest and method in ("POST", "PUT"):
            self._send(200, {})
            return True
        if rest and method == "DELETE":
            self._send(200, {})
            return True
        return False

    def _route_billing(self, method, rest):
        if rest == ["readiness"] and method == "GET":
            self._send(200, {
                "workspace": WORKSPACE["slug"],
                "workspaceId": WORKSPACE["id"],
                "stripeCustomerId": "cus_test",
                "defaultPaymentMethodId": "pm-1",
                "status": "ready",
                "checkedAt": "2025-08-01T00:00:00Z",
                "source": "stripe",
            })
            return True
        if rest == ["customer"] and method == "PUT":
            self._send(200, {"workspace": WORKSPACE["slug"], "stripeCustomerId": "cus_test", "defaultPaymentMethod": "pm-1"})
            return True
        if len(rest) == 2 and rest[1] == "totals" and method == "GET":
            self._send(200, {"ym": rest[0], "totals": {"compute": 1.5, "storage": 0.25}})
            return True
        return False

    def log_message(self, fmt, *args):
        sys.stderr.write("[mock] " + fmt % args + "\n")


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 18080
    # Signal readiness (and catch bind failures, e.g. port already in use)
    # before serving, so start scripts can wait on this line instead of
    # sleeping for a fixed amount of time.
    print(f"[mock] listening on http://127.0.0.1:{port}", file=sys.stderr, flush=True)
    HTTPServer(("127.0.0.1", port), Handler).serve_forever()
