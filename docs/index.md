---
page_title: "Nile Provider"
---

# Nile Provider

The Nile provider lets you manage and query resources in the [Nile](https://www.thenile.dev) control plane from Terraform. It implements the [Nile management API](https://www.thenile.dev/docs/api-reference).

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
```

## Authentication

The provider authenticates with a Nile bearer token. Configure it using either the `api_token` provider argument or the `NILE_API_TOKEN` environment variable.

The API base URL can be configured using either the `api_url` provider argument or the `NILE_API_URL` environment variable. If neither is set, the provider uses `https://global.thenile.dev`.

## Argument Reference

- `api_token` - (Optional, Sensitive) Bearer token used to authenticate against the Nile API. May also be set with `NILE_API_TOKEN`.
- `api_url` - (Optional) Base URL of the Nile API. Defaults to `https://global.thenile.dev`. May also be set with `NILE_API_URL`.

## Resources

- [`nile_database`](resources/database) - Manages a Nile database (create, rename, delete).
- [`nile_database_compute_instance`](resources/database_compute_instance) - Manages a dedicated compute instance (create, rename, resize, delete).
- [`nile_database_credential`](resources/database_credential) - Manages a database credential.
- [`nile_developer_invite`](resources/developer_invite) - Manages a developer invite.

## Data Sources

- [`nile_database`](data-sources/database) - Fetches a single database.
- [`nile_databases`](data-sources/databases) - Lists the databases of a workspace.
- [`nile_database_compute_instances`](data-sources/database_compute_instances) - Lists the dedicated compute instances of a database.
- [`nile_database_credentials`](data-sources/database_credentials) - Lists the credentials of a database (without passwords).
- [`nile_database_uptime_insights`](data-sources/database_uptime_insights) - Lists database uptime metrics.
- [`nile_database_error_insights`](data-sources/database_error_insights) - Lists database error metrics.
- [`nile_database_query_performance_insights`](data-sources/database_query_performance_insights) - Lists database query performance metrics.
- [`nile_regions`](data-sources/regions) - Lists the regions available to a workspace.
- [`nile_compute_types`](data-sources/compute_types) - Lists the compute types available to a workspace.
- [`nile_workspace`](data-sources/workspace) - Fetches a single workspace.
- [`nile_workspaces`](data-sources/workspaces) - Lists all workspaces the token can access.
- [`nile_workspace_developers`](data-sources/workspace_developers) - Lists the developers of a workspace.
- [`nile_workspace_invites`](data-sources/workspace_invites) - Lists the developer invites of a workspace.
- [`nile_workspace_compute_usage`](data-sources/workspace_compute_usage) - Lists the compute usage of a workspace.
- [`nile_workspace_subscription`](data-sources/workspace_subscription) - Fetches the current subscription.
- [`nile_workspace_subscription_history`](data-sources/workspace_subscription_history) - Lists the subscription history.
- [`nile_workspace_billing_readiness`](data-sources/workspace_billing_readiness) - Resolves the billing readiness of a workspace.
- [`nile_workspace_billing_totals`](data-sources/workspace_billing_totals) - Fetches the monthly rated totals.
- [`nile_developer`](data-sources/developer) - Returns the developer behind the configured token.

## Behaviour Notes

- **Asynchronous operations.** Creating a database or compute instance (and resizing an instance) polls the API until the object reports `READY`, so dependent resources can be created in the same apply.
- **Secrets.** Credential passwords and programmatic invite codes are returned by the API exactly once and stored as sensitive values.
- **`raw_json`.** Most data sources expose the full API payload in `raw_json` for fields the provider does not promote yet.
- **Import.** Every resource supports `terraform import`; see the resource pages for the identifier format.

## License

Licensed under the [European Union Public License, Version 1.2](https://opensource.org/license/EUPL-1.2) (SPDX: `EUPL-1.2`).
