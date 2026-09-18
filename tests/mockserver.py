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

The mock is permissive where the live API rejects API keys (live answers
403 forbidden_operation for API keys on workspace creation and all billing
mutations): the mock accepts any bearer token for those routes, so the smoke
test can exercise the full lifecycle. The free-tier insights 503
(db_config_missing) is likewise not simulated.
"""
import json
import os
import re
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
        "database": {"id": "db-seed", "name": "test_database", "status": "READY", "region": "AWS_US_WEST_2"},
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
        "name": "test_database",
        "status": "READY",
        "region": "AWS_US_WEST_2",
        "expandable": True,
        "created": "2025-01-02T00:00:00Z",
        "apiHost": "test-database.api.thenile.dev",
        "dbHost": "test-database.db.thenile.dev",
        "workspace": WORKSPACE,
    }


# Refresh token accepted by POST /oauth2/token; the mock answers with the
# same bearer TOKEN the authenticated endpoints expect.
OAUTH_REFRESH_TOKEN = "mock-refresh-token"

# Mutable server state.
STATE = {
    "workspaces": {WORKSPACE["slug"]: dict(WORKSPACE)},
    "next_workspace": 2,
    "databases": {"test_database": seeded_database()},
    "next_database": 1,
    "computes": {"test_database": {i["instanceId"]: i for i in SEEDED_INSTANCES}},
    "credentials": {},
    "next_credential": 1,
    "invites": {},
    "next_invite": 1,
    "subscriptions": {
        WORKSPACE["slug"]: {
            "workspace": WORKSPACE["slug"],
            "level": "paid",
            "validFrom": "2025-01-01T00:00:00Z",
            "validTo": "2026-01-01T00:00:00Z",
            "subscriptionId": "sub-1",
            "defaultPaymentMethod": "pm-1",
        }
    },
    "next_subscription": 2,
    "provisioned": {},
    "next_provisioned": 1,
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

    def _read_form(self):
        length = int(self.headers.get("Content-Length") or 0)
        if not length:
            return {}
        try:
            return {k: v[0] for k, v in parse_qs(self.rfile.read(length).decode()).items()}
        except ValueError:
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
        segments = self._segments()
        # Unauthenticated endpoints (OAuth token exchange, database
        # provisioning) skip the bearer check entirely.
        if segments not in (["oauth2", "token"], ["databases", "provision"]):
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

        # /oauth2/token (unauthenticated, form-encoded)
        if segments == ["oauth2", "token"] and method == "POST":
            form = self._read_form()
            if form.get("grant_type") != "refresh_token":
                self._send(400, error_payload("bad_request", "unsupported grant_type", 400))
                return True
            if form.get("refresh_token") != OAUTH_REFRESH_TOKEN:
                self._send(401, error_payload("invalid_credentials", "bad refresh token", 401))
                return True
            self._send(200, {
                "access_token": TOKEN,
                "token_type": "bearer",
                "expires_in": 3600,
                "scope": "offline_access",
            })
            return True

        # /databases/provision (unauthenticated)
        if segments == ["databases", "provision"] and method == "POST":
            region = self._read_body().get("region", "")
            n = STATE["next_provisioned"]
            claim_code = f"claim-{n}"
            STATE["next_provisioned"] += 1
            database_id = f"proddb-{n}"
            database_name = f"unauth_mock_{n}"
            api_host = f"https://{region.lower()}.api.thenile.dev/v2/databases/{database_id}"
            db_host = f"postgres://{region.lower()}.db.thenile.dev/{database_name}"
            password = f"password-prov-{n}"
            STATE["provisioned"][claim_code] = {
                "region": region,
                "database_id": database_id,
                "database_name": database_name,
            }
            # Response shape mirrors the live /databases/provision endpoint,
            # including the env object with embedded connection credentials.
            self._send(201, {
                "claimCode": claim_code,
                "region": region,
                "databaseId": database_id,
                "databaseName": database_name,
                "apiHost": api_host,
                "dbHost": db_host,
                "credentialId": f"prov-cred-{n}",
                "password": password,
                "sharded": True,
                "env": {
                    "NILEDB_POSTGRES_URL": db_host,
                    "POSTGRES_URL": f"postgres://u-prov-{n}:{password}@{region.lower()}.db.thenile.dev/{database_name}",
                    "NILEDB_URL": f"postgres://u-prov-{n}:{password}@{region.lower()}.db.thenile.dev/{database_name}",
                    "NILEDB_API_URL": api_host,
                    "NILEDB_PASSWORD": password,
                    "NILEDB_USER": f"u-prov-{n}",
                },
            })
            return True

        # /workspaces
        if segments == ["workspaces"] and method == "GET":
            self._send(200, list(STATE["workspaces"].values()))
            return True
        if segments == ["workspaces"] and method == "POST":
            name = self._read_body().get("name", "")
            if not name:
                self._send(400, error_payload("bad_request", "name is required", 400))
                return True
            slug = name.strip().lower().replace(" ", "-")
            if slug in STATE["workspaces"]:
                self._send(409, error_payload("duplicate_entity", "workspace exists", 409))
                return True
            workspace = {
                "id": f"ws-{STATE['next_workspace']}",
                "name": name,
                "slug": slug,
                "created": "2025-08-01T00:00:00Z",
            }
            STATE["next_workspace"] += 1
            STATE["workspaces"][slug] = workspace
            self._send(201, workspace)
            return True

        if len(segments) >= 2 and segments[0] == "workspaces":
            workspace_slug = segments[1]
            rest = segments[2:]
            return self._route_workspace(method, workspace_slug, rest, query)
        return False

    def _route_workspace(self, method, workspace_slug, rest, query):
        # /workspaces/{slug}
        if not rest and method == "GET":
            workspace = STATE["workspaces"].get(workspace_slug)
            if workspace is None:
                self._send(404, error_payload("entity_not_found", "no such workspace", 404))
                return True
            # The live API answers with a single object, not an array.
            self._send(200, workspace)
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
                    "test_database": {
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
            return self._route_subscription(method, workspace_slug, tail)
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
            if not re.fullmatch(r"[a-zA-Z_][a-zA-Z0-9_]*", name or ""):
                self._send(400, error_payload(
                    "bad_request",
                    f"Invalid id: database name did not match pattern "
                    f"^[a-zA-Z_][a-zA-Z0-9_]*$: {name}", 400))
                return True
            if name in STATE["databases"]:
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
        if rest == ["claim"] and method == "POST":
            claim_code = self._read_body().get("claimCode", "")
            provisioned = STATE["provisioned"].pop(claim_code, None)
            if provisioned is None:
                self._send(404, error_payload("entity_not_found", "unknown claim code", 404))
                return True
            # Live semantics: the claimed database keeps the name and id
            # assigned during (unauthenticated) provisioning.
            name = provisioned.get("database_name", f"unauth_mock_{STATE['next_database']}")
            database = {
                "id": provisioned.get("database_id", f"db-{STATE['next_database']}"),
                "name": name,
                "status": "READY",
                "region": provisioned.get("region", "AWS_US_WEST_2"),
                "expandable": False,
                "created": "2025-08-01T00:00:00Z",
                "apiHost": f"{name}.api.thenile.dev",
                "dbHost": f"{name}.db.thenile.dev",
                "workspace": STATE["workspaces"].get(workspace_slug, WORKSPACE),
            }
            STATE["next_database"] += 1
            STATE["databases"][name] = database
            self._send(201, database)
            return True

        name = rest[0]

        if len(rest) > 1 and rest[1] == "insights":
            # The live API only accepts the database id on this path and
            # rejects the database name with 400 "Invalid id". Route this
            # before the name-based lookups below: ids are not dictionary keys.
            database = next((d for d in STATE["databases"].values() if d["id"] == rest[0]), None)
            if database is None:
                self._send(400, error_payload("bad_request", f"Invalid id: {rest[0]}", 400))
                return True
            return self._route_insights(method, rest[2:], database["name"], query)

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
        return False

    def _route_compute(self, method, database_name, rest, query):
        instances = STATE["computes"].setdefault(database_name, {})

        if not rest and method == "GET":
            values = list(instances.values())
            if database_name == "test_database":
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
        if rest == ["rotate"] and method == "POST":
            body = self._read_body()
            matching = [c for c in credentials.values() if not tenant or c.get("tenant") == tenant]
            if not matching:
                self._send(404, error_payload("entity_not_found", "no credential to rotate", 404))
                return True
            old_credential = matching[0]
            del credentials[old_credential["id"]]
            credential_id = f"cred-{STATE['next_credential']}"
            STATE["next_credential"] += 1
            credential = dict(old_credential)
            credential["id"] = credential_id
            credential["password"] = f"password-{credential_id}"
            credential["created"] = "2025-08-02T00:00:00Z"
            if body.get("delayOldSecretsExpirationHours"):
                credential["oldSecretsExpirationHours"] = body["delayOldSecretsExpirationHours"]
            if body.get("reason"):
                credential["rotationReason"] = body["reason"]
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

    def _route_insights(self, method, rest, database_name, query):
        if method != "GET" or len(rest) != 1:
            return False
        kind = rest[0]
        # The live API rejects windows that are not aligned to whole minutes.
        for param in ("start", "end"):
            value = (query.get(param) or [""])[0]
            if value and not re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:00(?:\.0+)?Z", value):
                self._send(400, error_payload(
                    "bad_request", f"{param} and end must be minute-aligned", 400))
                return True
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

    def _route_subscription(self, method, workspace_slug, rest):
        subscription = STATE["subscriptions"].get(workspace_slug)

        if not rest and method == "GET":
            if subscription is None:
                self._send(404, error_payload("entity_not_found", "no active subscription", 404))
                return True
            self._send(200, subscription)
            return True
        if rest == ["history"] and method == "GET":
            self._send(200, [subscription] if subscription else [])
            return True
        if not rest and method == "POST":
            if subscription is not None:
                self._send(409, error_payload("duplicate_entity", "subscription already started", 409))
                return True
            level = self._read_body().get("level", "")
            STATE["subscriptions"][workspace_slug] = {
                "workspace": workspace_slug,
                "level": level,
                "validFrom": "2025-08-01T00:00:00Z",
                "validTo": "2026-08-01T00:00:00Z",
                "subscriptionId": f"sub-{STATE['next_subscription']}",
                "defaultPaymentMethod": "pm-1",
            }
            STATE["next_subscription"] += 1
            self._send(200, {})
            return True
        if not rest and method == "PUT":
            if subscription is None:
                self._send(404, error_payload("entity_not_found", "no active subscription", 404))
                return True
            subscription["level"] = self._read_body().get("level", "")
            self._send(200, {})
            return True
        if rest and method == "DELETE":
            # DELETE /workspaces/{slug}/subscription/{id}: the id segment is
            # the rest after the "subscription" head. The mock does not
            # validate it, mirroring the tolerant live API.
            STATE["subscriptions"].pop(workspace_slug, None)
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
