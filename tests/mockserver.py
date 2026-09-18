#!/usr/bin/env python3
"""Mock of the Nile GET /workspaces/{slug}/databases/{name}/compute endpoint."""
import json
from http.server import HTTPServer, BaseHTTPRequestHandler
import sys

RESPONSE = [
    {
        "id": "ci-abc123",
        "status": "running",
        "size": "standard-2",
        "region": "us-east-1",
        "created_at": "2025-06-01T12:00:00Z",
        "extra_nested": {"foo": "bar"}
    },
    {
        "id": "ci-def456",
        "status": "terminated",
        "size": "standard-4",
        "region": "eu-west-1",
        "created_at": "2025-07-15T08:30:00Z"
    }
]

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if "/databases/" in self.path and self.path.endswith("/compute"):
            if self.headers.get("Authorization") != "Bearer test-token-123":
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
