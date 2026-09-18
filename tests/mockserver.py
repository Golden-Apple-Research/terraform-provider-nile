#!/usr/bin/env python3
"""Mock of the Nile GET /workspaces/{slug}/databases/{name}/compute endpoint.

The payload mirrors the documented response shape, see
https://www.thenile.dev/docs/api-reference/databases/list-dedicated-compute-instances-for-a-database
"""
import json
from http.server import HTTPServer, BaseHTTPRequestHandler
import sys

TOKEN = "test-token-123"

RESPONSE = [
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
        "workspace": {"id": "ws-1", "name": "Test Workspace", "slug": "test-workspace"},
        "database": {"id": "db-1", "name": "test-database", "status": "READY", "region": "AWS_US_WEST_2"},
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


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if "/databases/" in self.path and self.path.endswith("/compute"):
            if self.headers.get("Authorization") != "Bearer " + TOKEN:
                self.send_response(401)
                self.end_headers()
                self.wfile.write(b'{"error": "unauthorized"}')
                return
            body = json.dumps(RESPONSE).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, fmt, *args):
        sys.stderr.write("[mock] " + fmt % args + "\n")


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 18080
    HTTPServer(("127.0.0.1", port), Handler).serve_forever()
