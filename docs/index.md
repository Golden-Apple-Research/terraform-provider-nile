---
page_title: "Nile Provider"
description: |-
  The Nile provider manages and queries databases, compute instances,
  credentials, developer invites, workspaces and billing data through the
  Nile control plane API.
---

# Nile Provider

The Nile provider lets you manage and query resources in the [Nile](https://www.thenile.dev)
control plane from Terraform. It implements the full
[Nile management API](https://www.thenile.dev/docs/api-reference).

Use the provider to provision databases and their dedicated compute instances,
to issue and inspect database credentials, and to read workspaces, developers,
subscriptions, billing and observability data. Most read-only API responses are
exposed both as typed attributes and as a `raw_json` payload, so fields the
provider does not promote yet remain reachable with `jsondecode()`.

## Example Usage

```terraform
terraform {
  required_providers {
    nile = {
      source  = "golden-apple-research/nile"
      version = "~> 0.1"
    }
  }
}

provider "nile" {
  # api_token can also be set with NILE_API_TOKEN.
  api_token = var.nile_api_token

  # api_url is optional and defaults to https://global.thenile.dev.
  # It can also be set with NILE_API_URL.
}

variable "nile_api_token" {
  type      = string
  sensitive = true
}

resource "nile_database" "app" {
  workspace_slug = "my-workspace"
  name           = "app-database"
  region         = "AWS_US_WEST_2"
}

resource "nile_database_credential" "app" {
  workspace_slug = nile_database.app.workspace_slug
  database_name  = nile_database.app.name
}

output "database_host" {
  value = nile_database.app.db_host
}

output "credential_password" {
  value     = nile_database_credential.app.password
  sensitive = true
}
```

## Authentication

The provider authenticates with a Nile bearer token. Configure it with either
the `api_token` provider argument or the `NILE_API_TOKEN` environment variable.
An unknown or empty token is reported before a client is created.

The API base URL is configured with either the `api_url` provider argument or
the `NILE_API_URL` environment variable. If neither is set, the provider uses
`https://global.thenile.dev`. HTTPS is required; plain HTTP is accepted only for
loopback test endpoints. Redirects are not followed, so the bearer token cannot
be forwarded to a different endpoint.

## Argument Reference

- `api_token` - (Optional, Sensitive) Bearer token used to authenticate against the Nile API. May also be set with `NILE_API_TOKEN`.
- `api_url` - (Optional) Base URL of the Nile API. Defaults to `https://global.thenile.dev`. May also be set with `NILE_API_URL`. HTTPS is required; plain HTTP is only accepted for loopback endpoints.

## Resources

- [`nile_database`](resources/database.md) - Manages a Nile database (create, rename, delete).
- [`nile_database_compute_instance`](resources/database_compute_instance.md) - Manages a dedicated compute instance (create, rename, resize, delete).
- [`nile_database_credential`](resources/database_credential.md) - Manages a database credential.
- [`nile_developer_invite`](resources/developer_invite.md) - Manages a developer invite.

## Data Sources

### Databases

- [`nile_database`](data-sources/database.md) - Fetches a single database.
- [`nile_databases`](data-sources/databases.md) - Lists the databases of a workspace.
- [`nile_database_compute_instances`](data-sources/database_compute_instances.md) - Lists the dedicated compute instances of a database, optionally within a time window.
- [`nile_database_credentials`](data-sources/database_credentials.md) - Lists the credentials of a database (without passwords).
- [`nile_database_uptime_insights`](data-sources/database_uptime_insights.md) - Lists time-weighted database uptime metrics.
- [`nile_database_error_insights`](data-sources/database_error_insights.md) - Lists bucketed SQL and connection error metrics.
- [`nile_database_query_performance_insights`](data-sources/database_query_performance_insights.md) - Lists Proxy and Thoth query performance metrics.

### Workspaces

- [`nile_workspace`](data-sources/workspace.md) - Fetches a single workspace.
- [`nile_workspaces`](data-sources/workspaces.md) - Lists all workspaces the token can access.
- [`nile_regions`](data-sources/regions.md) - Lists the regions available to a workspace.
- [`nile_compute_types`](data-sources/compute_types.md) - Lists the dedicated compute types available to a workspace.
- [`nile_workspace_developers`](data-sources/workspace_developers.md) - Lists the developers of a workspace.
- [`nile_workspace_compute_usage`](data-sources/workspace_compute_usage.md) - Lists the compute usage of a workspace.
- [`nile_workspace_subscription`](data-sources/workspace_subscription.md) - Fetches the current subscription.
- [`nile_workspace_subscription_history`](data-sources/workspace_subscription_history.md) - Lists the subscription history, most recent first.
- [`nile_workspace_billing_readiness`](data-sources/workspace_billing_readiness.md) - Resolves the billing readiness of a workspace.
- [`nile_workspace_billing_totals`](data-sources/workspace_billing_totals.md) - Fetches the monthly rated totals by component.

### Developers

- [`nile_developer`](data-sources/developer.md) - Returns the developer behind the configured token.
- [`nile_workspace_invites`](data-sources/workspace_invites.md) - Lists the developer invites of a workspace.

## Behaviour Notes

- **Asynchronous operations.** Creating a database or compute instance, renaming
  a database and resizing an instance are asynchronous. The resources poll the
  API until the object reports `READY` (or disappears on delete) before they
  complete, so dependent resources can be created in the same apply. Waiting is
  bounded by the resource `timeouts` blocks, which default to 20 minutes per
  operation.
- **Secrets.** Credential passwords and programmatic invite codes are returned
  by the API exactly once and are stored as sensitive values in state. Terraform
  state must therefore use an encrypted backend with restricted access.
- **`raw_json`.** Most data sources expose the full API payload as the sensitive
  `raw_json` attribute. Common secret fields (passwords, tokens, secrets,
  connection strings and invite codes) are replaced with `[REDACTED]` before the
  payload reaches state; use `jsondecode()` to read fields the provider does not
  promote yet.
- **Retries.** Transient failures — HTTP `408`, `429`, `5xx`, and network errors
  — are retried up to three times with exponential backoff and jitter for
  replay-safe HTTP methods. A `Retry-After` header takes precedence over the
  computed backoff (capped at 30 seconds); non-idempotent `POST` requests are
  never retried.
- **Import.** Every resource supports `terraform import`; see the resource pages
  for the identifier format.

## License

Licensed under the [European Union Public License, Version 1.2](https://opensource.org/license/EUPL-1.2)
(SPDX: `EUPL-1.2`).
